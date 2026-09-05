package message

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/staffauth"
)

// AttachmentHandler streams a Message's stored attachment back to the
// calling Staff member, narrowed by ADR-0008's attachment rule for a
// contractor Doula, same as ListHandler. Must be mounted behind
// staffauth.Middleware.
func AttachmentHandler(store objectstore.ObjectStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}

		engagementID := r.PathValue("engagementId")
		if !staffauth.ParseUUID(w, "engagement", engagementID) {
			return
		}
		if err := requireEngagementAtPractice(r.Context(), tx, engagementID, practiceID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				apierr.WriteError(w, "engagement not found", http.StatusNotFound)
				return
			}
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		staffID, _ := staffauth.StaffID(r.Context())
		reader, err := staffauth.ResolveReader(r.Context(), tx, practiceID, staffID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		canAccess, err := reader.CanAccessEngagement(r.Context(), tx, engagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !canAccess {
			apierr.WriteError(w, "engagement not found", http.StatusNotFound)
			return
		}

		messageID := r.PathValue("messageId")
		if !staffauth.ParseUUID(w, "message", messageID) {
			return
		}

		serveAttachment(w, r, tx, store, engagementID, messageID, staffauth.MsgInternalError)
	})
}

// ClientAttachmentHandler mirrors AttachmentHandler for the Client-portal
// population: clientauth.Middleware has already scoped the caller to
// their own Engagement (same reasoning as ClientListHandler), so no extra
// ownership check is needed here. Must be mounted behind
// clientauth.Middleware.
func ClientAttachmentHandler(store objectstore.ObjectStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, has := clientauth.Tx(r.Context())
		// coverage:ignore reason: clientauth.Middleware always sets a tx before this handler runs
		if !has {
			apierr.WriteError(w, clientauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		engagementID, _ := clientauth.EngagementID(r.Context())

		messageID := r.PathValue("messageId")
		if !staffauth.ParseUUID(w, "message", messageID) {
			return
		}

		serveAttachment(w, r, tx, store, engagementID, messageID, clientauth.MsgInternalError)
	})
}

// serveAttachment looks up messageID's attachment metadata, filtered by
// engagementID -- messageID alone is attacker-supplied, so this app-layer
// filter runs on top of messages' own RLS, the same posture listMessages
// takes -- then streams the object's bytes from store. Shared by
// AttachmentHandler and ClientAttachmentHandler.
func serveAttachment(w http.ResponseWriter, r *http.Request, tx *sql.Tx, store objectstore.ObjectStore, engagementID, messageID, internalErrorMsg string) {
	var objectPath, contentType, filename sql.NullString
	err := tx.QueryRowContext(r.Context(),
		`SELECT attachment_object_path, attachment_content_type, attachment_filename
		 FROM messages WHERE id = $1 AND engagement_id = $2`,
		messageID, engagementID,
	).Scan(&objectPath, &contentType, &filename)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !objectPath.Valid) {
		apierr.WriteError(w, "attachment not found", http.StatusNotFound)
		return
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		apierr.WriteError(w, internalErrorMsg, http.StatusInternalServerError)
		return
	}

	obj, err := store.Get(r.Context(), objectPath.String)
	if err != nil {
		apierr.WriteError(w, internalErrorMsg, http.StatusInternalServerError)
		return
	}
	defer func() { _ = obj.Close() }()

	w.Header().Set("Content-Type", contentType.String)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename.String))
	// coverage:ignore reason: response streaming failure, not exercised by unit tests
	if _, err := io.Copy(w, obj); err != nil {
		return
	}
}
