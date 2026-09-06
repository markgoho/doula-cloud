package sessionmint_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/portalaccount"
	"doula-cloud/api/internal/sessionmint"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// okStep is a Step that mints identityUID with no business work of its
// own -- the shape session.CreateHandler's step takes.
func okStep(identityUID string) sessionmint.Step {
	return func(context.Context, *sql.Tx) (sessionmint.Result, error) {
		return sessionmint.Result{IdentityUID: identityUID, Body: struct {
			OK bool `json:"ok"`
		}{true}}, nil
	}
}

// issue runs Issue on its own transaction and request, carrying
// cookieToken as the __session cookie (when not empty) and X-Confirmed
// when confirmed is true.
func issue(t *testing.T, db *testdb.DB, cookieToken string, confirmed bool, adapter sessionmint.Adapter, step sessionmint.Step, finish sessionmint.Finish) (rec *httptest.ResponseRecorder, enq *tasknudge.FakeEnqueuer, committed bool) {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	if cookieToken != "" {
		authntest.AddSessionCookie(req, cookieToken)
	}
	if confirmed {
		req.Header.Set("X-Confirmed", "true")
	}
	rec = httptest.NewRecorder()
	enq = &tasknudge.FakeEnqueuer{}

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	committed = sessionmint.Issue(rec, req, tx, enq, adapter, step, finish)
	return rec, enq, committed
}

// postMint POSTs to srv's own /mint route with no body, the shape every
// IssueFromDB test in this file drives its server through.
func postMint(t *testing.T, srv *httptest.Server) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/mint", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	return resp
}

