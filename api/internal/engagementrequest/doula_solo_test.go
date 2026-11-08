package engagementrequest_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/oncall"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// newServerWithOnCall mounts this package's surface and oncall's beside
// it, so a test can start an Engagement and then read its On-call window
// the way the Engagement page does.
func newServerWithOnCall(t *testing.T, db *testdb.DB, who fixture.StaffRecord) (srv *httptest.Server, session string) {
	t.Helper()
	enq := &tasknudge.FakeEnqueuer{}
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	engagementrequest.Mount(g, ir, db.App, enq)
	oncall.Mount(g, ir, enq)
	return httptest.NewServer(mux), fixture.Session(t, db, who)
}

// TestRequestHandler_ASoloOwnerIsOnHerOwnEngagementInOneAct is the case
// #1515 was decided for. A solo Owner starts work and names herself: the
// ask and its approval are one act (ADR-0017), and the same act attaches
// her, granted, with herself recorded as the person who attached. Nobody
// offers herself work.
func TestRequestHandler_ASoloOwnerIsOnHerOwnEngagementInOneAct(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db, fixture.Credits(3))
	owner := fixture.Staff(t, db, p, fixture.Founder())
	client := fixture.Client(t, db, p)
	enq := &tasknudge.FakeEnqueuer{}

	srv, session := newServer(t, db, owner, enq)
	defer srv.Close()

	outResp := reqtest.Post(t, requestsURL(srv.URL, p.ID, client.ID), reqtest.Session(session), reqtest.JSON(engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate, DoulaStaffID: &owner.ID}))
	defer outResp.Body.Close()
	out := reqtest.DecodeStatus[engagementrequest.RequestResponse](t, outResp, http.StatusCreated)
	if out.State != testStateApproved || out.EngagementID == "" {
		t.Fatalf("response = %+v, want approved with an engagementId", out)
	}

	got := attachmentsOn(t, db, out.EngagementID)
	if len(got) != 1 || got[0].staffID != owner.ID || got[0].origin != grantedOrigin || got[0].attachedBy != owner.ID || !got[0].open {
		t.Fatalf("attachments = %+v, want one open granted one for the Owner, attached by herself", got)
	}
	if got[0].feeAmountCents != nil || got[0].feeTerms != nil {
		t.Fatalf("attachment fee = %v / %v, want none", got[0].feeAmountCents, got[0].feeTerms)
	}
	actors, doulas := doulaAttachedRows(t, db, out.EngagementID)
	if len(actors) != 1 || actors[0] != owner.ID || doulas[0] != owner.ID {
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
	p := fixture.Practice(t, db, fixture.Credits(3))
	owner := fixture.Staff(t, db, p, fixture.Founder(), fixture.Contractor())
	client := fixture.Client(t, db, p)

	srv, session := newServer(t, db, owner, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	outResp := reqtest.Post(t, requestsURL(srv.URL, p.ID, client.ID), reqtest.Session(session), reqtest.JSON(engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate, DoulaStaffID: &owner.ID}))
	defer outResp.Body.Close()
	out := reqtest.DecodeStatus[engagementrequest.RequestResponse](t, outResp, http.StatusCreated)
	if out.State != testStateApproved || out.EngagementID == "" {
		t.Fatalf("response = %+v, want approved with an engagementId", out)
	}

	got := attachmentsOn(t, db, out.EngagementID)
	if len(got) != 1 || got[0].staffID != owner.ID || got[0].origin != grantedOrigin || got[0].attachedBy != owner.ID || !got[0].open {
		t.Fatalf("attachments = %+v, want one open granted one for the Owner, attached by herself", got)
	}
	if got[0].feeAmountCents != nil || got[0].feeTerms != nil {
		t.Fatalf("attachment fee = %v / %v, want none", got[0].feeAmountCents, got[0].feeTerms)
	}
	actors, doulas := doulaAttachedRows(t, db, out.EngagementID)
	if len(actors) != 1 || actors[0] != owner.ID || doulas[0] != owner.ID {
		t.Fatalf("doula_attached rows = actors %v doulas %v, want one where she attached herself", actors, doulas)
	}
}

