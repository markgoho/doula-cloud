package staffauth_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

const vouchOwnerUID = "owner-vouching"

// newVouchEnv is an Owner at the screen whose Bearer token verifies as
// her, signed in a moment ago -- the fresh re-authentication a vouch asks
// for. A test that is about the token passes its own Verifier.
func newVouchEnv(t *testing.T, staff []fixture.StaffOption, verifier authntest.Verifier) *env {
	t.Helper()
	return newEnv(t, signedInAs(append(staff, fixture.StaffUID(vouchOwnerUID))...), withVerifier(verifier))
}

func freshOwner() authntest.Verifier {
	return authntest.Verifier{UID: vouchOwnerUID, AuthTime: time.Now()}
}

// vouch asks for staffID's recovery vouched for by the Staff member at the
// screen and requires want. bearer is the fresh ID token, confirm the
// X-Confirmed backstop.
func (e *env) vouch(staffID, bearer string, confirm bool, want int) {
	e.T.Helper()
	url := e.PracticeURL("/staff/" + staffID + "/mfa-recovery/vouch")
	reqtest.ExpectStatus(e.T, http.MethodPost, url, want, e.SignedIn,
		reqtest.HeaderIf(bearer != "", "Authorization", "Bearer "+bearer), reqtest.HeaderIf(confirm, "X-Confirmed", "true"))
}

// TestVouchHandler_Refuses covers each way a vouch is turned away: no
// Bearer token, one that does not verify, one whose sign-in is stale, one
// that verifies but names a different identity (stolen or misdirected,
// either way not this Owner's own fresh sign-in), no confirmation, a
// non-Owner, a staff id that is not a uuid, and a Membership nobody holds.
func TestVouchHandler_Refuses(t *testing.T) {
	cases := []struct {
		name     string
		verifier authntest.Verifier
		doula    bool
		bearer   string
		confirm  bool
		// target returns the staff id the request names.
		target func(e *env, colleague fixture.StaffRecord) string
		want   int
	}{
		{name: "no Bearer token", verifier: freshOwner(), confirm: true, want: http.StatusUnauthorized},
		{name: "a Bearer token that does not verify", verifier: authntest.Verifier{Err: errBadToken}, bearer: "rejected-token", confirm: true, want: http.StatusUnauthorized},
		{name: "a stale sign-in", verifier: authntest.Verifier{UID: vouchOwnerUID, AuthTime: time.Now().Add(-time.Hour)}, bearer: "fresh-id-token", confirm: true, want: http.StatusUnauthorized},
		{name: "a token for someone else", verifier: authntest.Verifier{UID: "someone-else", AuthTime: time.Now()}, bearer: "someone-elses-token", confirm: true, want: http.StatusUnauthorized},
		{name: "no confirmation", verifier: freshOwner(), bearer: "fresh-id-token", want: http.StatusBadRequest},
		{name: "a non-Owner", verifier: freshOwner(), doula: true, bearer: "fresh-id-token", confirm: true, want: http.StatusForbidden,
			target: func(e *env, _ fixture.StaffRecord) string { return e.Staff.ID }},
		{name: "a staff id that is not a uuid", verifier: freshOwner(), bearer: "fresh-id-token", confirm: true, want: http.StatusBadRequest,
			target: func(*env, fixture.StaffRecord) string { return "not-a-uuid" }},
		{name: "a Membership nobody holds", verifier: freshOwner(), bearer: "fresh-id-token", confirm: true, want: http.StatusNotFound,
			target: func(*env, fixture.StaffRecord) string { return emptyUUID }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			role := fixture.Owner()
			if tc.doula {
				role = fixture.Doula()
			}
			e := newVouchEnv(t, []fixture.StaffOption{role}, tc.verifier)
			colleague := e.Colleague()
			staffID := colleague.ID
			if tc.target != nil {
				staffID = tc.target(e, colleague)
			}

			e.vouch(staffID, tc.bearer, tc.confirm, tc.want)
		})
	}
}

// TestVouchHandler_Success is #605's whole mechanism: a fresh 24-hour
// issued code, minted for the target's identity, recorded against the
// vouching Owner, and queued to the Owner's own address -- never the
// target's.
func TestVouchHandler_Success(t *testing.T) {
	e := newVouchEnv(t, []fixture.StaffOption{fixture.Owner()}, freshOwner())
	target := e.Colleague()

	e.vouch(target.ID, "fresh-id-token", true, http.StatusNoContent)

	var tokenHash, purpose string
	if err := e.DB.Admin.QueryRowContext(t.Context(),
		`SELECT token_hash, purpose::text FROM auth_tokens WHERE identity_uid = $1 AND used_at IS NULL`, target.UID,
	).Scan(&tokenHash, &purpose); err != nil {
		t.Fatalf("query auth_tokens: %v", err)
	}
	if purpose != "staff_mfa_recovery" {
		t.Fatalf("purpose = %q, want staff_mfa_recovery", purpose)
	}

	var voucheeStaffID, voucherStaffID string
	if err := e.DB.Admin.QueryRowContext(t.Context(),
		`SELECT staff_id, owner_staff_id FROM staff_mfa_recovery_vouches WHERE token_hash = $1`, tokenHash,
	).Scan(&voucheeStaffID, &voucherStaffID); err != nil {
		t.Fatalf("query staff_mfa_recovery_vouches: %v", err)
	}
	if voucheeStaffID != target.ID || voucherStaffID != e.Staff.ID {
		t.Fatalf("vouches row = (%q, %q), want (%q, %q)", voucheeStaffID, voucherStaffID, target.ID, e.Staff.ID)
	}

	recipientUID := testdb.Scalar[string](t, e.DB.Admin,
		`SELECT recipient_identity_uid FROM staff_mfa_recovery_outbox WHERE subject_staff_id = $1`, target.ID)
	if recipientUID != vouchOwnerUID {
		t.Fatalf("recipient_identity_uid = %q, want the owner's own identity %q, never the target's", recipientUID, vouchOwnerUID)
	}
}
