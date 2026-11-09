package staffauth_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// practiceSessionAs reads GET /api/practices/{practiceId}/session with
// credential and requires the 200.
func (e *env) practiceSessionAs(credential reqtest.Option) staffauth.PracticeSessionResponse {
	e.T.Helper()
	return reqtest.Expect[staffauth.PracticeSessionResponse](e.T, http.MethodGet, e.PracticeURL("/session"), http.StatusOK, credential)
}

// practiceSession reads it as the Staff member at the screen.
func (e *env) practiceSession() staffauth.PracticeSessionResponse {
	e.T.Helper()
	return e.practiceSessionAs(e.SignedIn)
}

// TestPracticeSessionHandler_ReportsPracticeAndRoles proves the moved
// handler (#836, out of routes_practice.go and into this package)
// answers with the caller's Practice name, roles, and employment-type
// axis -- exercised here through staffauth.Mount, the production
// interface, rather than through api's own route table.
func TestPracticeSessionHandler_ReportsPracticeAndRoles(t *testing.T) {
	e := newEnv(t,
		viaScene(fixture.InPractice(fixture.PracticeName("Practice Session Test Practice"))),
		signedInAs(fixture.Owner(), fixture.Doula(), fixture.Contractor()))

	out := e.practiceSession()

	if out.PracticeID != e.Practice.ID {
		t.Fatalf("practiceId = %q, want %q", out.PracticeID, e.Practice.ID)
	}
	if out.PracticeName != "Practice Session Test Practice" {
		t.Fatalf("practiceName = %q, want %q", out.PracticeName, "Practice Session Test Practice")
	}
	if len(out.Roles) != 2 {
		t.Fatalf("roles = %v, want 2 entries", out.Roles)
	}
	if !out.IsContractor {
		t.Fatal("isContractor = false, want true")
	}
	// #909: the caller's own Staff id rides along, so a route under this
	// Practice knows which roster entry is her without a second call.
	if out.StaffID != e.Staff.ID {
		t.Fatalf("staffId = %q, want %q", out.StaffID, e.Staff.ID)
	}

	lastPracticeID := testdb.Scalar[string](t, e.DB.Admin, `SELECT last_practice_id FROM staff WHERE identity_uid = $1`, e.Staff.UID)
	if lastPracticeID != e.Practice.ID {
		t.Fatalf("last_practice_id = %q, want %q", lastPracticeID, e.Practice.ID)
	}
}

// TestPracticeSessionHandler_HasClients proves HasClients (#2183) reads
// false for a Practice with no Client and true once it has one, whoever
// asks: a contractor Doula with no attachment reads the Practice's
// answer, not her own reach, so the bar still offers her Offers.
func TestPracticeSessionHandler_HasClients(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner(), fixture.Doula()))
	contractor := e.As(e.Colleague(fixture.Doula(), fixture.Contractor()))

	if e.practiceSession().HasClients {
		t.Fatal("hasClients = true, want false before any Client exists")
	}

	testdb.SeedNamedClient(t, e.DB, e.Practice.ID, "First Client", "")

	if !e.practiceSession().HasClients {
		t.Fatal("hasClients = false, want true once the Practice has a Client")
	}
	if !e.practiceSessionAs(contractor).HasClients {
		t.Fatal("hasClients = false for a contractor with no attachment, want the Practice's answer")
	}
}

// TestPracticeSessionHandler_HasOtherDoulas proves HasOtherDoulas (#2675)
// is false for the Practice's only Doula, and true for an Admin beside her
// and for her once a second Doula joins.
func TestPracticeSessionHandler_HasOtherDoulas(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner(), fixture.Doula()))
	admin := e.As(e.Colleague(fixture.Roles(adminRole)))

	if e.practiceSession().HasOtherDoulas {
		t.Fatal("hasOtherDoulas = true for the only Doula, want false")
	}
	if !e.practiceSessionAs(admin).HasOtherDoulas {
		t.Fatal("hasOtherDoulas = false for an Admin beside a Doula, want true")
	}

	e.Colleague(fixture.Doula(), fixture.Contractor())

	if !e.practiceSession().HasOtherDoulas {
		t.Fatal("hasOtherDoulas = false once a second Doula joins, want true")
	}
}

// TestPracticeSessionHandler_ClientsCanPay proves ClientsCanPay (#2293)
// is cardpayments.ClientsCanPay's fact: false until the Practice's card
// payments capability is active, then true.
func TestPracticeSessionHandler_ClientsCanPay(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))

	if e.practiceSession().ClientsCanPay {
		t.Fatal("clientsCanPay = true before card payments are active, want false")
	}

	testdb.SeedClientsCanPay(t, e.DB, e.Practice.ID)

	if !e.practiceSession().ClientsCanPay {
		t.Fatal("clientsCanPay = false once card payments are active, want true")
	}
}

// TestPracticeSessionHandler_BillingMode proves BillingMode (#2200) is
// practices.billing_mode as stored: empty until the Practice has chosen
// one, then the word it chose.
func TestPracticeSessionHandler_BillingMode(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))

	if got := e.practiceSession().BillingMode; got != "" {
		t.Fatalf("billingMode = %q before one is chosen, want empty", got)
	}

	testdb.SeedBillingMode(t, e.DB, e.Practice.ID, "by_hand")

	if got := e.practiceSession().BillingMode; got != "by_hand" {
		t.Fatalf("billingMode = %q, want by_hand", got)
	}
}

// TestPracticeSessionHandler_EmployeeNotContractor proves IsContractor
// reads false for an employee Membership, not just true for a
// contractor.
func TestPracticeSessionHandler_EmployeeNotContractor(t *testing.T) {
	e := newEnv(t)

	if e.practiceSession().IsContractor {
		t.Fatal("isContractor = true, want false for an employee")
	}
}
