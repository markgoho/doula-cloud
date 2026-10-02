package engagementrequest_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/oncall"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// founderRoles is what staffauth's signup gives each founder: Owner,
// Admin and Doula, as an employee. A solo Owner is that person alone.
var founderRoles = []string{ownerRole, adminRole, doulaRole}

// newServerWithOnCall mounts this package's surface and oncall's beside
// it, so a test can start an Engagement and then read its On-call window
// the way the Engagement page does.
func newServerWithOnCall(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	enq := &tasknudge.FakeEnqueuer{}
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	engagementrequest.Mount(g, ir, db.App, enq)
	oncall.Mount(g, ir, enq)
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

// TestRequestHandler_ASoloOwnerIsOnHerOwnEngagementInOneAct is the case
// #1515 was decided for. A solo Owner starts work and names herself: the
// ask and its approval are one act (ADR-0017), and the same act attaches
// her, granted, with herself recorded as the person who attached. Nobody
// offers herself work.
func TestRequestHandler_ASoloOwnerIsOnHerOwnEngagementInOneAct(t *testing.T) {
	db := testdb.New(t)
	practiceID, ownerID := testdb.SeedStaffAtNewPractice(t, db, "solo-owner", founderRoles, employeeType)
	seedCredits(t, db, practiceID)
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Test Client", "client.com")
	enq := &tasknudge.FakeEnqueuer{}

	srv, session := newServer(t, db, "solo-owner", enq)
	defer srv.Close()

	var out engagementrequest.RequestResponse
	decode(t, do(t, requestsURL(srv.URL, practiceID, clientID), session,
		engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate, DoulaStaffID: &ownerID}),
		http.StatusCreated, &out)
	if out.State != testStateApproved || out.EngagementID == "" {
		t.Fatalf("response = %+v, want approved with an engagementId", out)
	}

	got := attachmentsOn(t, db, out.EngagementID)
	if len(got) != 1 || got[0].staffID != ownerID || got[0].origin != grantedOrigin || got[0].attachedBy != ownerID || !got[0].open {
		t.Fatalf("attachments = %+v, want one open granted one for the Owner, attached by herself", got)
	}
	if got[0].feeAmountCents != nil || got[0].feeTerms != nil {
		t.Fatalf("attachment fee = %v / %v, want none", got[0].feeAmountCents, got[0].feeTerms)
	}
	actors, doulas := doulaAttachedRows(t, db, out.EngagementID)
	if len(actors) != 1 || actors[0] != ownerID || doulas[0] != ownerID {
		t.Fatalf("doula_attached rows = actors %v doulas %v, want one where she attached herself", actors, doulas)
	}
}

// TestRequestHandler_ASoloOwnerWhoIsAContractorIsOnHerOwnEngagement is
// #1625. The only Owner of a Practice is a contractor Doula, so nobody
// can send her an Offer and she cannot send one to herself. She starts
// work and names herself, and the one act attaches her: granted, with no
// fee, attached by herself, and one doula_attached entry that names her
// as the actor.
func TestRequestHandler_ASoloOwnerWhoIsAContractorIsOnHerOwnEngagement(t *testing.T) {
	db := testdb.New(t)
	practiceID, ownerID := testdb.SeedStaffAtNewPractice(t, db, "solo-owner", founderRoles, contractorType)
	seedCredits(t, db, practiceID)
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Test Client", "client.com")

	srv, session := newServer(t, db, "solo-owner", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	var out engagementrequest.RequestResponse
	decode(t, do(t, requestsURL(srv.URL, practiceID, clientID), session,
		engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate, DoulaStaffID: &ownerID}),
		http.StatusCreated, &out)
	if out.State != testStateApproved || out.EngagementID == "" {
		t.Fatalf("response = %+v, want approved with an engagementId", out)
	}

	got := attachmentsOn(t, db, out.EngagementID)
	if len(got) != 1 || got[0].staffID != ownerID || got[0].origin != grantedOrigin || got[0].attachedBy != ownerID || !got[0].open {
		t.Fatalf("attachments = %+v, want one open granted one for the Owner, attached by herself", got)
	}
	if got[0].feeAmountCents != nil || got[0].feeTerms != nil {
		t.Fatalf("attachment fee = %v / %v, want none", got[0].feeAmountCents, got[0].feeTerms)
	}
	actors, doulas := doulaAttachedRows(t, db, out.EngagementID)
	if len(actors) != 1 || actors[0] != ownerID || doulas[0] != ownerID {
		t.Fatalf("doula_attached rows = actors %v doulas %v, want one where she attached herself", actors, doulas)
	}
}