func countSessions(t *testing.T, db *testdb.DB, identityUID string) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM sessions WHERE identity_uid = $1`, identityUID,
	).Scan(&count); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	return count
}

func TestIssue_MintsForBothTiers(t *testing.T) {
	for _, tc := range []struct {
		name    string
		adapter sessionmint.Adapter
		uid     string
	}{
		{"staff", sessionmint.Staff(authn.VerifiedToken{UID: "staff-1", SecondFactor: true}), "staff-1"},
		{"portal", sessionmint.Portal(), portalaccount.NewIdentifier()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)

			rec, _, committed := issue(t, db, "", false, tc.adapter, okStep(tc.uid), nil)

			if !committed {
				t.Fatal("committed = false, want true")
			}
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			var cookieSet bool
			for _, c := range rec.Result().Cookies() {
				if c.Name == authn.SessionCookieName {
					cookieSet = true
				}
			}
			if !cookieSet {
				t.Error("no session cookie set")
			}
			if got := countSessions(t, db, tc.uid); got != 1 {
				t.Errorf("session rows for %s = %d, want 1", tc.uid, got)
			}
		})
	}
}

func TestIssue_CrossPopulationUnconfirmedRefusesAndMintsNothing(t *testing.T) {
	db := testdb.New(t)
	const staffUID = "staff-uid"
	token := authntest.SeedSession(t, db.App, staffUID)
	const portalUID = "portal-new"

	rec, _, committed := issue(t, db, token, false, sessionmint.Portal(), okStep(portalUID), nil)

	if committed {
		t.Fatal("committed = true, want false for an unconfirmed eviction")
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
	var out apierr.APIError
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if out.Code != string(authn.EvictionUnconfirmed) {
		t.Errorf("code = %q, want %q", out.Code, authn.EvictionUnconfirmed)
	}
	if got := countSessions(t, db, staffUID); got != 1 {
		t.Errorf("session rows for %s = %d, want 1 -- a refusal evicts nothing", staffUID, got)
	}
	if got := countSessions(t, db, portalUID); got != 0 {
		t.Errorf("session rows for %s = %d, want 0 -- refused, nothing minted", portalUID, got)
	}
}

func TestIssue_CrossPopulationConfirmedEvictsAndMints(t *testing.T) {
	db := testdb.New(t)
	const staffUID = "staff-uid"
	token := authntest.SeedSession(t, db.App, staffUID)
	const portalUID = "portal-new"

	rec, enq, committed := issue(t, db, token, true, sessionmint.Portal(), okStep(portalUID), nil)

	if !committed {
		t.Fatal("committed = false, want true once confirmed")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := countSessions(t, db, staffUID); got != 0 {
		t.Errorf("session rows for %s = %d, want 0 -- evicted", staffUID, got)
	}
	if got := countSessions(t, db, portalUID); got != 1 {
		t.Errorf("session rows for %s = %d, want 1", portalUID, got)
	}
	if len(enq.Calls()) != 1 {
		t.Errorf("nudges fired = %d, want 1 -- an evicted Staff session is notified", len(enq.Calls()))
	}
}

// TestIssue_SamePopulationEndsThePriorSessionSilently is #816's trap:
// mfaenroll always minted over its own pre-enrolment session, and every
// other seam left that row to expire uncleaned. Issue now ends it for
// every seam, without a warning, since nothing crosses a population.
func TestIssue_SamePopulationEndsThePriorSessionSilently(t *testing.T) {
	db := testdb.New(t)
	const staffUID = "staff-uid"
	token := authntest.SeedSession(t, db.App, staffUID)

	rec, enq, committed := issue(t, db, token, false, sessionmint.Staff(authn.VerifiedToken{UID: staffUID, SecondFactor: true}), okStep(staffUID), nil)

	if !committed {
		t.Fatal("committed = false, want true -- same population needs no confirmation")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := countSessions(t, db, staffUID); got != 1 {
		t.Errorf("session rows for %s = %d, want 1 (old ended, new minted)", staffUID, got)
	}
	if len(enq.Calls()) != 0 {
		t.Errorf("nudges fired = %d, want 0 -- a same-population replacement is silent", len(enq.Calls()))
	}
}

// seedSessionOnTx inserts a session row on tx itself, standing in for
// whatever a real step writes on its own transaction -- sessions carries
// no RLS, so this exercises rollback/commit without a policy getting in
// the way of a test that isn't about RLS at all.
func seedSessionOnTx(ctx context.Context, t *testing.T, tx *sql.Tx, identityUID string) {
	t.Helper()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, identity_uid, expires_at, second_factor) VALUES ($1, $2, now() + interval '1 hour', false)`,
		"seed-"+identityUID, identityUID,
	); err != nil {
		t.Fatalf("seed session on tx: %v", err)
	}
}

func TestIssue_RefusalRollsBackWhatStepWrote(t *testing.T) {
	db := testdb.New(t)
	step := func(ctx context.Context, tx *sql.Tx) (sessionmint.Result, error) {
		seedSessionOnTx(ctx, t, tx, "refused-uid")
		return sessionmint.Result{Refusal: &sessionmint.Refusal{Status: http.StatusBadRequest, Message: "no"}}, nil
	}

	rec, _, committed := issue(t, db, "", false, sessionmint.Staff(authn.VerifiedToken{UID: "refused-uid"}), step, nil)

	if committed {
		t.Fatal("committed = true, want false")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if got := countSessions(t, db, "refused-uid"); got != 0 {
		t.Errorf("session rows = %d, want 0 -- a rolled-back refusal leaves nothing behind", got)
	}
}

// TestIssue_KeptRefusalCommitsWhatStepWrote covers staffauth's own
// expired-invitation branch: the refusal itself is worth keeping despite
// minting nothing.
func TestIssue_KeptRefusalCommitsWhatStepWrote(t *testing.T) {
	db := testdb.New(t)
	step := func(ctx context.Context, tx *sql.Tx) (sessionmint.Result, error) {
		seedSessionOnTx(ctx, t, tx, "kept-uid")
		return sessionmint.Result{Refusal: &sessionmint.Refusal{Status: http.StatusGone, Message: "expired", Keep: true}}, nil
	}

	rec, _, committed := issue(t, db, "", false, sessionmint.Staff(authn.VerifiedToken{UID: "kept-uid"}), step, nil)

	if !committed {
		t.Fatal("committed = false, want true -- Keep asks for a commit")
	}
	if rec.Code != http.StatusGone {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusGone)
	}
	if got := countSessions(t, db, "kept-uid"); got != 1 {
		t.Errorf("session rows = %d, want 1 -- Keep commits what the step wrote", got)
	}
}

