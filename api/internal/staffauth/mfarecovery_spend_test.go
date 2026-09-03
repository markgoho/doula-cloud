package staffauth_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/authtoken"
	"doula-cloud/api/internal/mfarecoverymail"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

func newSpendServer(accounts *authntest.FakeAccountManager, db *testdb.DB) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /staff/mfa-recovery/spend", staffauth.SpendMFARecoveryHandler(accounts, db.App))
	return httptest.NewServer(mux)
}

// seedIssuedCode mints a live Owner-vouched issued code for targetUID and
// records ownerStaffID as its voucher, the same two writes VouchHandler
// makes in one request -- done here directly so spend tests don't depend
// on the vouch endpoint working.
func seedIssuedCode(t *testing.T, db *testdb.DB, targetUID, targetStaffID, ownerStaffID string) (code string) {
	t.Helper()
	code, err := authtoken.MintCode(t.Context(), db.App, targetUID, authtoken.PurposeStaffMFARecovery, mfarecoverymail.CodeLifetime, time.Now())
	if err != nil {
		t.Fatalf("MintCode: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO staff_mfa_recovery_vouches (token_hash, staff_id, owner_staff_id) VALUES ($1, $2, $3)`,
		authtoken.Digest(code), targetStaffID, ownerStaffID,
	); err != nil {
		t.Fatalf("seed vouches row: %v", err)
	}
	return code
}

// seedSavedCode inserts one live saved code for staffID directly,
// bypassing reconcileSavedCodes/RotateSavedCodesHandler so spend tests
// don't depend on either working.
func seedSavedCode(t *testing.T, db *testdb.DB, staffID, code string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO staff_mfa_recovery_codes (staff_id, code_hash) VALUES ($1, $2)`,
		staffID, authtoken.Digest(code),
	); err != nil {
		t.Fatalf("seed saved code: %v", err)
	}
}

func liveSavedCodeCount(t *testing.T, db *testdb.DB, staffID string) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM staff_mfa_recovery_codes WHERE staff_id = $1 AND used_at IS NULL AND revoked_at IS NULL`, staffID,
	).Scan(&count); err != nil {
		t.Fatalf("count live saved codes: %v", err)
	}
	return count
}

