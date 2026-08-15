package visit

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/staffauth"
)

// Visit is one row of a Visit list: who it's assigned to and when it was
// created.
type Visit struct {
	VisitID   string    `json:"visitId"`
	StaffID   string    `json:"staffId"`
	StaffName string    `json:"staffName"`
	CreatedAt time.Time `json:"createdAt"`
}

// ListHandler lists every Visit under an Engagement, regardless of which
// Doula it's assigned to -- same "any Staff at the Practice can see it"
// visibility as engagement.ListHandler. Must be mounted behind
// staffauth.Middleware.
func ListHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := requireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}

		engagementID := r.PathValue("engagementId")
		if !parseUUID(w, "engagement", engagementID) {
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

		list, err := listVisits(r.Context(), tx, engagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(list); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// listVisits is filtered by engagementID explicitly, on top of the RLS
// scoping staffauth.Middleware already set up on tx -- the app layer's own
// filter, so a bug in either one alone can't leak rows.
func listVisits(ctx context.Context, tx *sql.Tx, engagementID string) ([]Visit, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT v.id, s.id, s.name, v.created_at
		 FROM visits v
		 JOIN staff s ON s.id = v.staff_id
		 WHERE v.engagement_id = $1
		 ORDER BY v.created_at`,
		engagementID,
	)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return nil, fmt.Errorf("visit: list visits: %w", err)
	}
	defer func() { _ = rows.Close() }()

	list := []Visit{}
	for rows.Next() {
		var v Visit
		if err := rows.Scan(&v.VisitID, &v.StaffID, &v.StaffName, &v.CreatedAt); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("visit: scan visit row: %w", err)
		}
		list = append(list, v)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("visit: iterate visit rows: %w", err)
	}
	return list, nil
}
