package message

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/staffauth"
)

// maxAttachmentBytes is the ~10MB attachment size cap from #56's
// resolution. maxCreateRequestBytes adds headroom for the surrounding
// multipart form fields/boundary so a request just over the attachment
// cap is rejected as "too large", not misreported as a generic parse
// failure.
const (
	maxAttachmentBytes    = 10 << 20
	maxCreateRequestBytes = maxAttachmentBytes + 64<<10
	attachmentFormField   = "attachment"
	bodyFormField         = "body"
)

// CreateRequest is the body of a text-only (application/json)
// create-Message request. A multipart/form-data request instead carries a
// "body" field and an optional "attachment" file field -- see
// decodeMultipartCreate.
type CreateRequest struct {
	Body string `json:"body"`
}

// attachmentInfo describes an attachment already validated and uploaded to
// an ObjectStore, ready to persist alongside its Message row.
type attachmentInfo struct {
	objectPath  string
	contentType string
	filename    string
	byteSize    int64
}

// CreateHandler posts a Message, with an optional single image/PDF
// attachment, to an Engagement's thread as the calling Staff member. Any
// Staff member with access to the Practice may post -- there is no
// per-role restriction, unlike visit.CreateHandler's Doula-only rule,
// since #58's thread has no per-Staff-member sub-threads. On success,
// notifies the Client's registered push subscription(s) (#61) before
// responding. Must be mounted behind staffauth.Middleware.
func CreateHandler(store objectstore.ObjectStore, pusher push.Pusher) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		staffID, _ := staffauth.StaffID(r.Context())

		engagementID := r.PathValue("engagementId")
		if !staffauth.ParseUUID(w, "engagement", engagementID) {
			return
		}
		if err := requireEngagementAtPractice(r.Context(), tx, engagementID, practiceID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "engagement not found", http.StatusNotFound)
				return
			}
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		messageID := uuid.NewString()
		body, attachment, ok := decodeCreate(w, r, store, engagementID, messageID, staffauth.MsgInternalError)
		if !ok {
			return
		}

		item, err := insertMessage(r.Context(), tx, messageID, engagementID, senderTypeStaff, staffID, body, attachment)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		notifyRecipient(r.Context(), tx, pusher, engagementID, senderTypeClient)
		writeCreated(w, item, staffauth.MsgInternalError)
	})
}

// decodeCreate reads a create-Message request, branching on Content-Type:
// multipart/form-data carries text plus an optional attachment,
// application/json (the original shape) stays text-only. Branching here,
// rather than switching the endpoint to multipart-only, keeps the
// existing JSON contract intact -- an additive change, per
// docs/api-design.md -- since #60 only adds an *optional* attachment.
// Writes its own error response and returns ok=false on failure.
func decodeCreate(w http.ResponseWriter, r *http.Request, store objectstore.ObjectStore, engagementID, messageID, internalErrorMsg string) (body string, attachment *attachmentInfo, ok bool) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		req, ok := decodeCreateRequest(w, r)
		return req.Body, nil, ok
	}
	return decodeMultipartCreate(w, r, store, engagementID, messageID, internalErrorMsg)
}

// decodeMultipartCreate parses a multipart/form-data create-Message
// request and, if it carries an attachment, validates and uploads it
// before returning -- "size/content-type validation happens in the
// handler before Put is called", per #60's AC. r.Body is wrapped in
// http.MaxBytesReader first so an oversized request is rejected while
// parsing, before any of it reaches the ObjectStore.
func decodeMultipartCreate(w http.ResponseWriter, r *http.Request, store objectstore.ObjectStore, engagementID, messageID, internalErrorMsg string) (body string, attachment *attachmentInfo, ok bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxCreateRequestBytes)
	if err := r.ParseMultipartForm(1 << 20); err != nil { //nolint:gosec // bounded by the MaxBytesReader wrap above, not unbounded
		// A request that trips MaxBytesReader is still "attachment too
		// large" from the caller's point of view -- report the same 413
		// as the post-parse header.Size check below, rather than letting
		// the parse-boundary leak out as a different status code for what
		// is, to the caller, the same failure.
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			http.Error(w, "attachment exceeds the 10MB limit", http.StatusRequestEntityTooLarge)
			return "", nil, false
		}
		http.Error(w, "malformed multipart request", http.StatusBadRequest)
		return "", nil, false
	}

	body = strings.TrimSpace(r.FormValue(bodyFormField))

	file, header, err := r.FormFile(attachmentFormField)
	switch {
	case errors.Is(err, http.ErrMissingFile):
		if body == "" {
			http.Error(w, "body or attachment is required", http.StatusBadRequest)
			return "", nil, false
		}
		return body, nil, true
	case err != nil:
		// coverage:ignore reason: r.FormFile only returns a non-ErrMissingFile error on a
		// filesystem-level failure opening an already-parsed part, not exercised by unit tests
		http.Error(w, "invalid attachment", http.StatusBadRequest)
		return "", nil, false
	}
	defer func() { _ = file.Close() }()

	info, ok := validateAndStoreAttachment(w, r, store, file, header, engagementID, messageID, internalErrorMsg)
	if !ok {
		return "", nil, false
	}
	return body, info, true
}

// validAttachmentContentType reports whether contentType is one of #56's
// two allowed attachment kinds.
func validAttachmentContentType(contentType string) bool {
	return contentType == "application/pdf" || strings.HasPrefix(contentType, "image/")
}