func TestSpendMFARecoveryHandler_InvalidRequestBody(t *testing.T) {
	db := testdb.New(t)
	srv := newSpendServer(authntest.NewFakeAccountManager(), db)
	defer srv.Close()

	resp := postJSONTo(t, srv, "/staff/mfa-recovery/spend", `not json`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestSpendMFARecoveryHandler_UnknownAddress(t *testing.T) {
	db := testdb.New(t)
	srv := newSpendServer(authntest.NewFakeAccountManager(), db)
	defer srv.Close()

	resp := postJSONTo(t, srv, "/staff/mfa-recovery/spend", `{"email":"nobody@example.com","code":"12345678"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestSpendMFARecoveryHandler_UnknownAddressAndWrongCodeAnswerIdentically
// is #168's account-enumeration rule, the AC's own words: a known address
// with a wrong code must read exactly like an unknown address.
func TestSpendMFARecoveryHandler_UnknownAddressAndWrongCodeAnswerIdentically(t *testing.T) {
	db := testdb.New(t)
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed("known-uid", "known@example.com", true)
	srv := newSpendServer(accounts, db)
	defer srv.Close()

	unknown := postJSONTo(t, srv, "/staff/mfa-recovery/spend", `{"email":"nobody@example.com","code":"12345678"}`)
	defer unknown.Body.Close()
	wrongCode := postJSONTo(t, srv, "/staff/mfa-recovery/spend", `{"email":"known@example.com","code":"12345678"}`)
	defer wrongCode.Body.Close()

	if unknown.StatusCode != wrongCode.StatusCode {
		t.Fatalf("status codes differ: unknown=%d wrongCode=%d", unknown.StatusCode, wrongCode.StatusCode)
	}
	unknownBody, err1 := readBody(unknown)
	wrongCodeBody, err2 := readBody(wrongCode)
	if err1 != nil || err2 != nil {
		t.Fatalf("read bodies: %v / %v", err1, err2)
	}
	if unknownBody != wrongCodeBody {
		t.Fatalf("bodies differ: unknown=%q wrongCode=%q", unknownBody, wrongCodeBody)
	}
}

func TestSpendMFARecoveryHandler_MissingFields(t *testing.T) {
	db := testdb.New(t)
	srv := newSpendServer(authntest.NewFakeAccountManager(), db)
	defer srv.Close()

	resp := postJSONTo(t, srv, "/staff/mfa-recovery/spend", `{"email":"","code":""}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestSpendMFARecoveryHandler_IssuedCode covers #605's Owner-vouch path
// end to end: the enrolment clears, no session is minted, every existing
// session ends, and the audit row names the vouching Owner.
func TestSpendMFARecoveryHandler_IssuedCode(t *testing.T) {
	db := testdb.New(t)
	const targetUID = "target-spend-issued"
	targetID := seedStaffWithEmail(t, db, targetUID, "target-issued@example.com")
	const ownerUID = "owner-spend-issued"
	ownerID := seedStaff(t, db, ownerUID)

	code := seedIssuedCode(t, db, targetUID, targetID, ownerID)
	authntest.SeedSession(t, db.App, targetUID)

	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(targetUID, "target-issued@example.com", true)
	accounts.EnrollTOTP(targetUID)
	srv := newSpendServer(accounts, db)
	defer srv.Close()

	resp := postJSONTo(t, srv, "/staff/mfa-recovery/spend", `{"email":"target-issued@example.com","code":"`+code+`"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	if resp.Header.Get("Set-Cookie") != "" {
		t.Fatalf("Set-Cookie = %q, want no session minted", resp.Header.Get("Set-Cookie"))
	}
	if accounts.HasSecondFactor(targetUID) {
		t.Fatal("TOTP enrolment still present, want it cleared")
	}
	if got := authntest.CountFor(t, db.App, targetUID); got != 0 {
		t.Fatalf("session rows = %d, want 0", got)
	}

	var reason, actorStaffID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT reason::text, actor_staff_id FROM staff_auth_events WHERE staff_id = $1`, targetID,
	).Scan(&reason, &actorStaffID); err != nil {
		t.Fatalf("query staff_auth_events: %v", err)
	}
	if reason != "owner_vouched" || actorStaffID != ownerID {
		t.Fatalf("event = (%q, %q), want (owner_vouched, %q)", reason, actorStaffID, ownerID)
	}
}

// TestSpendMFARecoveryHandler_ClearSecondFactorsFailureIs500 covers the
// Admin SDK failure branch clearEnrolmentAndRecord's first call can take
// -- proving a spend that cannot actually clear the factor never reaches
// 204, rather than silently succeeding.
func TestSpendMFARecoveryHandler_ClearSecondFactorsFailureIs500(t *testing.T) {
	db := testdb.New(t)
	const targetUID = "target-spend-clear-fails"
	targetID := seedStaffWithEmail(t, db, targetUID, "target-clear-fails@example.com")
	const ownerUID = "owner-spend-clear-fails"
	ownerID := seedStaff(t, db, ownerUID)
	code := seedIssuedCode(t, db, targetUID, targetID, ownerID)

	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(targetUID, "target-clear-fails@example.com", true)
	accounts.ClearSecondFactorsErr = errors.New("admin sdk unreachable")
	srv := newSpendServer(accounts, db)
	defer srv.Close()

	resp := postJSONTo(t, srv, "/staff/mfa-recovery/spend", `{"email":"target-clear-fails@example.com","code":"`+code+`"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
}

func TestSpendMFARecoveryHandler_IssuedCodeIsSingleUse(t *testing.T) {
	db := testdb.New(t)
	const targetUID = "target-spend-single-use"
	targetID := seedStaffWithEmail(t, db, targetUID, "target-single-use@example.com")
	const ownerUID = "owner-spend-single-use"
	ownerID := seedStaff(t, db, ownerUID)
	code := seedIssuedCode(t, db, targetUID, targetID, ownerID)

	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(targetUID, "target-single-use@example.com", true)
	srv := newSpendServer(accounts, db)
	defer srv.Close()

	first := postJSONTo(t, srv, "/staff/mfa-recovery/spend", `{"email":"target-single-use@example.com","code":"`+code+`"}`)
	defer first.Body.Close()
	if first.StatusCode != http.StatusNoContent {
		t.Fatalf("first spend status = %d, want 204", first.StatusCode)
	}

	second := postJSONTo(t, srv, "/staff/mfa-recovery/spend", `{"email":"target-single-use@example.com","code":"`+code+`"}`)
	defer second.Body.Close()
	if second.StatusCode != http.StatusBadRequest {
		t.Fatalf("second spend status = %d, want 400", second.StatusCode)
	}
}

// TestSpendMFARecoveryHandler_SavedCode covers the sole-Owner self-service
// path: the enrolment clears, the audit row names her as her own actor,
// and a replacement code keeps her live-code count unchanged.
func TestSpendMFARecoveryHandler_SavedCode(t *testing.T) {
	db := testdb.New(t)
	const targetUID = "target-spend-saved"
	targetID := seedStaffWithEmail(t, db, targetUID, "target-saved@example.com")
	const code = "a-saved-code-plaintext"
	seedSavedCode(t, db, targetID, code)

	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(targetUID, "target-saved@example.com", true)
	accounts.EnrollTOTP(targetUID)
	srv := newSpendServer(accounts, db)
	defer srv.Close()

	resp := postJSONTo(t, srv, "/staff/mfa-recovery/spend", `{"email":"target-saved@example.com","code":"`+code+`"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	if accounts.HasSecondFactor(targetUID) {
		t.Fatal("TOTP enrolment still present, want it cleared")
	}

	var reason, actorStaffID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT reason::text, actor_staff_id FROM staff_auth_events WHERE staff_id = $1`, targetID,
	).Scan(&reason, &actorStaffID); err != nil {
		t.Fatalf("query staff_auth_events: %v", err)
	}
	if reason != "self_service" || actorStaffID != targetID {
		t.Fatalf("event = (%q, %q), want (self_service, %q)", reason, actorStaffID, targetID)
	}

	if got := liveSavedCodeCount(t, db, targetID); got != 1 {
		t.Fatalf("live saved codes = %d, want 1 (the replacement)", got)
	}
}

func readBody(resp *http.Response) (string, error) {
	b, err := io.ReadAll(resp.Body)
	return string(b), err
}
