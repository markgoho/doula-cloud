package engagement

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"doula-cloud/api/internal/staffauth"
)

// Detail is an Engagement's basic detail: the Client it's for, its
// status, and when it was created -- the landing point later features
// (Visits, Plans, Contracts, Messages) attach to.
type Detail struct {
	EngagementID string    `json:"engagementId"`
	ClientID     string    `json:"clientId"`
	ClientName   string    `json:"clientName"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"createdAt"`
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
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		canAccess, err := reader.CanAccessEngagement(r.Context(), tx, engagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !canAccess {
			http.Error(w, "engagement not found", http.StatusNotFound)
			return
		}

		var d Detail
		err = tx.QueryRowContext(r.Context(),
			`SELECT e.id, c.id, c.name, e.status, e.created_at
			 FROM engagements e
			 JOIN clients c ON c.id = e.client_id
			 WHERE e.id = $1 AND e.practice_id = $2`,
			engagementID, practiceID,
		).Scan(&d.EngagementID, &d.ClientID, &d.ClientName, &d.Status, &d.CreatedAt)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "engagement not found", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(d); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}
