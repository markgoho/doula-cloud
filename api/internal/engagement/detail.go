package engagement

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

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

// DetailHandler views one Engagement's basic detail. Must be mounted
// behind staffauth.Middleware.
func DetailHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := requireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}

		engagementID := r.PathValue("engagementId")
		if _, err := uuid.Parse(engagementID); err != nil {
			http.Error(w, "invalid engagement id", http.StatusBadRequest)
			return
		}

		var d Detail
		err := tx.QueryRowContext(r.Context(),
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
