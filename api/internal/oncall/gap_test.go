package oncall_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/oncall"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// Saturday evening inside the solo fixture's 2026-10-09..2026-11-13 window.
var (
	gapStart = time.Date(2026, 10, 17, 22, 0, 0, 0, time.UTC)
	gapEnd   = time.Date(2026, 10, 18, 10, 0, 0, 0, time.UTC)
)

// The GapRequest fields a range refusal is keyed under.
const (
	fieldStartsAt = "startsAt"
	fieldEndsAt   = "endsAt"
)

func gapBody(staffID string, covering *string) oncall.GapRequest {
	reason := "  Sister's wedding  "
	return oncall.GapRequest{StaffID: staffID, StartsAt: &gapStart, EndsAt: &gapEnd, Reason: &reason, CoveringStaffID: covering}
}

// recordGap is a POST of body to the birth's gaps as who.
func (e *env) recordGap(who reqtest.Option, body oncall.GapRequest) oncall.Gap {
	e.t.Helper()
	return reqtest.Expect[oncall.Gap](e.t, http.MethodPost, e.gapsURL(), http.StatusCreated, who, reqtest.JSON(body))
}

func (e *env) pendingNotices(gapID string) int {
	e.t.Helper()
	return e.count(`SELECT count(*) FROM coverage_gap_outbox WHERE gap_id = $1 AND status = 'pending'`, gapID)
}

func TestCreateGap_AnUncoveredGapIsRecordedAndQueuesOneNotice(t *testing.T) {
	e := newEnv(t)

	gap := e.recordGap(e.asOwner, gapBody(e.Doula.ID, nil))

	if gap.StaffID != e.Doula.ID || !gap.StartsAt.Equal(gapStart) || gap.CoveringStaffID != nil {
		t.Fatalf("gap = %+v", gap)
	}
	if gap.Reason == nil || *gap.Reason != "Sister's wedding" {
		t.Errorf("reason = %v, want it trimmed", gap.Reason)
	}
	if got := e.activityActors("coverage_gap_created"); len(got) != 1 || got[0] != e.Owner.ID {
		t.Errorf("created entries = %v, want one naming the Owner %s", got, e.Owner.ID)
	}
	if n := e.pendingNotices(gap.ID); n != 1 {
		t.Errorf("pending notices = %d, want 1", n)
	}
	if calls := e.enq.Calls(); len(calls) != 1 || calls[0] != tasknudge.CoverageGap {
		t.Errorf("nudges = %v, want one %s", calls, tasknudge.CoverageGap)
	}
}

func TestCreateGap_ACoveredGapQueuesNothing(t *testing.T) {
	e := newEnv(t)
	backup := e.backup()

	gap := e.recordGap(e.asOwner, gapBody(e.Doula.ID, &backup.ID))

	if gap.CoveringStaffName == nil || *gap.CoveringStaffName != backupName {
		t.Fatalf("covering = %v, want Bo Backup", gap.CoveringStaffName)
	}
	if n := e.pendingNotices(gap.ID); n != 0 || len(e.enq.Calls()) != 0 {
		t.Fatalf("a covered gap queued %d notices and %d nudges, want none", n, len(e.enq.Calls()))
	}
}

func TestCreateGap_RefusesWhatItShould(t *testing.T) {
	e := newEnv(t)
	stranger := e.stranger()
	outside := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	early := gapStart.Add(-time.Hour)
	doulaID := e.Doula.ID

	tests := []struct {
		name  string
		body  oncall.GapRequest
		field string
		want  string
	}{
		{"the Doula is not on the birth", gapBody(stranger.ID, nil), "staffId", oncall.MsgNotOnCall},
		{"the cover is not on the birth, so a gap cannot attach her", gapBody(doulaID, &stranger.ID), "coveringStaffId", oncall.MsgNotOnCall},
		{"the cover is the same Doula", gapBody(doulaID, &doulaID), "coveringStaffId", oncall.MsgCoverIsTheSame},
		// #1969: an empty or reversed range is refused against the field
		// the page shows it under, in the page's own words.
		{"it ends before it starts", oncall.GapRequest{StaffID: doulaID, StartsAt: &gapStart, EndsAt: &early}, fieldEndsAt, oncall.MsgEndsBeforeStarts},
		{"it ends the moment it starts", oncall.GapRequest{StaffID: doulaID, StartsAt: &gapStart, EndsAt: &gapStart}, fieldEndsAt, oncall.MsgEndsBeforeStarts},
		{"it runs past the window", oncall.GapRequest{StaffID: doulaID, StartsAt: &gapStart, EndsAt: &outside}, fieldStartsAt, oncall.MsgOutsideWindow},
		{"it has no start", oncall.GapRequest{StaffID: doulaID, EndsAt: &gapEnd}, fieldStartsAt, oncall.MsgStartsAtRequired},
		{"it has no end", oncall.GapRequest{StaffID: doulaID, StartsAt: &gapStart}, fieldEndsAt, oncall.MsgEndsAtRequired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reqtest.Expect[refusal](t, http.MethodPost, e.gapsURL(), http.StatusBadRequest, e.asOwner, reqtest.JSON(tt.body))
			if got.Details[tt.field] != tt.want {
				t.Fatalf("details = %v, want %q under %q", got.Details, tt.want, tt.field)
			}
		})
	}

	t.Run("neither end is given, so both are refused at once", func(t *testing.T) {
		got := reqtest.Expect[refusal](t, http.MethodPost, e.gapsURL(), http.StatusBadRequest, e.asOwner, reqtest.JSON(oncall.GapRequest{StaffID: doulaID}))
		if got.Details[fieldStartsAt] != oncall.MsgStartsAtRequired || got.Details[fieldEndsAt] != oncall.MsgEndsAtRequired {
			t.Fatalf("details = %v, want both ends refused", got.Details)
		}
	})
	if n := e.count(`SELECT count(*) FROM engagement_coverage_gaps`); n != 0 {
		t.Fatalf("%d gaps were written by refused requests", n)
	}
}

