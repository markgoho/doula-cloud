package staffauth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// countRefusals counts every second-factor refusal recorded at
// practiceID, whoever tried.
func countRefusals(t *testing.T, db *testdb.DB, practiceID string) int {
	t.Helper()
	var n int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM activity WHERE practice_id = $1 AND action = $2`,
		practiceID, staffauth.ActionSecondFactorRefused,
	).Scan(&n); err != nil {
		t.Fatalf("count refusals: %v", err)
	}
	return n
}

// An Owner with no second factor who throws the switch is refused by the
// act's seam, the switch stays off, and the refusal is recorded once.
func TestSecondFactorAct_RefusesAndRecords(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-no-factor-act"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)

	srv, _ := newMFARequiredServer(t, db, authntest.NewFakeAccountManager())
	defer srv.Close()

	session := authntest.SeedSessionWithSecondFactor(t, db.App, ownerUID, false)
	resp := putMFARequired(t, srv, session, practiceID, true, true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	if got := apierrtest.Decode(t, resp).Code; got != apierr.CodeSecondFactorRequired {
		t.Fatalf("code = %q, want %q", got, apierr.CodeSecondFactorRequired)
	}
	if n := countRefusals(t, db, practiceID); n != 1 {
		t.Fatalf("refusals recorded = %d, want 1", n)
	}
	var required bool
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT require_mfa_for_all_staff FROM practices WHERE id = $1`, practiceID).Scan(&required); err != nil {
		t.Fatalf("read switch: %v", err)
	}
	if required {
		t.Fatal("switch is on after a refused request")
	}
}

// A Doula with no second factor who asks for an act meets the role
// refusal she met before #1532, and nothing is recorded: the seam runs
// after the role check, so it only ever refuses a person who may do the
// act.
func TestSecondFactorAct_RoleRefusalComesFirst(t *testing.T) {
	db := testdb.New(t)
	const doulaUID = "doula-no-factor-act"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, doulaUID, []string{doulaRole}, employeeType)

	srv, _ := newMFARequiredServer(t, db, authntest.NewFakeAccountManager())
	defer srv.Close()

	session := authntest.SeedSessionWithSecondFactor(t, db.App, doulaUID, false)
	resp := putMFARequired(t, srv, session, practiceID, true, true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	if got := apierrtest.Decode(t, resp).Code; got != apierr.CodeForbidden {
		t.Fatalf("code = %q, want %q", got, apierr.CodeForbidden)
	}
	if n := countRefusals(t, db, practiceID); n != 0 {
		t.Fatalf("refusals recorded = %d, want 0", n)
	}
}

// At a Practice whose switch is on, the boundary still refuses everyone
// with no second factor before any act runs, an Owner on an act's route
// included -- the Practice's own posture answers first.
func TestSecondFactorAct_SwitchOnBoundaryAnswersFirst(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-no-factor-switch-on"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET require_mfa_for_all_staff = true WHERE id = $1`, practiceID); err != nil {
		t.Fatalf("throw switch: %v", err)
	}

	srv, _ := newMFARequiredServer(t, db, authntest.NewFakeAccountManager())
	defer srv.Close()

	session := authntest.SeedSessionWithSecondFactor(t, db.App, ownerUID, false)
	resp := putMFARequired(t, srv, session, practiceID, false, true)
	defer resp.Body.Close()
	if got := apierrtest.Decode(t, resp).Code; got != apierr.CodeMFARequired {
		t.Fatalf("code = %q, want %q", got, apierr.CodeMFARequired)
	}
	if n := countRefusals(t, db, practiceID); n != 0 {
		t.Fatalf("refusals recorded = %d, want 0", n)
	}
}

// Off the five acts' routes the boundary is unchanged until #1531: an
// Owner with no second factor is still refused the Practice.
func TestSecondFactorAct_OtherOwnerRoutesStillRefusedAtTheBoundary(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-no-factor-impact"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)

	srv, _ := newMFARequiredServer(t, db, authntest.NewFakeAccountManager())
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/api/practices/"+practiceID+"/mfa-required/impact", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, authntest.SeedSessionWithSecondFactor(t, db.App, ownerUID, false))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if got := apierrtest.Decode(t, resp).Code; got != apierr.CodeMFARequired {
		t.Fatalf("code = %q, want %q", got, apierr.CodeMFARequired)
	}
}

// The Practice session carries the session's own second-factor fact, so
// each act's screen can say what the act needs before she tries.
func TestPracticeSession_CarriesTheSecondFactor(t *testing.T) {
	for _, secondFactor := range []bool{false, true} {
		db := testdb.New(t)
		const doulaUID = "doula-session-factor"
		practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, doulaUID, []string{doulaRole}, employeeType)

		srv, _ := newMFARequiredServer(t, db, authntest.NewFakeAccountManager())

		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/api/practices/"+practiceID+"/session", nil)
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		authntest.AddSessionCookie(req, authntest.SeedSessionWithSecondFactor(t, db.App, doulaUID, secondFactor))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		var body staffauth.PracticeSessionResponse
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		_ = resp.Body.Close()
		srv.Close()
		if body.SecondFactor != secondFactor {
			t.Fatalf("secondFactor = %v, want %v", body.SecondFactor, secondFactor)
		}
	}
}

func TestSecondFactor_FalseOffAnUnresolvedRequest(t *testing.T) {
	if staffauth.SecondFactor(context.Background()) {
		t.Fatal("SecondFactor = true on a context Middleware never resolved")
	}
}
