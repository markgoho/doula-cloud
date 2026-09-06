package staffauth_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// #745: this endpoint answers against what the calling identity already
// holds. These cover the two branches that reads: a staff row with no
// Membership, which is a Staff member her last Practice removed, and a
// staff row that already has one, which is a call that must build
// nothing.

const resumeUID = "resume-owner"

// TestSignupHandler_ResumesStaffRowWithNoMembership proves a signup
// against an identity that already has a staff row and no Membership
// completes rather than colliding: the existing staff row gets the new
// Practice, keeps its identity, and no second staff row appears.
func TestSignupHandler_ResumesStaffRowWithNoMembership(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(authntest.Verifier{UID: resumeUID, Email: jamieEmail}, db)
	defer srv.Close()
	seeded := seedStaff(t, db, resumeUID)

	resp := postSignup(t, srv, "tok", staffauth.SignupRequest{
		WorkState:    "VT",
		PracticeName: "Resumed Practice",
		StaffName:    jamieOwnerName,
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var out staffauth.SignupResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.StaffID != seeded {
		t.Fatalf("staffId = %q, want the existing row %q", out.StaffID, seeded)
	}

	var staffCount int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM staff WHERE identity_uid = $1`, resumeUID,
	).Scan(&staffCount); err != nil {
		t.Fatalf("count staff: %v", err)
	}
	if staffCount != 1 {
		t.Fatalf("staff rows = %d, want exactly 1", staffCount)
	}

	var roleCount int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT array_length(roles, 1) FROM practice_memberships WHERE practice_id = $1 AND staff_id = $2`,
		out.PracticeID, seeded,
	).Scan(&roleCount); err != nil {
		t.Fatalf("query membership roles: %v", err)
	}
	if roleCount != 3 {
		t.Fatalf("roles = %d, want 3 (owner, admin, doula)", roleCount)
	}

	// What this form collected about her is a current answer, not a
	// repeat of what the seeded row held -- and the work state's column
	// moves with its event.
	var storedName, stored string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT name, work_state FROM staff WHERE id = $1`, seeded,
	).Scan(&storedName, &stored); err != nil {
		t.Fatalf("query staff row: %v", err)
	}
	if storedName != jamieOwnerName {
		t.Fatalf("name = %q, want %q -- the name she typed was discarded", storedName, jamieOwnerName)
	}
	if stored != "VT" {
		t.Fatalf("work_state = %q, want %q", stored, "VT")
	}
	var previous string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT coalesce(previous_work_state, '') FROM staff_work_state_events
		  WHERE staff_id = $1 ORDER BY created_at DESC LIMIT 1`, seeded,
	).Scan(&previous); err != nil {
		t.Fatalf("query work state event: %v", err)
	}
	if previous != "NY" {
		t.Fatalf("previous_work_state = %q, want %q -- a resumed signup records a correction, not a first assertion", previous, "NY")
	}
}

// TestSignupHandler_ResumeQueuesNoSecondVerificationMail proves the
// resumed half sends nothing: the staff row already existed, so its
// address has already been sent a verification link.
func TestSignupHandler_ResumeQueuesNoSecondVerificationMail(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(authntest.Verifier{UID: resumeUID, Email: jamieEmail}, db)
	defer srv.Close()
	seedStaff(t, db, resumeUID)

	resp := postSignup(t, srv, "tok", staffauth.SignupRequest{
		WorkState: "NY", PracticeName: "Quiet Practice", StaffName: jamieOwnerName,
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var mailCount int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM staff_token_mail_outbox WHERE identity_uid = $1`,
		resumeUID,
	).Scan(&mailCount); err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	if mailCount != 0 {
		t.Fatalf("outbox messages = %d, want 0 on a resumed signup", mailCount)
	}
}

// TestSignupHandler_RefusesWhenAlreadyInAPractice proves the third
// branch: an identity that already holds a Membership is told so in
// words a reader can act on, and the Practice name she typed creates
// nothing.
func TestSignupHandler_RefusesWhenAlreadyInAPractice(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(authntest.Verifier{UID: resumeUID, Email: jamieEmail}, db)
	defer srv.Close()

	first := postSignup(t, srv, "tok", staffauth.SignupRequest{
		WorkState: "NY", PracticeName: "First Practice", StaffName: jamieName,
	})
	_ = first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first signup status = %d, want %d", first.StatusCode, http.StatusCreated)
	}

	const secondName = "Second Practice"
	second := postSignup(t, srv, "tok", staffauth.SignupRequest{
		WorkState: "NY", PracticeName: secondName, StaffName: jamieName,
	})
	defer second.Body.Close()
	if second.StatusCode != http.StatusConflict {
		t.Fatalf("second signup status = %d, want %d", second.StatusCode, http.StatusConflict)
	}
	var out apierr.APIError
	if err := json.NewDecoder(second.Body).Decode(&out); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if out.Message != staffauth.MsgAlreadyBelongsToPractice {
		t.Fatalf("message = %q, want %q", out.Message, staffauth.MsgAlreadyBelongsToPractice)
	}

	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM practices WHERE name = $1`, secondName,
	).Scan(&count); err != nil {
		t.Fatalf("count practices: %v", err)
	}
	if count != 0 {
		t.Fatalf("refused signup left %d Practice rows behind, want 0", count)
	}
}
