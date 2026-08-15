package clientauth

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/authn"
)

// EngagementSummary is one Engagement a Client has, and the Practice it's
// at -- enough for the frontend to decide where to land the Client after
// login.
type EngagementSummary struct {
	EngagementID string `json:"engagementId"`
	PracticeName string `json:"practiceName"`
	Status       string `json:"status"`
}

// SessionResponse is what the frontend needs to decide where to land a
// Client-portal user after login: straight to their only Engagement, or a
// picker if they somehow have more than one (a Client can, in principle,
// have Engagements at more than one Practice over time).
type SessionResponse struct {
	ClientID    string              `json:"clientId"`
	Engagements []EngagementSummary `json:"engagements"`
}

// SessionHandler resolves the verified caller to a Client (via
// client_portal_users) and reports their Engagements. It runs before any
// Engagement is chosen, so -- like clientauth.Middleware before it checks
// Engagement ownership -- it only ever sets app.current_identity_uid and
// then app.current_client_id, never anything scoped by :engagementId.
func SessionHandler(verifier authn.Verifier, db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, uid, ok := authn.Begin(w, r, verifier, db)
		if !ok {
			return
		}
		defer func() { _ = tx.Rollback() }()

		resp, status, msg := session(r, tx, uid)
		if status != http.StatusOK {
			http.Error(w, msg, status)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
		}
	})
}

func session(r *http.Request, tx *sql.Tx, identityUID string) (SessionResponse, int, string) {
	ctx := r.Context()

	clientID, found, err := setIdentityAndResolveClient(ctx, tx, identityUID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SessionResponse{}, http.StatusInternalServerError, MsgInternalError
	}
	if !found {
		return SessionResponse{}, http.StatusNotFound, "no matching client account"
	}

	// The Client's own Engagements are always visible to them, so -- unlike
	// clientauth.Middleware, which only sets app.current_client_id once
	// it's confirmed a specific :engagementId belongs to the caller --
	// this can set it unconditionally right after identity resolution.
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_client_id', $1, true)`, clientID); err != nil {
		return SessionResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	engagements, err := listEngagements(ctx, tx, clientID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SessionResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	return SessionResponse{ClientID: clientID, Engagements: engagements}, http.StatusOK, ""
}

func listEngagements(ctx context.Context, tx *sql.Tx, clientID string) ([]EngagementSummary, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT e.id, p.name, e.status
		 FROM engagements e
		 JOIN practices p ON p.id = e.practice_id
		 WHERE e.client_id = $1
		 ORDER BY e.created_at`,
		clientID,
	)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return nil, fmt.Errorf("clientauth: list engagements: %w", err)
	}
	defer func() { _ = rows.Close() }()

	engagements := []EngagementSummary{}
	for rows.Next() {
		var e EngagementSummary
		if err := rows.Scan(&e.EngagementID, &e.PracticeName, &e.Status); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("clientauth: scan engagement row: %w", err)
		}
		engagements = append(engagements, e)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("clientauth: iterate engagement rows: %w", err)
	}
	return engagements, nil
}
