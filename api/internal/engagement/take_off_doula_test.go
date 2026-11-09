package engagement_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// The tests in this file are #2596: an Owner or an Admin takes a Doula off
// an Engagement. The Attachment is ended, never deleted; the ledger says
// who and when; the Doula gets one mail, unless she is the one who
// pressed; her Visits that have not happened stay hers.

// takeOff takes staffID off engagementID as the Staff member at the
// screen and requires want.
func (e *env) takeOff(engagementID, staffID string, want int) attachSelfBody {
	e.T.Helper()
	return reqtest.Expect[attachSelfBody](e.T, http.MethodDelete, e.urlOf(engagementID, "/attachments/"+staffID), want, e.SignedIn)
}

func (e *env) countTakenOffNotices(staffID string) int {
	e.T.Helper()
	return testdb.Scalar[int](e.T, e.DB.Admin,
		`SELECT count(*) FROM staff_notice_outbox
		  WHERE kind = 'taken_off_engagement' AND subject_id = $1 AND recipient_staff_id = $2`,
		e.Engagement.ID, staffID)
}

func (e *env) seedVisitAt(staffID, when string) {
	e.T.Helper()
	e.Exec(`INSERT INTO visits (engagement_id, staff_id, scheduled_at) VALUES ($1, $2, $3::timestamptz)`, e.Engagement.ID, staffID, when)
}

// ownerWithDoulaOnIt is the Owner at the screen and a Doula she has put
// on an active Engagement.
func ownerWithDoulaOnIt(t *testing.T) attachingEnv {
	t.Helper()
	e := newAttachingEnv(t, ownerCase, nil, withEngagement(fixture.Status(fixture.StatusActive)))
	e.put(http.StatusOK)
	return e
}

func TestTakeOffDoulaHandler_EndsTheAttachmentAndRecordsIt(t *testing.T) {
	e := ownerWithDoulaOnIt(t)
	e.seedVisitAt(e.Doula.ID, "2999-01-01T12:00:00Z")
	e.seedVisitAt(e.Doula.ID, "2001-01-01T12:00:00Z")
	e.Exec(`INSERT INTO visits (engagement_id, staff_id) VALUES ($1, $2)`, e.Engagement.ID, e.Doula.ID)

	on := e.put(http.StatusOK)
	if len(on.Doulas) != 1 || on.Doulas[0].OpenVisits != 2 {
		t.Fatalf("doulas before = %+v, want one with 2 Visits that have not happened (one future, one undated; the past one is not counted)", on.Doulas)
	}

	body := e.takeOff(e.Engagement.ID, e.Doula.ID, http.StatusOK)

	if len(body.Doulas) != 0 {
		t.Fatalf("doulas = %+v, want nobody on it", body.Doulas)
	}
	var endedBy string
	var rows int
	if err := e.DB.Admin.QueryRowContext(t.Context(),
		`SELECT count(*), max(ended_by::text) FROM engagement_attachments
		  WHERE engagement_id = $1 AND staff_id = $2 AND ended_at IS NOT NULL`,
		e.Engagement.ID, e.Doula.ID).Scan(&rows, &endedBy); err != nil {
		t.Fatalf("read ended attachment: %v", err)
	}
	if rows != 1 || endedBy != e.Staff.ID {
		t.Fatalf("ended rows = %d by %q, want one ended by the caller", rows, endedBy)
	}
	assertDiff(t, readFactDiff(t, e.DB, e.Engagement.ID, "doula_taken_off"), map[string]any{activity.DiffKeyAttachedStaffID: e.Doula.ID})
	ledger := e.activityAs(e.Staff)
	var found bool
	for _, entry := range ledger.Items {
		if entry.Action == "doula_taken_off" {
			found = true
			if entry.ActorName != attachingOwnerName || entry.Detail != "Taken off this work as the Doula: "+attachedDoulaName || entry.CreatedAt.IsZero() {
				t.Fatalf("ledger entry = %+v, want who, whom and when", entry)
			}
		}
	}
	if !found {
		t.Fatalf("ledger = %+v, want a doula_taken_off entry", ledger.Items)
	}
	if n := e.countTakenOffNotices(e.Doula.ID); n != 1 {
		t.Fatalf("notices = %d, want 1", n)
	}
	if visits := testdb.Scalar[int](t, e.DB.Admin, `SELECT count(*) FROM visits WHERE engagement_id = $1 AND staff_id = $2`, e.Engagement.ID, e.Doula.ID); visits != 3 {
		t.Fatalf("visits = %d, want all three still hers", visits)
	}
}

func TestTakeOffDoulaHandler_SafeToRepeat(t *testing.T) {
	e := ownerWithDoulaOnIt(t)

	e.takeOff(e.Engagement.ID, e.Doula.ID, http.StatusOK)
	e.takeOff(e.Engagement.ID, e.Doula.ID, http.StatusOK)

	if n := e.countActivity("doula_taken_off"); n != 1 {
		t.Fatalf("doula_taken_off entries = %d, want 1", n)
	}
	if n := e.countTakenOffNotices(e.Doula.ID); n != 1 {
		t.Fatalf("notices = %d, want 1", n)
	}
}

func TestTakeOffDoulaHandler_NobodyIsMailedAboutHerOwnTakingOff(t *testing.T) {
	e := newAttachingEnv(t, roleCase{"owner and doula", []fixture.StaffOption{fixture.Owner(), fixture.Doula()}}, nil,
		withEngagement(fixture.Status(fixture.StatusActive)))
	e.attachDoula(e.Engagement.ID, e.Staff.ID, http.StatusOK)

	e.takeOff(e.Engagement.ID, e.Staff.ID, http.StatusOK)

	if n := e.countActivity("doula_taken_off"); n != 1 {
		t.Fatalf("doula_taken_off entries = %d, want 1", n)
	}
	if n := e.countTakenOffNotices(e.Staff.ID); n != 0 {
		t.Fatalf("notices = %d, want none for the person who pressed", n)
	}
}

func TestTakeOffDoulaHandler_RefusalsAndRoles(t *testing.T) {
	e := ownerWithDoulaOnIt(t)
	const missing = "11111111-1111-1111-1111-111111111111"

	cases := []struct {
		name         string
		engagementID string
		staffID      string
		wantStatus   int
	}{
		{"unknown engagement", missing, e.Doula.ID, http.StatusNotFound},
		{"malformed engagement", malformedUUID, e.Doula.ID, http.StatusBadRequest},
		{"malformed staff", e.Engagement.ID, malformedUUID, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(*testing.T) {
			e.takeOff(tc.engagementID, tc.staffID, tc.wantStatus)
		})
	}

	t.Run("a doula cannot take a colleague off", func(t *testing.T) {
		reqtest.ExpectStatus(t, http.MethodDelete, e.urlOf(e.Engagement.ID, "/attachments/"+e.Doula.ID), http.StatusForbidden, e.As(e.Doula))

		if got := readOpenAttachments(t, e.DB, e.Engagement.ID, e.Doula.ID); len(got) != 1 {
			t.Fatalf("attachment = %+v, want it still open", got)
		}
	})
}
