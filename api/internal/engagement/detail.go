// Package engagement holds the Staff-side BFF handlers for Engagement
// detail and completion. All handlers rely on staffauth.Middleware
// having already resolved the caller's Staff/Practice ids and opened a
// request-scoped *sql.Tx with app.current_practice_id set, the same way
// staffauth's own Owner-only handlers (invite, role assignment) do. The
// Client write surface and the Clients list moved to package client
// (#397); this package's own surface is now just the Engagement itself.
package engagement

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/staffauth"
)

// Detail is an Engagement's basic detail: the Client it's for, its
// status, when it was created, and when it's due -- the landing point
// later features (Visits, Plans, Contracts, Messages) attach to.
type Detail struct {
	EngagementID string    `json:"engagementId"`
	ClientID     string    `json:"clientId"`
	ClientName   string    `json:"clientName"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"createdAt"`
	// DueDate is ADR-0017's `engagements.due_date`, nullable because a
	// postpartum-only Engagement has none. Mirrors portal.Detail's own
	// field (#505) -- same nullable-column read, same omitted-when-null
	// shape (#538).
	DueDate *string `json:"dueDate,omitempty"`
}

// DetailHandler views one Engagement's basic detail: every Staff role
// except a contractor Doula without an open, granted attachment on it
// (ADR-0008). Must be mounted behind staffauth.Middleware.
func DetailHandler() http.Handler {
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

		var d Detail
		var givenName string
		var preferredName sql.NullString
		var dueDate sql.NullString
		err = tx.QueryRowContext(r.Context(),
			`SELECT e.id, c.id, c.given_name, c.preferred_name, e.status, e.created_at, e.due_date::text
			 FROM engagements e
			 JOIN clients c ON c.id = e.client_id
			 WHERE e.id = $1 AND e.practice_id = $2`,
			engagementID, practiceID,
		).Scan(&d.EngagementID, &d.ClientID, &givenName, &preferredName, &d.Status, &d.CreatedAt, &dueDate)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "engagement not found", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		d.ClientName = client.PreferredName(givenName, preferredName.String)
		if dueDate.Valid {
			d.DueDate = &dueDate.String
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(d); err != nil {
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}
