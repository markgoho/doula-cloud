// Package engagement holds the Staff-side BFF handlers for Client and
// Engagement: list, create, and view. All three rely on
// staffauth.Middleware having already resolved the caller's Staff/Practice
// ids and opened a request-scoped *sql.Tx with app.current_practice_id
// set, the same way staffauth's own Owner-only handlers (invite, role
// assignment) do.
package engagement

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/staffauth"
)

// intakeStatus is the status every new Engagement starts at -- the
// beginning of the intake-through-postpartum span described in
// CONTEXT.md. There is no create-time way to set a different status.
const intakeStatus = "intake"

// CreateClientRequest is the body of a create-Client-and-Engagement
// request.
type CreateClientRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// CreateClientResponse identifies the Client and Engagement rows created.
type CreateClientResponse struct {
	ClientID     string `json:"clientId"`
	EngagementID string `json:"engagementId"`
	Status       string `json:"status"`
}

// CreateHandler creates a Client and, in the same request, an Engagement
// linking them to the current Practice -- there is no way to create a
// Client without one. Must be mounted behind staffauth.Middleware. db is
// needed only for the ErrNoCreditsRemaining path: queuing the
// out-of-Credits Notification (#342) via billing.QueueOutOfCreditsNotification,
// which must survive the request tx's rollback and so can't run on that
// tx.
func CreateHandler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}

		var req CreateClientRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		req.Email = strings.TrimSpace(req.Email)
		if req.Name == "" || req.Email == "" {
			http.Error(w, "name and email are required", http.StatusBadRequest)
			return
		}

		// Generated in Go, not via `RETURNING id`: at the moment the client
		// row is inserted no engagement referencing it exists yet, so it
		// doesn't match the clients_select SELECT policy, and Postgres
		// applies SELECT policies to RETURNING rows too (see
		// 00005_client_engagement.sql).
		clientID := uuid.NewString()
		engagementID := uuid.NewString()

		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO clients (id, name, email) VALUES ($1, $2, $3)`,
			clientID, req.Name, req.Email,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO engagements (id, client_id, practice_id, status) VALUES ($1, $2, $3, $4)`,
			engagementID, clientID, practiceID, intakeStatus,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		// Runs after the Engagement insert, not before: the consumption
		// row's consumed_engagement_id FK requires the Engagement to
		// already exist. On ErrNoCreditsRemaining, staffauth.Middleware's
		// deferred tx.Commit() would otherwise still persist the Client
		// and Engagement just written above, so this is the one handler
		// in the codebase that must roll back explicitly before
		// responding -- Middleware's later Commit() then fails harmlessly
		// against an already-done tx.
		if err := billing.ConsumeCredit(r.Context(), tx, practiceID, engagementID); err != nil {
			if errors.Is(err, billing.ErrNoCreditsRemaining) {
				// Read while the request tx (and its app.current_practice_id)
				// is still live -- credit_ledger is practice-tier RLS, and
				// this is the last point in the request that tx is usable
				// for it.
				shouldNotify, notifyCheckErr := billing.ShouldQueueOutOfCreditsNotification(r.Context(), tx, practiceID)
				_ = tx.Rollback()
				if notifyCheckErr != nil {
					// coverage:ignore reason: DB query failure, not exercised by unit tests
					http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
					return
				}
				if shouldNotify {
					if err := billing.QueueOutOfCreditsNotification(r.Context(), db, practiceID); err != nil {
						// coverage:ignore reason: DB query failure, not exercised by unit tests
						http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
						return
					}
				}
				http.Error(w, "no credits remaining, ask a Practice Owner to buy more", http.StatusPaymentRequired)
				return
			}
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(CreateClientResponse{ClientID: clientID, EngagementID: engagementID, Status: intakeStatus}); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}