// validateAndStoreAttachment enforces the ~10MB size cap and image/PDF
// content-type rule on an uploaded file, sniffing the real content type
// via http.DetectContentType rather than trusting the client-declared
// header (trivially spoofable) -- the sniffed type is also what's
// persisted, so the download endpoint later serves back the type that was
// actually validated. On success, uploads the file to store under a path
// scoped by engagementID/messageID.
func validateAndStoreAttachment(w http.ResponseWriter, r *http.Request, store objectstore.ObjectStore, file multipart.File, header *multipart.FileHeader, engagementID, messageID, internalErrorMsg string) (*attachmentInfo, bool) {
	if header.Size > maxAttachmentBytes {
		http.Error(w, "attachment exceeds the 10MB limit", http.StatusRequestEntityTooLarge)
		return nil, false
	}

	sniff := make([]byte, 512)
	n, err := io.ReadFull(file, sniff)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		// coverage:ignore reason: a parsed multipart.File failing mid-read is a
		// filesystem-level failure, not exercised by unit tests
		http.Error(w, "invalid attachment", http.StatusBadRequest)
		return nil, false
	}
	contentType := http.DetectContentType(sniff[:n])
	if !validAttachmentContentType(contentType) {
		http.Error(w, "attachment must be an image or a PDF", http.StatusBadRequest)
		return nil, false
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		// coverage:ignore reason: multipart.File always supports Seek in practice, not exercised by unit tests
		http.Error(w, internalErrorMsg, http.StatusInternalServerError)
		return nil, false
	}

	objectPath := fmt.Sprintf("messages/%s/%s", engagementID, messageID)
	if err := store.Put(r.Context(), objectPath, contentType, file); err != nil {
		http.Error(w, internalErrorMsg, http.StatusInternalServerError)
		return nil, false
	}

	return &attachmentInfo{
		objectPath:  objectPath,
		contentType: contentType,
		filename:    header.Filename,
		byteSize:    header.Size,
	}, true
}

// decodeCreateRequest decodes and validates a CreateRequest body, shared by
// CreateHandler and ClientCreateHandler so the "text required, trimmed"
// rule lives in one place. Writes its own error response and returns
// ok=false on failure.
func decodeCreateRequest(w http.ResponseWriter, r *http.Request) (CreateRequest, bool) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return CreateRequest{}, false
	}
	req.Body = strings.TrimSpace(req.Body)
	if req.Body == "" {
		http.Error(w, "body is required", http.StatusBadRequest)
		return CreateRequest{}, false
	}
	return req, true
}

// writeCreated writes item as a 201 JSON response, shared by CreateHandler
// and ClientCreateHandler.
func writeCreated(w http.ResponseWriter, item Message, internalErrorMsg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	// coverage:ignore reason: response encoding failure, not exercised by unit tests
	if err := json.NewEncoder(w).Encode(item); err != nil {
		http.Error(w, internalErrorMsg, http.StatusInternalServerError)
	}
}

// insertMessage writes a Message row from the calling Staff or Client
// identity (senderType picks which) and returns it in the same Message
// shape ListHandler uses, so the frontend can append the response
// directly without a refetch. Unlike listMessages' COALESCE across two
// LEFT JOINs for a polymorphic sender, the sender here is always the
// caller, so a single lookup (picked by resolveSenderName) is enough.
// attachment is nil for a text-only Message; body is stored as NULL
// (rather than an empty string) for an attachment-only Message, matching
// 00008_messaging.sql's "body nullable if attachment-only" intent.
func insertMessage(ctx context.Context, tx *sql.Tx, messageID, engagementID, senderType, senderID, body string, attachment *attachmentInfo) (Message, error) {
	msg := Message{
		MessageID:  messageID,
		SenderType: senderType,
		SenderID:   senderID,
		Body:       body,
	}

	var bodyArg sql.NullString
	if body != "" {
		bodyArg = sql.NullString{String: body, Valid: true}
	}

	var objectPath, contentType, filename sql.NullString
	var byteSize sql.NullInt64
	if attachment != nil {
		objectPath = sql.NullString{String: attachment.objectPath, Valid: true}
		contentType = sql.NullString{String: attachment.contentType, Valid: true}
		filename = sql.NullString{String: attachment.filename, Valid: true}
		byteSize = sql.NullInt64{Int64: attachment.byteSize, Valid: true}
		msg.AttachmentContentType = attachment.contentType
		msg.AttachmentFilename = attachment.filename
	}

	err := tx.QueryRowContext(ctx,
		`INSERT INTO messages (id, engagement_id, sender_type, sender_id, body,
			attachment_object_path, attachment_content_type, attachment_byte_size, attachment_filename)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING created_at`,
		messageID, engagementID, senderType, senderID, bodyArg,
		objectPath, contentType, byteSize, filename,
	).Scan(&msg.CreatedAt)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return Message{}, fmt.Errorf("message: insert message: %w", err)
	}

	name, err := resolveSenderName(ctx, tx, senderType, senderID)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return Message{}, err
	}
	msg.SenderName = name

	return msg, nil
}

// resolveSenderName looks up the display name of the Staff or Client that
// sent a Message, picking the static query by senderType (senderTypeStaff
// or senderTypeClient) rather than building the query dynamically.
func resolveSenderName(ctx context.Context, tx *sql.Tx, senderType, senderID string) (string, error) {
	query := `SELECT name FROM staff WHERE id = $1`
	if senderType == senderTypeClient {
		query = `SELECT name FROM clients WHERE id = $1`
	}

	var name string
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err := tx.QueryRowContext(ctx, query, senderID).Scan(&name); err != nil {
		return "", fmt.Errorf("message: resolve sender name: %w", err)
	}
	return name, nil
}
