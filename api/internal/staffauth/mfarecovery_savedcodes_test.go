package staffauth_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

func newRotateSavedCodesServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("POST /staff/mfa-recovery/saved-codes/rotate", staffauth.RotateSavedCodesHandler(db.App))
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func postRotate(t *testing.T, srv *httptest.Server, session string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/staff/mfa-recovery/saved-codes/rotate", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if session != "" {
		authntest.AddSessionCookie(req, session)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func liveSavedCodesFor(t *testing.T, db *testdb.DB, staffID string) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM staff_mfa_recovery_codes WHERE staff_id = $1 AND used_at IS NULL AND revoked_at IS NULL`, staffID,
	).Scan(&count); err != nil {
		t.Fatalf("count live saved codes: %v", err)
	}
	return count
}

func digestHex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// TestSignupHandler_MintsSavedCodesForFoundingOwner covers #615's AC:
// saved codes mint on the Membership event that makes a person a
// Practice's sole Owner, and a brand-new signup is that event too.
func TestSignupHandler_MintsSavedCodesForFoundingOwner(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(authntest.Verifier{UID: "founding-owner-uid", Email: "owner@example.com"}, db)
	defer srv.Close()

	resp := postSignup(t, srv, "tok", staffauth.SignupRequest{
		WorkState: "NY", PracticeName: "Founding Practice", StaffName: "Founder",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	var signed staffauth.SignupResponse
	if err := json.NewDecoder(resp.Body).Decode(&signed); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got := liveSavedCodesFor(t, db, signed.StaffID); got != 10 {
		t.Fatalf("live saved codes = %d, want 10", got)
	}
}

// TestAcceptInvite_SecondOwnerRevokesFirstOwnersSavedCodes covers #615's
// AC: a Practice dropping from sole to co-Owned revokes the codes, since
// the reason for holding them is gone. The new co-Owner never gets any
// -- she has an Owner above (beside) her from the moment she joins.
func TestAcceptInvite_SecondOwnerRevokesFirstOwnersSavedCodes(t *testing.T) {
	db := testdb.New(t)
	founderID, practiceID := seedOwnerMembership(t, db, "founder-two-owners")
	// seedOwnerMembership does not mint codes itself (it is a direct SQL
	// fixture, not the signup flow) -- seed the live-codes state a real
	// founding Owner would already hold, directly, so this test does not
	// also depend on RotateSavedCodesHandler working.
	for i := range 10 {
		seedSavedCode(t, db, founderID, founderID+"-code-"+string(rune('a'+i)))
	}
	if got := liveSavedCodesFor(t, db, founderID); got != 10 {
		t.Fatalf("precondition: founder live codes = %d, want 10", got)
	}

	_, token := seedInvitationWithToken(t, db, practiceID, founderID, "coowner@example.com", "{"+ownerRole+"}", employeeType, time.Now().Add(time.Hour))
	acceptSrv := newAcceptServer(t, db, "co-owner-uid", "coowner@example.com")
	defer acceptSrv.Close()
	acceptResp := postAccept(t, acceptSrv, staffauth.AcceptInviteRequest{WorkState: "NY", InviteToken: token, Name: "Co Owner"})
	defer acceptResp.Body.Close()
	if acceptResp.StatusCode != http.StatusOK {
		t.Fatalf("accept status = %d, want 200", acceptResp.StatusCode)
	}
	var accepted staffauth.AcceptInviteResponse
	if err := json.NewDecoder(acceptResp.Body).Decode(&accepted); err != nil {
		t.Fatalf("decode accept response: %v", err)
	}

	if got := liveSavedCodesFor(t, db, founderID); got != 0 {
		t.Fatalf("founder live codes after second owner joined = %d, want 0 (revoked)", got)
	}
	if got := liveSavedCodesFor(t, db, accepted.StaffID); got != 0 {
		t.Fatalf("new co-owner live codes = %d, want 0 (never sole)", got)
	}
}

// TestRemoveMembership_RestoresSoleOwnerSavedCodes covers the reverse:
// two Owners down to one mints a fresh set for whoever is left.
func TestRemoveMembership_RestoresSoleOwnerSavedCodes(t *testing.T) {
	db := testdb.New(t)
	ownerAID, practiceID := seedOwnerMembership(t, db, "owner-a-restore")
	ownerBID := seedStaff(t, db, "owner-b-restore")
	seedMembership(t, db, practiceID, ownerBID)
	membershipSrv, session := newMembershipServer(t, db, "owner-a-restore")
	defer membershipSrv.Close()

	// Promote B to owner too -- reconcileOwnersAtPractice fires on this
	// edit and should revoke A's codes (if any) and mint none for B.
	promote := patchMembership(t, membershipSrv, session, practiceID, ownerBID,
		staffauth.UpdateMembershipRequest{Roles: []string{ownerRole}, EmploymentType: employeeType})
	_ = promote.Body.Close()
	if promote.StatusCode != http.StatusOK {
		t.Fatalf("promote status = %d, want 200", promote.StatusCode)
	}

	// Now remove B -- A becomes sole Owner again.
	remove := deleteMembership(t, membershipSrv, session, practiceID, ownerBID)
	defer remove.Body.Close()
	if remove.StatusCode != http.StatusNoContent {
		t.Fatalf("remove status = %d, want 204", remove.StatusCode)
	}

	if got := liveSavedCodesFor(t, db, ownerAID); got != 10 {
		t.Fatalf("owner A live codes after becoming sole again = %d, want 10", got)
	}
}

func TestRotateSavedCodesHandler_MissingSession(t *testing.T) {
	db := testdb.New(t)
	srv, _ := newRotateSavedCodesServer(t, db, "rotate-no-session")
	defer srv.Close()

	resp := postRotate(t, srv, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

// TestRotateSavedCodesHandler_UnknownStaff covers a live session whose
// identity resolves to no staff row -- the shape a person who abandoned
// an invitation halfway arrives in (mirrors TestUpdateWorkState_UnknownStaff).
func TestRotateSavedCodesHandler_UnknownStaff(t *testing.T) {
	db := testdb.New(t)
	srv, session := newRotateSavedCodesServer(t, db, "rotate-session-without-a-staff-row")
	defer srv.Close()

	resp := postRotate(t, srv, session)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestRotateSavedCodesHandler_NonSoleOwnerForbidden(t *testing.T) {
	db := testdb.New(t)
	const uid = "doula-rotating"
	seedStaffWithMembership(t, db, uid) // '{doula}', not owner
	srv, session := newRotateSavedCodesServer(t, db, uid)
	defer srv.Close()

	resp := postRotate(t, srv, session)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

// TestRotateSavedCodesHandler_ReturnsTenLiveCodesMatchingHashes proves
// the one place a saved code's plaintext is ever shown, and that what is
// shown is exactly what got hashed at rest.
func TestRotateSavedCodesHandler_ReturnsTenLiveCodesMatchingHashes(t *testing.T) {
	db := testdb.New(t)
	const uid = "sole-owner-rotating"
	staffID, _ := seedOwnerMembership(t, db, uid)
	srv, session := newRotateSavedCodesServer(t, db, uid)
	defer srv.Close()

	resp := postRotate(t, srv, session)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body staffauth.RotateSavedCodesResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Codes) != 10 {
		t.Fatalf("codes = %d, want 10", len(body.Codes))
	}
	seen := map[string]bool{}
	for _, code := range body.Codes {
		if seen[code] {
			t.Fatalf("duplicate code in returned set: %q", code)
		}
		seen[code] = true
		var count int
		if err := db.Admin.QueryRowContext(t.Context(),
			`SELECT count(*) FROM staff_mfa_recovery_codes WHERE staff_id = $1 AND code_hash = $2 AND used_at IS NULL AND revoked_at IS NULL`,
			staffID, digestHex(code),
		).Scan(&count); err != nil {
			t.Fatalf("query code_hash: %v", err)
		}
		if count != 1 {
			t.Fatalf("returned code %q has no matching live row (count=%d)", code, count)
		}
	}
}

// TestRotateSavedCodesHandler_RevokesPriorBatch proves rotating again
// invalidates the previous set, not just tops it back up to ten.
func TestRotateSavedCodesHandler_RevokesPriorBatch(t *testing.T) {
	db := testdb.New(t)
	const uid = "sole-owner-rerotating"
	staffID, _ := seedOwnerMembership(t, db, uid)
	srv, session := newRotateSavedCodesServer(t, db, uid)
	defer srv.Close()

	first := postRotate(t, srv, session)
	var firstBatch staffauth.RotateSavedCodesResponse
	if err := json.NewDecoder(first.Body).Decode(&firstBatch); err != nil {
		t.Fatalf("decode first response: %v", err)
	}
	_ = first.Body.Close()

	second := postRotate(t, srv, session)
	_ = second.Body.Close()

	if got := liveSavedCodesFor(t, db, staffID); got != 10 {
		t.Fatalf("live codes after re-rotate = %d, want 10 (not 20)", got)
	}

	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(uid, uid+"@example.com", true)
	spendSrv := newSpendServer(accounts, db)
	defer spendSrv.Close()
	spend := postJSONTo(t, spendSrv, "/staff/mfa-recovery/spend",
		`{"email":"`+uid+`@example.com","code":"`+firstBatch.Codes[0]+`"}`)
	defer spend.Body.Close()
	if spend.StatusCode != http.StatusBadRequest {
		t.Fatalf("spending a code from the revoked first batch: status = %d, want 400", spend.StatusCode)
	}
}
