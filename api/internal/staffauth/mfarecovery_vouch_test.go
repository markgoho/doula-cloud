package staffauth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

func newVouchServer(t *testing.T, db *testdb.DB, verifier authn.Verifier, enq tasknudge.Enqueuer) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("POST /practices/{practiceId}/staff/{staffId}/mfa-recovery/vouch",
		staffauth.Middleware(db.App)(staffauth.VouchHandler(verifier, enq)))
	return httptest.NewServer(mux)
}

func vouchRequest(t *testing.T, srv *httptest.Server, session, practiceID, staffID string, bearer string, confirmed bool) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		srv.URL+"/practices/"+practiceID+"/staff/"+staffID+"/mfa-recovery/vouch", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if confirmed {
		req.Header.Set("X-Confirmed", "true")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func TestVouchHandler_MissingBearerUnauthorized(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-vouch-no-bearer"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)
	const targetUID = "target-vouch-no-bearer"
	targetID := seedStaff(t, db, targetUID)
	seedMembership(t, db, practiceID, targetID)

	session := authntest.SeedSession(t, db.App, ownerUID)
	srv := newVouchServer(t, db, authntest.Verifier{UID: ownerUID, AuthTime: time.Now()}, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := vouchRequest(t, srv, session, practiceID, targetID, "", true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestVouchHandler_InvalidBearerTokenUnauthorized(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-vouch-invalid-bearer"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)
	const targetUID = "target-vouch-invalid-bearer"
	targetID := seedStaff(t, db, targetUID)
	seedMembership(t, db, practiceID, targetID)

	session := authntest.SeedSession(t, db.App, ownerUID)
	srv := newVouchServer(t, db, authntest.Verifier{Err: errBadToken}, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := vouchRequest(t, srv, session, practiceID, targetID, "rejected-token", true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestVouchHandler_StaleReauthUnauthorized(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-vouch-stale"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)
	const targetUID = "target-vouch-stale"
	targetID := seedStaff(t, db, targetUID)
	seedMembership(t, db, practiceID, targetID)

	session := authntest.SeedSession(t, db.App, ownerUID)
	srv := newVouchServer(t, db, authntest.Verifier{UID: ownerUID, AuthTime: time.Now().Add(-time.Hour)}, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := vouchRequest(t, srv, session, practiceID, targetID, "fresh-id-token", true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestVouchHandler_ReauthUIDMismatchUnauthorized(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-vouch-mismatch"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)
	const targetUID = "target-vouch-mismatch"
	targetID := seedStaff(t, db, targetUID)
	seedMembership(t, db, practiceID, targetID)

	session := authntest.SeedSession(t, db.App, ownerUID)
	// The Bearer token verifies fine, but names a different identity --
	// stolen or misdirected, either way not this Owner's own fresh
	// sign-in.
	srv := newVouchServer(t, db, authntest.Verifier{UID: "someone-else", AuthTime: time.Now()}, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := vouchRequest(t, srv, session, practiceID, targetID, "someone-elses-token", true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestVouchHandler_MissingConfirmationBadRequest(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-vouch-unconfirmed"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)
	const targetUID = "target-vouch-unconfirmed"
	targetID := seedStaff(t, db, targetUID)
	seedMembership(t, db, practiceID, targetID)

	session := authntest.SeedSession(t, db.App, ownerUID)
	srv := newVouchServer(t, db, authntest.Verifier{UID: ownerUID, AuthTime: time.Now()}, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := vouchRequest(t, srv, session, practiceID, targetID, "fresh-id-token", false)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestVouchHandler_NonOwnerForbidden(t *testing.T) {
	db := testdb.New(t)
	const doulaUID = "doula-vouching"
	staffID, practiceID := seedStaffWithMembership(t, db, doulaUID) // '{doula}', not owner

	session := authntest.SeedSession(t, db.App, doulaUID)
	srv := newVouchServer(t, db, authntest.Verifier{UID: doulaUID, AuthTime: time.Now()}, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := vouchRequest(t, srv, session, practiceID, staffID, "fresh-id-token", true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestVouchHandler_MalformedStaffID(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-vouch-malformed-target"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)

	session := authntest.SeedSession(t, db.App, ownerUID)
	srv := newVouchServer(t, db, authntest.Verifier{UID: ownerUID, AuthTime: time.Now()}, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := vouchRequest(t, srv, session, practiceID, "not-a-uuid", "fresh-id-token", true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestVouchHandler_NoSuchMembership(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-vouch-no-target"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)

	session := authntest.SeedSession(t, db.App, ownerUID)
	srv := newVouchServer(t, db, authntest.Verifier{UID: ownerUID, AuthTime: time.Now()}, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := vouchRequest(t, srv, session, practiceID, emptyUUID, "fresh-id-token", true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// TestVouchHandler_Success is #605's whole mechanism: a fresh 24-hour
// issued code, minted for the target's identity, recorded against the
// vouching Owner, and queued to the Owner's own address -- never the
// target's.
func TestVouchHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-vouch-success"
	ownerID, practiceID := seedOwnerMembership(t, db, ownerUID)
	const targetUID = "target-vouch-success"
	targetID := seedStaff(t, db, targetUID)
	seedMembership(t, db, practiceID, targetID)

	session := authntest.SeedSession(t, db.App, ownerUID)
	srv := newVouchServer(t, db, authntest.Verifier{UID: ownerUID, AuthTime: time.Now()}, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := vouchRequest(t, srv, session, practiceID, targetID, "fresh-id-token", true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}

	var tokenHash, purpose string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT token_hash, purpose::text FROM auth_tokens WHERE identity_uid = $1 AND used_at IS NULL`, targetUID,
	).Scan(&tokenHash, &purpose); err != nil {
		t.Fatalf("query auth_tokens: %v", err)
	}
	if purpose != "staff_mfa_recovery" {
		t.Fatalf("purpose = %q, want staff_mfa_recovery", purpose)
	}

	var voucheeStaffID, voucherStaffID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT staff_id, owner_staff_id FROM staff_mfa_recovery_vouches WHERE token_hash = $1`, tokenHash,
	).Scan(&voucheeStaffID, &voucherStaffID); err != nil {
		t.Fatalf("query staff_mfa_recovery_vouches: %v", err)
	}
	if voucheeStaffID != targetID || voucherStaffID != ownerID {
		t.Fatalf("vouches row = (%q, %q), want (%q, %q)", voucheeStaffID, voucherStaffID, targetID, ownerID)
	}

	var recipientUID, subjectID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT recipient_identity_uid, subject_staff_id FROM staff_mfa_recovery_outbox WHERE subject_staff_id = $1`, targetID,
	).Scan(&recipientUID, &subjectID); err != nil {
		t.Fatalf("query staff_mfa_recovery_outbox: %v", err)
	}
	if recipientUID != ownerUID {
		t.Fatalf("recipient_identity_uid = %q, want the owner's own identity %q, never the target's", recipientUID, ownerUID)
	}
}
