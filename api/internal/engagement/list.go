package engagement

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/staffauth"
)

// ClientEngagement is one row of the Client+Engagement list: a Client who
// has an Engagement at the current Practice.
type ClientEngagement struct {
	ClientID     string `json:"clientId"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	EngagementID string `json:"engagementId"`
	Status       string `json:"status"`
}

// ListHandler lists every Client with an Engagement at the current
// Practice, regardless of which Staff member created it -- v1 has no
// restricted-visibility model. Must be mounted behind staffauth.Middleware.
func ListHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}

		list, err := listClientEngagements(r.Context(), tx, practiceID)
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

// listClientEngagements is filtered by practiceID explicitly, on top of
// the RLS scoping staffauth.Middleware already set up on tx -- the app
// layer's own filter, so a bug in either one alone can't leak rows.
func listClientEngagements(ctx context.Context, tx *sql.Tx, practiceID string) ([]ClientEngagement, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT c.id, c.name, c.email, e.id, e.status
		 FROM engagements e
		 JOIN clients c ON c.id = e.client_id
		 WHERE e.practice_id = $1
		 ORDER BY c.name`,
		practiceID,
	)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return nil, fmt.Errorf("engagement: list client engagements: %w", err)
	}
	defer func() { _ = rows.Close() }()

	list := []ClientEngagement{}
	for rows.Next() {
		var ce ClientEngagement
		if err := rows.Scan(&ce.ClientID, &ce.Name, &ce.Email, &ce.EngagementID, &ce.Status); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("engagement: scan client engagement row: %w", err)
		}
		list = append(list, ce)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("engagement: iterate client engagement rows: %w", err)
	}
	return list, nil
}
