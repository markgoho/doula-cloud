package oncall_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/oncall"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

func (e *env) narrow(staffID string, body oncall.NarrowingRequest, want int) {
	e.t.Helper()
	reqtest.ExpectStatus(e.t, http.MethodPut, e.narrowURL(staffID), want, e.asOwner, reqtest.JSON(body))
}

func TestNarrowing_APrimaryAndABackupEachCarryTheirOwnDays(t *testing.T) {
	e := newEnv(t)
	backup := e.backup()

	to, from := octTwentyTwo, octTwentyThree
	e.narrow(e.Doula.ID, oncall.NarrowingRequest{To: &to}, http.StatusOK)
	e.narrow(backup.ID, oncall.NarrowingRequest{From: &from}, http.StatusOK)

	days := map[string][2]string{}
	for _, d := range e.panel().Doulas {
		days[d.StaffID] = [2]string{*d.OnCallFrom, *d.OnCallTo}
	}
	if days[e.Doula.ID] != [2]string{windowStart, octTwentyTwo} || days[backup.ID] != [2]string{octTwentyThree, windowEnd} {
		t.Fatalf("on-call days = %v, want the primary to the 22nd and the backup from the 23rd", days)
	}
	if n := len(e.activityActors("on_call_narrowing_changed")); n != 2 {
		t.Fatalf("narrowing entries = %d, want 2", n)
	}

	// Clearing puts her back on the whole window.
	e.narrow(e.Doula.ID, oncall.NarrowingRequest{}, http.StatusOK)
	for _, d := range e.panel().Doulas {
		if d.StaffID == e.Doula.ID && (d.From != nil || *d.OnCallTo != windowEnd) {
			t.Fatalf("after clearing, primary = %+v, want the whole window", d)
		}
	}
}

func TestNarrowing_RefusesWhatItShould(t *testing.T) {
	e := newEnv(t)
	stranger := e.stranger()
	from, to, junk := "2026-10-25", octTwenty, "next week"

	tests := []struct {
		name   string
		who    reqtest.Option
		target string
		body   oncall.NarrowingRequest
		status int
		field  string
		want   string
	}{
		{"it ends before it starts", e.asOwner, e.Doula.ID, oncall.NarrowingRequest{From: &from, To: &to}, http.StatusBadRequest, "to", oncall.MsgNarrowOrder},
		{"it is not a date", e.asOwner, e.Doula.ID, oncall.NarrowingRequest{From: &junk}, http.StatusBadRequest, "from", oncall.MsgNarrowDate},
		{"the Doula is not on the birth", e.asOwner, stranger.ID, oncall.NarrowingRequest{}, http.StatusNotFound, "", ""},
		{"a Doula may not narrow anyone", e.as(e.Doula), e.Doula.ID, oncall.NarrowingRequest{}, http.StatusForbidden, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reqtest.Expect[refusal](t, http.MethodPut, e.narrowURL(tt.target), tt.status, tt.who, reqtest.JSON(tt.body))
			if tt.field != "" && got.Details[tt.field] != tt.want {
				t.Errorf("details = %v, want %q under %q", got.Details, tt.want, tt.field)
			}
		})
	}
}

func TestPanel_AContractorReadsOnlyABirthSheIsOn(t *testing.T) {
	e := newEnv(t)
	contractor := fixture.Staff(t, e.db, e.Practice, fixture.Contractor())
	asContractor := e.as(contractor)

	reqtest.ExpectStatus(t, http.MethodGet, e.panelURL(), http.StatusNotFound, asContractor)

	testdb.SeedGrantedAttachment(t, e.db, e.Birth.ID, contractor.ID)
	panel := reqtest.Expect[oncall.EngagementOnCall](t, http.MethodGet, e.panelURL(), http.StatusOK, asContractor)
	if panel.Window == nil || len(panel.Doulas) != 2 {
		t.Fatalf("panel = %+v, want the window and both Doulas on her own birth", panel)
	}
}

func TestPanel_StatesWhyThereIsNoWindow(t *testing.T) {
	e := newEnv(t)

	e.exec(`UPDATE engagements SET due_date = NULL WHERE id = $1`, e.Birth.ID)
	if panel := e.panel(); panel.NoWindowReason != oncall.NoWindowNoDueDate || panel.Window != nil {
		t.Fatalf("panel = %+v, want no window for want of a due date", panel)
	}

	e.exec(`UPDATE engagement_attachments SET ended_at = now(), ended_by = staff_id WHERE engagement_id = $1`, e.Birth.ID)
	if panel := e.panel(); panel.NoWindowReason != oncall.NoWindowNobodyAttached {
		t.Fatalf("panel = %+v, want no window because nobody is on it", panel)
	}

	unknown := e.practiceURL("/engagements/00000000-0000-0000-0000-000000000000/on-call")
	reqtest.ExpectStatus(t, http.MethodGet, unknown, http.StatusNotFound, e.asOwner)
}
