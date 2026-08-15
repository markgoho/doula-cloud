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

		var req CreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		req.Body = strings.TrimSpace(req.Body)
		if req.Body == "" {
			http.Error(w, "body is required", http.StatusBadRequest)
			return
		}

		messageID := uuid.NewString()
		item, err := insertMessage(r.Context(), tx, messageID, engagementID, staffID, req.Body)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(item); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// insertMessage writes a Message row from the calling Staff member and
// returns it in the same Message shape ListHandler uses, so the frontend
// can append the response directly without a refetch. Unlike listMessages'
// COALESCE across two LEFT JOINs for a polymorphic sender, the sender here
// is always the calling Staff member, so a single staff lookup is enough.
func insertMessage(ctx context.Context, tx *sql.Tx, messageID, engagementID, staffID, body string) (Message, error) {
	msg := Message{
		MessageID:  messageID,
		SenderType: senderTypeStaff,
		SenderID:   staffID,
		Body:       body,
	}

	err := tx.QueryRowContext(ctx,
		`INSERT INTO messages (id, engagement_id, sender_type, sender_id, body)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING created_at`,
		messageID, engagementID, senderTypeStaff, staffID, body,
	).Scan(&msg.CreatedAt)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return Message{}, fmt.Errorf("message: insert message: %w", err)
	}

	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err := tx.QueryRowContext(ctx, `SELECT name FROM staff WHERE id = $1`, staffID).Scan(&msg.SenderName); err != nil {
		return Message{}, fmt.Errorf("message: resolve sender name: %w", err)
	}

	return msg, nil
}