// TestRequestHandler_ASoloOwnerWithNoDoulaYetIsNotAttached proves an
// agency Owner's answer on the same collapsed path: the Engagement
// exists, First Value is reached (ADR-0048), and nobody is on it.
func TestRequestHandler_ASoloOwnerWithNoDoulaYetIsNotAttached(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db, fixture.Credits(3))
	agencyOwner := fixture.Staff(t, db, p, fixture.Founder())
	client := fixture.Client(t, db, p)

	srv, session := newServer(t, db, agencyOwner, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	outResp := reqtest.Post(t, requestsURL(srv.URL, p.ID, client.ID), reqtest.Session(session), reqtest.JSON(engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate}))
	defer outResp.Body.Close()
	out := reqtest.DecodeStatus[engagementrequest.RequestResponse](t, outResp, http.StatusCreated)
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
// The Attachment is the fact that somebody is on the birth, and GLOSSARY.md
// gives a window to a `birth` Engagement with a granted Attachment whose
// status is not `completed` (decided on #1616). An Engagement starts at
// `intake`, so on the day it starts the window is already there and she
// is the Doula on call, with no Offer to herself, no Visit naming herself
// and no move to `active` in between. The care phase (ADR-0015) says only
// whether care has started, so moving to `active` changes nothing about
// the window, and `completed` is what takes it away.
func TestSoloOwnersFirstBirthNeedsNoFurtherActToBeOnCall(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db, fixture.Credits(3))
	owner := fixture.Staff(t, db, p, fixture.Founder())
	client := fixture.Client(t, db, p)

	srv, session := newServerWithOnCall(t, db, owner)
	defer srv.Close()

	startedResp := reqtest.Post(t, requestsURL(srv.URL, p.ID, client.ID), reqtest.Session(session), reqtest.JSON(engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate, DoulaStaffID: &owner.ID}))
	defer startedResp.Body.Close()
	started := reqtest.DecodeStatus[engagementrequest.RequestResponse](t, startedResp, http.StatusCreated)
	onCallURL := srv.URL + "/api/practices/" + p.ID + "/engagements/" + started.EngagementID + "/on-call"

	atIntakeResp := reqtest.Get(t, onCallURL, reqtest.Session(session))
	defer atIntakeResp.Body.Close()
	atIntake := reqtest.DecodeStatus[oncall.EngagementOnCall](t, atIntakeResp, http.StatusOK)
	assertSoloOwnerOnCall(t, atIntake, owner.ID, "intake")

	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagements SET status = 'active' WHERE id = $1`, started.EngagementID,
	); err != nil {
		t.Fatalf("move engagement to active: %v", err)
	}

	activeResp := reqtest.Get(t, onCallURL, reqtest.Session(session))
	defer activeResp.Body.Close()
	active := reqtest.DecodeStatus[oncall.EngagementOnCall](t, activeResp, http.StatusOK)
	assertSoloOwnerOnCall(t, active, owner.ID, "active")

	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagements SET status = 'completed', ending_reason = 'care_complete', birth_outcome = 'unknown' WHERE id = $1`, started.EngagementID,
	); err != nil {
		t.Fatalf("move engagement to completed: %v", err)
	}

	completedResp := reqtest.Get(t, onCallURL, reqtest.Session(session))
	defer completedResp.Body.Close()
	completed := reqtest.DecodeStatus[oncall.EngagementOnCall](t, completedResp, http.StatusOK)
	if completed.Window != nil || completed.NoWindowReason != oncall.NoWindowCompleted {
		t.Fatalf("completed: window = %+v, reason = %q, want no window and %q",
			completed.Window, completed.NoWindowReason, oncall.NoWindowCompleted)
	}
}

// assertSoloOwnerOnCall fails unless the read has a window and names the
// Owner alone as the Doula on call.
func assertSoloOwnerOnCall(t *testing.T, got oncall.EngagementOnCall, ownerID, phase string) {
	t.Helper()
	if got.Window == nil {
		t.Fatalf("%s: window = none (%q), want one for a birth with a granted Attachment", phase, got.NoWindowReason)
	}
	if len(got.Doulas) != 1 || got.Doulas[0].StaffID != ownerID {
		t.Fatalf("%s: doulas on call = %+v, want the Owner alone", phase, got.Doulas)
	}
}
