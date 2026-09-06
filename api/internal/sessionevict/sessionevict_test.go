package sessionevict_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/portalaccount"
	"doula-cloud/api/internal/sessionevict"
	"doula-cloud/api/internal/testdb"
)

// apply runs Apply on its own transaction, the way a mint seam runs it
// inside the one it is already holding, and commits when it says the
// caller may go on. Returns the recorder so a refusal can be read back.
func apply(
	t *testing.T, db *testdb.DB, cookieToken string, isConfirmed bool, minting authn.Tier,
) (rec *httptest.ResponseRecorder, queued, ok bool) {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	if cookieToken != "" {
		authntest.AddSessionCookie(req, cookieToken)
	}
	if isConfirmed {
		req.Header.Set("X-Confirmed", "true")
	}
	rec = httptest.NewRecorder()

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	queued, ok = sessionevict.Apply(rec, req, tx, minting, time.Now())
	if ok {
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
	}
	return rec, queued, ok
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

func countEvictionNotices(t *testing.T, db *testdb.DB, identityUID string) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM session_notice_outbox WHERE identity_uid = $1 AND kind = 'session_evicted'`,
		identityUID,
	).Scan(&count); err != nil {
		t.Fatalf("count notices: %v", err)
	}
	return count
}

func TestApply_NothingToEvictLetsTheCallerMint(t *testing.T) {
	db := testdb.New(t)

	rec, queued, ok := apply(t, db, "", false, authn.TierStaff)

	if !ok {
		t.Fatal("ok = false, want true when the caller holds no session")
	}
	if queued {
		t.Error("queued = true, want false -- nothing was evicted")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want no response written", rec.Code)
	}
}

func TestApply_UnconfirmedRefusesAndLeavesTheSessionAlone(t *testing.T) {
	db := testdb.New(t)
	const staffUID = "staff-uid"
	token := authntest.SeedSession(t, db.App, staffUID)

	rec, _, ok := apply(t, db, token, false, authn.TierPortal)

	if ok {
		t.Fatal("ok = true, want false for an unconfirmed eviction")
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
	var out apierr.APIError
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if out.Code != string(authn.EvictionUnconfirmed) {
		t.Fatalf("code = %q, want %q", out.Code, authn.EvictionUnconfirmed)
	}
	if got := countSessions(t, db, staffUID); got != 1 {
		t.Errorf("session rows = %d, want 1 -- a refusal evicts nothing", got)
	}
}

func TestApply_ConfirmedDeletesTheSessionAndQueuesItsNotice(t *testing.T) {
	db := testdb.New(t)
	const staffUID = "staff-uid"
	token := authntest.SeedSession(t, db.App, staffUID)

	_, queued, ok := apply(t, db, token, true, authn.TierPortal)

	if !ok {
		t.Fatal("ok = false, want true once the caller has confirmed")
	}
	if !queued {
		t.Error("queued = false, want true -- an evicted Staff session is notified")
	}
	if got := countSessions(t, db, staffUID); got != 0 {
		t.Errorf("session rows = %d, want 0 -- an evicted token must stop verifying", got)
	}
	if got := countEvictionNotices(t, db, staffUID); got != 1 {
		t.Errorf("eviction notices = %d, want 1", got)
	}
}

// An evicted Client is deleted like any other, and sends no mail --
// sessionnotice.QueueSessionEvicted records why.
func TestApply_ConfirmedPortalEvictionQueuesNoNotice(t *testing.T) {
	db := testdb.New(t)
	portalUID := portalaccount.NewIdentifier()
	token := authntest.SeedSession(t, db.App, portalUID)

	_, queued, ok := apply(t, db, token, true, authn.TierStaff)

	if !ok {
		t.Fatal("ok = false, want true once the caller has confirmed")
	}
	if queued {
		t.Error("queued = true, want false for an evicted Client")
	}
	if got := countSessions(t, db, portalUID); got != 0 {
		t.Errorf("session rows = %d, want 0", got)
	}
}
