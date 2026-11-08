package oncall_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/oncall"
	"doula-cloud/api/internal/reqtest"
)

// gapEnv is the env with a backup on the birth and one uncovered gap
// already saved through the API.
type gapEnv struct {
	*env
	Backup fixture.StaffRecord
	GapID  string
}

func newGapEnv(t *testing.T) gapEnv {
	t.Helper()
	e := gapEnv{env: newEnv(t)}
	e.Backup = e.backup()
	e.GapID = e.recordGap(e.asOwner, gapBody(e.Doula.ID, nil)).ID
	e.forgetNudges()
	return e
}

func (e gapEnv) gapURL() string { return e.gapsURL() + "/" + e.GapID }

func (e gapEnv) save(who reqtest.Option, body oncall.GapRequest) oncall.Gap {
	e.t.Helper()
	return reqtest.Expect[oncall.Gap](e.t, http.MethodPut, e.gapURL(), http.StatusOK, who, reqtest.JSON(body))
}

func TestUpdateGap_AddingCoverQueuesNothingAndIsRecorded(t *testing.T) {
	e := newGapEnv(t)
	e.exec(`UPDATE coverage_gap_outbox SET status = 'sent', sent_at = now()`)

	gap := e.save(e.asOwner, gapBody(e.Doula.ID, &e.Backup.ID))

	if gap.CoveringStaffID == nil || *gap.CoveringStaffID != e.Backup.ID {
		t.Fatalf("covering = %v, want the backup", gap.CoveringStaffID)
	}
	if n := e.count(`SELECT count(*) FROM coverage_gap_outbox WHERE gap_id = $1`, e.GapID); n != 1 || e.nudges() != 0 {
		t.Fatalf("adding cover queued: %d rows (want the original 1) and %d nudges (want 0)", n, e.nudges())
	}
	if got := e.activityActors("coverage_gap_updated"); len(got) != 1 || got[0] != e.Owner.ID {
		t.Fatalf("updated entries = %v, want one naming the Owner", got)
	}
}

func TestUpdateGap_SavingItStillUncoveredQueuesAgain(t *testing.T) {
	e := newGapEnv(t)
	e.exec(`UPDATE coverage_gap_outbox SET status = 'sent', sent_at = now()`)

	e.save(e.asOwner, gapBody(e.Doula.ID, nil))

	if n := e.pendingNotices(e.GapID); n != 1 || e.nudges() != 1 {
		t.Fatalf("an uncovered save queued %d pending and %d nudges, want 1 and 1", n, e.nudges())
	}
}

func TestUpdateGap_TwoSavesBeforeTheMailGoesAreOneNotice(t *testing.T) {
	e := newGapEnv(t)

	e.save(e.asOwner, gapBody(e.Doula.ID, nil))

	if n := e.pendingNotices(e.GapID); n != 1 || e.nudges() != 0 {
		t.Fatalf("a second save before the send left %d pending and %d nudges, want 1 and 0", n, e.nudges())
	}
}

// TestGapWrites_NoDoulaIsAFieldRefusal is #1916: the gap form opens with
// no Doula chosen, so a write that names none is refused against the
// Doula field in the page's own words -- for an Owner, and for a Doula,
// who would otherwise be told she may record a gap only for herself.
func TestGapWrites_NoDoulaIsAFieldRefusal(t *testing.T) {
	e := newGapEnv(t)

	tests := []struct {
		name   string
		who    reqtest.Option
		method string
		url    string
	}{
		{"an Owner records one", e.asOwner, http.MethodPost, e.gapsURL()},
		{"an Owner edits one", e.asOwner, http.MethodPut, e.gapURL()},
		{"a Doula records one", e.as(e.Doula), http.MethodPost, e.gapsURL()},
		{"a Doula edits her own", e.as(e.Doula), http.MethodPut, e.gapURL()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reqtest.Expect[refusal](t, tt.method, tt.url, http.StatusBadRequest, tt.who, reqtest.JSON(gapBody("", nil)))
			if got.Details["staffId"] != oncall.MsgStaffIDNeeded {
				t.Fatalf("details = %v, want staffId %q", got.Details, oncall.MsgStaffIDNeeded)
			}
		})
	}
	if n := e.count(`SELECT count(*) FROM engagement_coverage_gaps WHERE staff_id = $1`, e.Doula.ID); n != 1 {
		t.Fatalf("gaps = %d, want only the fixture's own", n)
	}
}

func TestUpdateGap_ADoulaCannotEditAColleaguesGap(t *testing.T) {
	e := newGapEnv(t)
	backup := e.as(e.Backup)

	reqtest.ExpectStatus(t, http.MethodPut, e.gapURL(), http.StatusForbidden, backup, reqtest.JSON(gapBody(e.Doula.ID, &e.Backup.ID)))
	reqtest.ExpectStatus(t, http.MethodDelete, e.gapURL(), http.StatusForbidden, backup)
}

func TestClearGap_ClearsOnceAndRecordsIt(t *testing.T) {
	e := newGapEnv(t)

	reqtest.ExpectStatus(t, http.MethodDelete, e.gapURL(), http.StatusNoContent, e.asOwner)

	if n := e.count(`SELECT count(*) FROM engagement_coverage_gaps WHERE id = $1 AND cleared_by = $2 AND cleared_at IS NOT NULL`, e.GapID, e.Owner.ID); n != 1 {
		t.Fatal("the gap row was not cleared by the Owner; a clear must keep the row")
	}
	if got := e.activityActors("coverage_gap_cleared"); len(got) != 1 || got[0] != e.Owner.ID {
		t.Fatalf("cleared entries = %v, want one naming the Owner", got)
	}
	reqtest.ExpectStatus(t, http.MethodDelete, e.gapURL(), http.StatusNotFound, e.asOwner)
	reqtest.ExpectStatus(t, http.MethodPut, e.gapURL(), http.StatusNotFound, e.asOwner, reqtest.JSON(gapBody(e.Doula.ID, nil)))
}
