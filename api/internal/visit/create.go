package visit

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"doula-cloud/api/internal/staffauth"
)

// CreateResponse identifies the Visit row created.
type CreateResponse struct {
	VisitID string `json:"visitId"`
	StaffID string `json:"staffId"`
}

// CreateHandler creates a Visit under an Engagement, assigned to the
// calling Staff member. Must be mounted behind staffauth.Middleware; the
// caller must hold the Doula role at the current Practice.
func CreateHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := requireDoula(w, r)
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

		visitID := uuid.NewString()
		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO visits (id, engagement_id, staff_id) VALUES ($1, $2, $3)`,
			visitID, engagementID, staffID,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(CreateResponse{VisitID: visitID, StaffID: staffID}); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}
