package message

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"doula-cloud/api/internal/staffauth"
)

// CreateRequest is the body of a create-Message request. Text-only:
// attachments are a separate ticket (#56's Implementation Decisions).
type CreateRequest struct {
	Body string `json:"body"`
}

// CreateHandler posts a text-only Message to an Engagement's thread as
// the calling Staff member. Any Staff member with access to the Practice
// may post -- there is no per-role restriction, unlike visit.CreateHandler's
// Doula-only rule, since #58's thread has no per-Staff-member sub-threads.
// Must be mounted behind staffauth.Middleware.
func CreateHandler() http.Handler {
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

		req, ok := decodeCreateRequest(w, r)
		if !ok {
			return
		}

		messageID := uuid.NewString()
		item, err := insertMessage(r.Context(), tx, messageID, engagementID, senderTypeStaff, staffID, req.Body)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		writeCreated(w, item, staffauth.MsgInternalError)
	})
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
func insertMessage(ctx context.Context, tx *sql.Tx, messageID, engagementID, senderType, senderID, body string) (Message, error) {
	msg := Message{
		MessageID:  messageID,
		SenderType: senderType,
		SenderID:   senderID,
		Body:       body,
	}

	err := tx.QueryRowContext(ctx,
		`INSERT INTO messages (id, engagement_id, sender_type, sender_id, body)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING created_at`,
		messageID, engagementID, senderType, senderID, body,
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
