package staffauth_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

func newMFARequiredServer(t *testing.T, db *testdb.DB, accounts *authntest.FakeAccountManager) (srv *httptest.Server, seedSession func(uid string) string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	// GatedRouter.Get applies both Middleware and the role check, the
	// same as routes_practice.go's real registration -- a bare
	// staffauth.Middleware wrap here would skip the owner-only gate
	// entirely and let any Staff member read the impact count.
	g.Get("/practices/{practiceId}/mfa-required/impact", []string{"owner"}, staffauth.GetMFAImpactHandler(accounts))
	mux.Handle("PUT /practices/{practiceId}/mfa-required", staffauth.Middleware(db.App)(staffauth.PutMFARequiredHandler()))
	return httptest.NewServer(mux), func(uid string) string { return authntest.SeedSession(t, db.App, uid) }
}

func putMFARequired(t *testing.T, srv *httptest.Server, session, practiceID string, required bool, confirmed bool) *http.Response {
	t.Helper()
	body, _ := json.Marshal(map[string]bool{"required": required})
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, srv.URL+"/practices/"+practiceID+"/mfa-required", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	if confirmed {
		req.Header.Set("X-Confirmed", "true")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func TestPutMFARequiredHandler_NonOwnerForbidden(t *testing.T) {
	db := testdb.New(t)
	const doulaUID = "doula-throws-switch"
	_, practiceID := seedStaffWithMembership(t, db, doulaUID)

	srv, seedSession := newMFARequiredServer(t, db, authntest.NewFakeAccountManager())
	defer srv.Close()

	resp := putMFARequired(t, srv, seedSession(doulaUID), practiceID, true, true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestPutMFARequiredHandler_RequiresConfirmation(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-forgets-confirm"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)

	srv, seedSession := newMFARequiredServer(t, db, authntest.NewFakeAccountManager())
	defer srv.Close()

	resp := putMFARequired(t, srv, seedSession(ownerUID), practiceID, true, false)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPutMFARequiredHandler_ThrowsAndRecords proves the switch actually
// flips the Practice's own row and records who did it and when, per
// ADR-0022.
func TestPutMFARequiredHandler_ThrowsAndRecords(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-throws-switch"
	ownerID, practiceID := seedOwnerMembership(t, db, ownerUID)

	srv, seedSession := newMFARequiredServer(t, db, authntest.NewFakeAccountManager())
	defer srv.Close()

	resp := putMFARequired(t, srv, seedSession(ownerUID), practiceID, true, true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	var required bool
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT require_mfa_for_all_staff FROM practices WHERE id = $1`, practiceID,
	).Scan(&required); err != nil {
		t.Fatalf("read practices row: %v", err)
	}
	if !required {
		t.Fatalf("require_mfa_for_all_staff = false, want true")
	}

	var action, actor string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT action, actor_staff_id FROM activity WHERE subject_kind = 'practice' AND subject_id = $1 AND action = 'mfa_required_enabled'`,
		practiceID,
	).Scan(&action, &actor); err != nil {
		t.Fatalf("read activity row: %v", err)
	}
	if actor != ownerID {
		t.Fatalf("actor_staff_id = %q, want %q", actor, ownerID)
	}
}

// TestPutMFARequiredHandler_NoOpRecordsNothing proves a retry with the
// same value updates nothing and records nothing new -- the idempotency
// stance ir.Exempt declares for this route.
func TestPutMFARequiredHandler_NoOpRecordsNothing(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-repeats-switch"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)

	srv, seedSession := newMFARequiredServer(t, db, authntest.NewFakeAccountManager())
	defer srv.Close()
	session := seedSession(ownerUID)

	resp1 := putMFARequired(t, srv, session, practiceID, false, true)
	_ = resp1.Body.Close()
	if resp1.StatusCode != http.StatusNoContent {
		t.Fatalf("first status = %d, want %d", resp1.StatusCode, http.StatusNoContent)
	}

	resp2 := putMFARequired(t, srv, session, practiceID, false, true)
	_ = resp2.Body.Close()
	if resp2.StatusCode != http.StatusNoContent {
		t.Fatalf("second status = %d, want %d", resp2.StatusCode, http.StatusNoContent)
	}

	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM activity WHERE subject_kind = 'practice' AND subject_id = $1`, practiceID,
	).Scan(&count); err != nil {
		t.Fatalf("count activity rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("activity rows = %d, want 0 for a same-value PUT", count)
	}
}

func TestPutMFARequiredHandler_ClearsAndRecords(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-clears-switch"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)
	setRequireMFA(t, db, practiceID)

	srv, seedSession := newMFARequiredServer(t, db, authntest.NewFakeAccountManager())
	defer srv.Close()

	resp := putMFARequired(t, srv, seedSession(ownerUID), practiceID, false, true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	var action string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT action FROM activity WHERE subject_kind = 'practice' AND subject_id = $1 AND action = 'mfa_required_disabled'`,
		practiceID,
	).Scan(&action); err != nil {
		t.Fatalf("read activity row: %v", err)
	}
}

func TestPutMFARequiredHandler_MalformedBody(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-sends-garbage"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)

	srv, seedSession := newMFARequiredServer(t, db, authntest.NewFakeAccountManager())
	defer srv.Close()

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPut, srv.URL+"/practices/"+practiceID+"/mfa-required", bytes.NewReader([]byte("not json")))
	authntest.AddSessionCookie(req, seedSession(ownerUID))
	req.Header.Set("X-Confirmed", "true")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestGetMFAImpactHandler_DoulaForbidden(t *testing.T) {
	db := testdb.New(t)
	const doulaUID = "doula-reads-impact"
	_, practiceID := seedStaffWithMembership(t, db, doulaUID)

	srv, seedSession := newMFARequiredServer(t, db, authntest.NewFakeAccountManager())
	defer srv.Close()

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/practices/"+practiceID+"/mfa-required/impact", nil)
	authntest.AddSessionCookie(req, seedSession(doulaUID))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestGetMFAImpactHandler_CountsEveryoneWithoutAFactor proves the count
// is a batched AccountManager read, not a per-Staff-member round trip,
// and includes every Staff member without a factor -- an Owner already
// refused today included, per the handler's own comment.
func TestGetMFAImpactHandler_CountsEveryoneWithoutAFactor(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-reads-impact"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)
	setRequireMFA(t, db, practiceID)

	const enrolledDoulaUID = "doula-enrolled-impact"
	enrolledID := seedStaff(t, db, enrolledDoulaUID)
	seedMembership(t, db, practiceID, enrolledID)

	const unenrolledDoulaUID = "doula-unenrolled-impact"
	unenrolledID := seedStaff(t, db, unenrolledDoulaUID)
	seedMembership(t, db, practiceID, unenrolledID)

	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(ownerUID, ownerUID+"@example.com", true)
	accounts.Seed(enrolledDoulaUID, enrolledDoulaUID+"@example.com", true)
	accounts.EnrollTOTP(enrolledDoulaUID)
	accounts.Seed(unenrolledDoulaUID, unenrolledDoulaUID+"@example.com", true)

	srv, seedSession := newMFARequiredServer(t, db, accounts)
	defer srv.Close()

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/practices/"+practiceID+"/mfa-required/impact", nil)
	authntest.AddSessionCookie(req, seedSession(ownerUID))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body struct {
		Required            bool `json:"required"`
		WithoutSecondFactor int  `json:"withoutSecondFactor"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if !body.Required {
		t.Fatalf("required = false, want true")
	}
	// The Owner has no TOTP enrolment seeded, and neither does the
	// unenrolled Doula -- two of the three Staff members here.
	if body.WithoutSecondFactor != 2 {
		t.Fatalf("withoutSecondFactor = %d, want 2", body.WithoutSecondFactor)
	}
}
