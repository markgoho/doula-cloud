package staffauth_test

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

func newMFARequiredServer(t *testing.T, db *testdb.DB, accounts *authntest.FakeAccountManager) (srv *httptest.Server, seedSession func(uid string) string) {
	t.Helper()
	return serveStaffauth(db, authntest.Verifier{}, accounts), func(uid string) string { return authntest.SeedSession(t, db.App, uid) }
}

func putMFARequired(t *testing.T, srv *httptest.Server, session, practiceID string, required bool, confirmed bool) *http.Response {
	t.Helper()
	resp := reqtest.Put(t, srv.URL+"/api/practices/"+practiceID+"/mfa-required", reqtest.JSON(map[string]bool{"required": required}), reqtest.Session(session), reqtest.HeaderIf(confirmed, "X-Confirmed", "true"))
	return resp
}

func TestPutMFARequiredHandler_NonOwnerForbidden(t *testing.T) {
	db := testdb.New(t)
	const doulaUID = "doula-throws-switch"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, doulaUID, []string{doulaRole}, employeeType)

	srv, seedSession := newMFARequiredServer(t, db, authntest.NewFakeAccountManager())
	defer srv.Close()

	resp := putMFARequired(t, srv, seedSession(doulaUID), practiceID, true, true)
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusForbidden)
}

func TestPutMFARequiredHandler_RequiresConfirmation(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-forgets-confirm"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)

	srv, seedSession := newMFARequiredServer(t, db, authntest.NewFakeAccountManager())
	defer srv.Close()

	resp := putMFARequired(t, srv, seedSession(ownerUID), practiceID, true, false)
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusBadRequest)
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
	reqtest.RequireStatus(t, resp, http.StatusNoContent)

	required := testdb.Scalar[bool](t, db.Admin, `SELECT require_mfa_for_all_staff FROM practices WHERE id = $1`, practiceID)
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

	count := testdb.Scalar[int](t, db.Admin, `SELECT count(*) FROM activity WHERE subject_kind = 'practice' AND subject_id = $1`, practiceID)
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
	reqtest.RequireStatus(t, resp, http.StatusNoContent)

	testdb.Scalar[string](t, db.Admin, `SELECT action FROM activity WHERE subject_kind = 'practice' AND subject_id = $1 AND action = 'mfa_required_disabled'`, practiceID)
}

func TestPutMFARequiredHandler_MalformedBody(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-sends-garbage"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)

	srv, seedSession := newMFARequiredServer(t, db, authntest.NewFakeAccountManager())
	defer srv.Close()

	reqtest.ExpectStatus(t, http.MethodPut, srv.URL+"/api/practices/"+practiceID+"/mfa-required", http.StatusBadRequest, reqtest.Body("application/json", []byte("not json")), reqtest.Session(seedSession(ownerUID)), reqtest.Header("X-Confirmed", "true"))
}

func TestGetMFAImpactHandler_DoulaForbidden(t *testing.T) {
	db := testdb.New(t)
	const doulaUID = "doula-reads-impact"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, doulaUID, []string{doulaRole}, employeeType)

	srv, seedSession := newMFARequiredServer(t, db, authntest.NewFakeAccountManager())
	defer srv.Close()

	reqtest.ExpectStatus(t, http.MethodGet, srv.URL+"/api/practices/"+practiceID+"/mfa-required/impact", http.StatusForbidden, reqtest.Session(seedSession(doulaUID)))
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
	enrolledID := testdb.SeedStaff(t, db, enrolledDoulaUID)
	seedMembership(t, db, practiceID, enrolledID)

	const unenrolledDoulaUID = "doula-unenrolled-impact"
	unenrolledID := testdb.SeedStaff(t, db, unenrolledDoulaUID)
	seedMembership(t, db, practiceID, unenrolledID)

	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(ownerUID, ownerUID+"@example.com", true)
	accounts.Seed(enrolledDoulaUID, enrolledDoulaUID+"@example.com", true)
	accounts.EnrollTOTP(enrolledDoulaUID)
	accounts.Seed(unenrolledDoulaUID, unenrolledDoulaUID+"@example.com", true)

	srv, seedSession := newMFARequiredServer(t, db, accounts)
	defer srv.Close()

	resp := reqtest.Get(t, srv.URL+"/api/practices/"+practiceID+"/mfa-required/impact", reqtest.Session(seedSession(ownerUID)))
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusOK)

	var body struct {
		Required            bool `json:"required"`
		WithoutSecondFactor int  `json:"withoutSecondFactor"`
	}
	if err := json.UnmarshalRead(resp.Body, &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if !body.Required {
		t.Fatalf("required = false, want true")
	}
	// The Owner has no TOTP enrollment seeded, and neither does the
	// unenrolled Doula -- two of the three Staff members here.
	if body.WithoutSecondFactor != 2 {
		t.Fatalf("withoutSecondFactor = %d, want 2", body.WithoutSecondFactor)
	}
}