// TestRequestHandler_ASoloOwnerWithNoDoulaYetIsNotAttached proves an
// agency Owner's answer on the same collapsed path: the Engagement
// exists, First Value is reached (ADR-0048), and nobody is on it.
func TestRequestHandler_ASoloOwnerWithNoDoulaYetIsNotAttached(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "agency-owner", founderRoles, employeeType)
	seedCredits(t, db, practiceID)
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Test Client", "client.com")

	srv, session := newServer(t, db, "agency-owner", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	var out engagementrequest.RequestResponse
	decode(t, do(t, requestsURL(srv.URL, practiceID, clientID), session,
		engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate}),
		http.StatusCreated, &out)
	if out.State != testStateApproved || out.EngagementID == "" {
		t.Fatalf("response = %+v, want approved with an engagementId", out)
	}
	if got := attachmentsOn(t, db, out.EngagementID); len(got) != 0 {
		t.Fatalf("attachments = %+v, want none", got)
	}
}

// TestSoloOwnersFirstBirthNeedsNoFurtherActToBeOnCall reads the On-call
// window of a solo Owner's first birth Engagement, started in one act
// with a due date and with herself named.
//
// What that one act settles is the half of the window that was missing
// before #1596: somebody holds a granted Attachment. CONTEXT.md gives a
// window to an `active` Engagement only, and an Engagement starts at
// `intake`, so on the day it starts the read answers "not active" and
// never "nobody attached". The move to `active` is the Engagement's own
// care phase (ADR-0015), which a scheduled Visit makes by itself, and it
// is no act of attaching. Once it is made, the window is there and she
// is the Doula on call, with no Offer to herself and no Visit naming
// herself in between.
func TestSoloOwnersFirstBirthNeedsNoFurtherActToBeOnCall(t *testing.T) {
	db := testdb.New(t)
	practiceID, ownerID := testdb.SeedStaffAtNewPractice(t, db, "solo-owner", founderRoles, employeeType)
	seedCredits(t, db, practiceID)
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Test Client", "client.com")

	srv, session := newServerWithOnCall(t, db, "solo-owner")
	defer srv.Close()

	var started engagementrequest.RequestResponse
	decode(t, do(t, requestsURL(srv.URL, practiceID, clientID), session,
		engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate, DoulaStaffID: &ownerID}),
		http.StatusCreated, &started)
	onCallURL := srv.URL + "/api/practices/" + practiceID + "/engagements/" + started.EngagementID + "/on-call"

	var atIntake oncall.EngagementOnCall
	decode(t, get(t, onCallURL, session), http.StatusOK, &atIntake)
	if atIntake.NoWindowReason != oncall.NoWindowNotActive {
		t.Fatalf("no-window reason at intake = %q, want %q: the Attachment is not what is missing",
			atIntake.NoWindowReason, oncall.NoWindowNotActive)
	}

	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagements SET status = 'active' WHERE id = $1`, started.EngagementID,
	); err != nil {
		t.Fatalf("move engagement to active: %v", err)
	}

	var active oncall.EngagementOnCall
	decode(t, get(t, onCallURL, session), http.StatusOK, &active)
	if active.Window == nil {
		t.Fatalf("window = none (%q), want one for an active birth with a due date and a granted Attachment", active.NoWindowReason)
	}
	if len(active.Doulas) != 1 || active.Doulas[0].StaffID != ownerID {
		t.Fatalf("doulas on call = %+v, want the Owner alone", active.Doulas)
	}
}
