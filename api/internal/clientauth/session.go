package clientauth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/apierr"
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
// picker otherwise -- #312: a Portal Account reaches many Clients, at
// most one per Practice (ADR-0015), so there is no single "the" Client to
// report here any more, only the Engagements those Clients hold.
type SessionResponse struct {
	// SignInAddress is the address her Portal Account signs in with
	// today (#619): the change screen has to show her which mailbox that
	// is before asking for a new one, and this is the only read that
	// tells it. Additive to the contract, and it discloses nothing --
	// she just signed in with it.
	SignInAddress string              `json:"signInAddress"`
	Engagements   []EngagementSummary `json:"engagements"`
}

// SessionHandler resolves the verified caller's identity and reports every
// Engagement it reaches, across every Client and Practice (ADR-0015: one
// Portal Account, many Clients). It runs before any Engagement is chosen,
// so -- like clientauth.Middleware before it checks Engagement ownership --
// it only ever sets app.current_identity_uid, and #312 deliberately never
// sets app.current_client_id here: that variable would narrow the read to
// one Client, which is the bug this handler exists to not have.
func SessionHandler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, uid, _, ok := authn.Begin(w, r, db)
		if !ok {
			return
		}
		defer func() { _ = tx.Rollback() }()

		resp, status, msg := session(r, tx, uid)
		if status != http.StatusOK {
			apierr.WriteError(w, msg, status)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, resp)
	})
}

func session(r *http.Request, tx *sql.Tx, identityUID string) (SessionResponse, int, string) {
	ctx := r.Context()

	found, err := setIdentityAndCheckClient(ctx, tx, identityUID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SessionResponse{}, http.StatusInternalServerError, apierr.MsgInternalError
	}
	if !found {
		return SessionResponse{}, http.StatusNotFound, "no matching client account"
	}

	// Deliberately never sets app.current_client_id: #312's identity-tier
	// policy, engagements_identity_visibility (00082), only matches while
	// that variable is unset, and it is what makes this list span every
	// Client her Portal Account reaches -- one per Practice (ADR-0015) --
	// rather than whichever single row a resolver picked first.
	engagements, err := listEngagementsForIdentity(ctx, tx)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SessionResponse{}, http.StatusInternalServerError, apierr.MsgInternalError
	}

	// Read through portal_accounts_signin_lookup (00074), the same
	// USING (true) SELECT policy the magic-link request reads through --
	// portal_accounts carries no Practice or Client column to scope a
	// policy of its own against.
	var signInAddress string
	if err := tx.QueryRowContext(ctx, `SELECT sign_in_address FROM portal_accounts WHERE identifier = $1`, identityUID).Scan(&signInAddress); err != nil && !errors.Is(err, sql.ErrNoRows) {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SessionResponse{}, http.StatusInternalServerError, apierr.MsgInternalError
	}

	return SessionResponse{SignInAddress: signInAddress, Engagements: engagements}, http.StatusOK, ""
}

// setIdentityAndCheckClient sets app.current_identity_uid -- the session
// variable client_portal_users' self-visibility RLS policy reads -- then
// reports whether identityUID reaches any Client at all. It deliberately
// stops there rather than resolving a single client_id: #312, a Portal
// Account can reach many Clients, one per Practice (ADR-0015), and this
// endpoint has no single Engagement addressed yet to narrow to one.
func setIdentityAndCheckClient(ctx context.Context, tx *sql.Tx, identityUID string) (bool, error) {
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_identity_uid', $1, true)`, identityUID); err != nil {
		return false, fmt.Errorf("clientauth: set current identity uid: %w", err)
	}

	var found bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM client_portal_users WHERE identity_uid = $1)`, identityUID).Scan(&found); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, fmt.Errorf("clientauth: check client account: %w", err)
	}
	return found, nil
}

// listEngagementsForIdentity reads every Engagement app.current_identity_uid
// reaches, across every Practice, relying entirely on
// engagements_identity_visibility (00082) to scope the rows -- there is no
// WHERE clause of its own to get wrong.
func listEngagementsForIdentity(ctx context.Context, tx *sql.Tx) ([]EngagementSummary, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT e.id, p.name, e.status
		 FROM engagements e
		 JOIN practices p ON p.id = e.practice_id
		 ORDER BY e.created_at`,
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
