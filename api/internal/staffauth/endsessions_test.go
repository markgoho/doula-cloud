package staffauth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

func newEndSessionsServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("DELETE /practices/{practiceId}/staff/{staffId}/sessions",
		staffauth.Middleware(db.App)(staffauth.EndSessionsHandler()))
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func deleteSessions(t *testing.T, srv *httptest.Server, session string, practiceID, staffID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodDelete,
		srv.URL+"/practices/"+practiceID+"/staff/"+staffID+"/sessions", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func TestEndSessionsHandler_NonOwnerForbidden(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-ending-sessions"
	staffID, practiceID := seedStaffWithMembership(t, db, identityUID) // '{doula}', not owner

	srv, session := newEndSessionsServer(t, db, identityUID)
	defer srv.Close()

	resp := deleteSessions(t, srv, session, practiceID, staffID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestEndSessionsHandler_OwnerAtDifferentPracticeForbidden covers the AC
// that an Owner elsewhere is refused: staffauth.Middleware itself rejects
// them before RequireOwner ever runs, since they hold no membership at
// the Practice named in the URL.
func TestEndSessionsHandler_OwnerAtDifferentPracticeForbidden(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-elsewhere"
	seedOwnerMembership(t, db, ownerUID) // owns a different Practice

	const targetUID = "target-at-other-practice"
	targetID, otherPracticeID := seedStaffWithMembership(t, db, targetUID)

	srv, session := newEndSessionsServer(t, db, ownerUID)
	defer srv.Close()

	resp := deleteSessions(t, srv, session, otherPracticeID, targetID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestEndSessionsHandler_NoSuchMembership(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-no-such-target"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)

	srv, session := newEndSessionsServer(t, db, ownerUID)
	defer srv.Close()

	resp := deleteSessions(t, srv, session, practiceID, "00000000-0000-0000-0000-000000000000")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestEndSessionsHandler_Success is the ticket's core behavior: every
// session the target holds, across devices, is gone, while the acting
// Owner's own session is untouched.
func TestEndSessionsHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-ends-sessions"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)

	const targetUID = "staff-offboarded"
	targetID := seedStaff(t, db, targetUID)
	seedMembership(t, db, practiceID, targetID)
	authntest.SeedSession(t, db.App, targetUID) // laptop
	authntest.SeedSession(t, db.App, targetUID) // phone

	srv, session := newEndSessionsServer(t, db, ownerUID)
	defer srv.Close()

	resp := deleteSessions(t, srv, session, practiceID, targetID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	if got := authntest.CountFor(t, db.App, targetUID); got != 0 {
		t.Fatalf("target session rows = %d, want 0", got)
	}
	if got := authntest.CountFor(t, db.App, ownerUID); got != 1 {
		t.Fatalf("acting owner's own session rows = %d, want 1 (unaffected)", got)
	}
}