// TestCreateGap_AnIntakeBirthHasAWindowToRecordAGapIn is #1616: a birth
// with a Doula on it has a window from intake, so a gap can be recorded.
func TestCreateGap_AnIntakeBirthHasAWindowToRecordAGapIn(t *testing.T) {
	e := newEnv(t)
	e.exec(`UPDATE engagements SET status = 'intake' WHERE id = $1`, e.Birth.ID)

	e.recordGap(e.asOwner, gapBody(e.Doula.ID, nil))
}

// TestCreateGap_SaysWhichReasonThereIsNoWindow: "add a due date" is the
// wrong advice for four of the five reasons a birth has no window, so
// each says its own sentence -- the same rule the roster follows when it
// states why a birth has no window rather than guessing.
func TestCreateGap_SaysWhichReasonThereIsNoWindow(t *testing.T) {
	tests := []struct {
		name  string
		fixup string
		want  string
	}{
		{"there is no due date", `UPDATE engagements SET due_date = NULL WHERE id = $1`, oncall.MsgNoWindowNoDueDate},
		{"postpartum care is not on-call work", `UPDATE engagements SET kind = 'postpartum' WHERE id = $1`, oncall.MsgNoWindowPostpartum},
		{"care has ended", `UPDATE engagements SET status = 'completed', ending_reason = 'care_complete', birth_outcome = 'unknown' WHERE id = $1`, oncall.MsgNoWindowCompleted},
		{"nobody is on the birth", `UPDATE engagement_attachments SET ended_at = now(), ended_by = staff_id WHERE engagement_id = $1`, oncall.MsgNoWindowNobodyOnIt},
		{"the baby arrived before on call would have opened", `UPDATE engagements SET birth_outcome = 'live_birth', pregnancy_ended_on = '2026-09-20' WHERE id = $1`, oncall.MsgNoWindowEnded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			e.exec(tt.fixup, e.Birth.ID)

			got := reqtest.Expect[refusal](t, http.MethodPost, e.gapsURL(), http.StatusConflict, e.asOwner, reqtest.JSON(gapBody(e.Doula.ID, nil)))

			if got.Message != tt.want {
				t.Fatalf("message = %v, want %q", got.Message, tt.want)
			}
		})
	}
}

func TestCreateGap_ADoulaRecordsOnlyHerOwn(t *testing.T) {
	e := newEnv(t)
	colleague := e.backup()

	reqtest.ExpectStatus(t, http.MethodPost, e.gapsURL(), http.StatusForbidden, e.as(colleague), reqtest.JSON(gapBody(e.Doula.ID, nil)))
	e.recordGap(e.as(colleague), gapBody(colleague.ID, nil))
}

func TestCreateGap_AContractorReachesOnlyHerOwnBirth(t *testing.T) {
	e := newEnv(t)
	contractor := fixture.Staff(t, e.db, e.Practice, fixture.Contractor())

	reqtest.ExpectStatus(t, http.MethodPost, e.gapsURL(), http.StatusNotFound, e.as(contractor), reqtest.JSON(gapBody(contractor.ID, nil)))

	testdb.SeedGrantedAttachment(t, e.db, e.Birth.ID, contractor.ID)
	e.recordGap(e.as(contractor), gapBody(contractor.ID, nil))
}