func TestIssue_StepErrorRollsBackAndWrites500(t *testing.T) {
	db := testdb.New(t)
	step := func(context.Context, *sql.Tx) (sessionmint.Result, error) {
		return sessionmint.Result{}, errors.New("boom")
	}

	rec, _, committed := issue(t, db, "", false, sessionmint.Portal(), step, nil)

	if committed {
		t.Fatal("committed = true, want false")
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestIssue_RunsFinishAfterMintBeforeCommit(t *testing.T) {
	db := testdb.New(t)
	const portalUID = "portal-finish"
	var finishRan bool
	finish := func(ctx context.Context, tx *sql.Tx) error {
		finishRan = true
		// Queried on tx itself, not db.Admin: the mint has not committed
		// yet, so a separate connection cannot see it.
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM sessions WHERE identity_uid = $1`, portalUID).Scan(&count); err != nil {
			t.Fatalf("count sessions on tx: %v", err)
		}
		if count != 1 {
			t.Errorf("session rows at finish time = %d, want 1 -- finish runs after mint", count)
		}
		return nil
	}

	_, _, committed := issue(t, db, "", false, sessionmint.Portal(), okStep(portalUID), finish)

	if !committed {
		t.Fatal("committed = false, want true")
	}
	if !finishRan {
		t.Error("finish did not run")
	}
}

func TestIssue_FinishErrorRollsBackTheMint(t *testing.T) {
	db := testdb.New(t)
	const portalUID = "portal-finish-fails"
	finish := func(context.Context, *sql.Tx) error {
		return errors.New("boom")
	}

	rec, _, committed := issue(t, db, "", false, sessionmint.Portal(), okStep(portalUID), finish)

	if committed {
		t.Fatal("committed = true, want false")
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestIssueFromDB_OpensCommitsAndRollsBackItsOwnTransaction(t *testing.T) {
	db := testdb.New(t)
	const portalUID = "portal-fromdb"
	mux := http.NewServeMux()
	mux.HandleFunc("POST /mint", func(w http.ResponseWriter, r *http.Request) {
		sessionmint.IssueFromDB(w, r, db.App, &tasknudge.FakeEnqueuer{}, sessionmint.Portal(), okStep(portalUID), nil)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp := postMint(t, srv)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := countSessions(t, db, portalUID); got != 1 {
		t.Errorf("session rows for %s = %d, want 1", portalUID, got)
	}
}

func TestIssueFromDB_RollsBackOnRefusal(t *testing.T) {
	db := testdb.New(t)
	step := func(context.Context, *sql.Tx) (sessionmint.Result, error) {
		return sessionmint.Result{Refusal: &sessionmint.Refusal{Status: http.StatusBadRequest, Message: "no"}}, nil
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /mint", func(w http.ResponseWriter, r *http.Request) {
		sessionmint.IssueFromDB(w, r, db.App, &tasknudge.FakeEnqueuer{}, sessionmint.Portal(), step, nil)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp := postMint(t, srv)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestIssue_ExplicitStatusIsWritten(t *testing.T) {
	db := testdb.New(t)
	const uid = "created-uid"
	step := func(context.Context, *sql.Tx) (sessionmint.Result, error) {
		return sessionmint.Result{IdentityUID: uid, Body: struct {
			ID string `json:"id"`
		}{"x"}, Status: http.StatusCreated}, nil
	}

	rec, _, committed := issue(t, db, "", false, sessionmint.Portal(), step, nil)

	if !committed {
		t.Fatal("committed = false, want true")
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
}
