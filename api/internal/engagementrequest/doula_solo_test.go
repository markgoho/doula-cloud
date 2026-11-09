package engagementrequest_test

import (
	"database/sql"
	"net/http"
	"testing"

	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/oncall"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
)

// alsoMountingOnCall mounts oncall's surface beside this package's, so a
// test can start an Engagement and then read its On-call window the way the
// Engagement page does.
var alsoMountingOnCall = fixture.AlsoMounting(func(g *staffauth.GatedRouter, ir *idempotency.Router, _ *sql.DB) {
	oncall.Mount(g, ir, &tasknudge.FakeEnqueuer{})
})

// TestRequestHandler_ASoloOwnerStartsWorkInOneAct is the case #1515 was
// decided for. A solo Owner starts work: the ask and its approval are one
// act (ADR-0017), and when she names herself the same act attaches her,
// granted, with herself recorded as the person who attached. Nobody offers
// herself work. #1625 holds it for an Owner who is a contractor Doula,
// whom nobody can send an Offer. An agency Owner who names nobody reaches
// First Value (ADR-0048) with nobody on the Engagement.
func TestRequestHandler_ASoloOwnerStartsWorkInOneAct(t *testing.T) {
	for _, tc := range []struct {
		name      string
		owner     []fixture.StaffOption
		namesSelf bool
		attached  bool
	}{
		{"an Owner names herself", []fixture.StaffOption{fixture.Founder()}, true, true},
		{"a contractor Owner names herself", []fixture.StaffOption{fixture.Founder(), fixture.Contractor()}, true, true},
		{"an agency Owner names nobody", []fixture.StaffOption{fixture.Founder()}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.owner...), funded)
			body := birthAsk
			if tc.namesSelf {
				body.DoulaStaffID = &e.Staff.ID
			}

			out := e.ask(body)

			if out.State != testStateApproved || out.EngagementID == "" {
				t.Fatalf("response = %+v, want approved with an engagementId", out)
			}
			if tc.attached {
				e.requireAttached(out.EngagementID, e.Staff.ID, e.Staff.ID)
			} else if got := attachmentsOn(t, e.DB, out.EngagementID); len(got) != 0 {
				t.Fatalf("attachments = %+v, want none", got)
			}
		})
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
	e := newEnv(t, signedInAs(fixture.Founder()), funded, alsoMountingOnCall)
	started := e.ask(engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate, DoulaStaffID: &e.Staff.ID})
	onCall := func() oncall.EngagementOnCall {
		return reqtest.Expect[oncall.EngagementOnCall](t, http.MethodGet, e.PracticeURL("/engagements/"+started.EngagementID+"/on-call"), http.StatusOK, e.SignedIn)
	}

	assertSoloOwnerOnCall(t, onCall(), e.Staff.ID, "intake")

	e.Exec(`UPDATE engagements SET status = 'active' WHERE id = $1`, started.EngagementID)

	assertSoloOwnerOnCall(t, onCall(), e.Staff.ID, "active")

	e.Exec(`UPDATE engagements SET status = 'completed', ending_reason = 'care_complete', birth_outcome = 'unknown' WHERE id = $1`, started.EngagementID)

	completed := onCall()
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
