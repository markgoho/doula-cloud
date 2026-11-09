package oncall_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/oncall"
	"doula-cloud/api/internal/reqtest"
)

func TestRoster_WindowIsDerivedAndMovesWithTheDueDate(t *testing.T) {
	e := newEnv(t)

	got := e.roster(octFirst, octLast)
	if len(got.Windows) != 1 || got.Windows[0].Window != (oncall.Window{Start: windowStart, End: windowEnd}) {
		t.Fatalf("windows = %+v, want one 2026-10-09..2026-11-13", got.Windows)
	}

	e.Exec(`UPDATE engagements SET due_date = '2026-11-20' WHERE id = $1`, e.Birth.ID)
	got = e.roster(octFirst, "2026-12-31")
	if got.Windows[0].Window != (oncall.Window{Start: birthDue, End: "2026-12-04"}) {
		t.Fatalf("after correcting the due date, window = %+v, want 2026-10-30..2026-12-04", got.Windows[0].Window)
	}

	e.Exec(`UPDATE engagements SET birth_outcome = 'live_birth', pregnancy_ended_on = '2026-11-18' WHERE id = $1`, e.Birth.ID)
	got = e.roster(octFirst, "2026-12-31")
	if got.Windows[0].Window.End != "2026-11-18" {
		t.Fatalf("after recording the birth, window ends %s, want 2026-11-18", got.Windows[0].Window.End)
	}

	got = e.roster("2026-11-19", "2026-11-30")
	if len(got.Windows) != 0 {
		t.Fatalf("a range after the birth still shows %+v", got.Windows)
	}
}

func TestRoster_TheGrantDateRuleAndAPerEngagementOverride(t *testing.T) {
	e := newEnv(t)
	e.Exec(`UPDATE engagement_attachments SET attached_at = '2026-06-01T15:00:00Z' WHERE engagement_id = $1`, e.Birth.ID)

	e.Exec(`UPDATE practices SET on_call_start_rule = 'attachment_granted' WHERE id = $1`, e.Practice.ID)
	if got := e.roster(grantedOn, "2026-06-30"); len(got.Windows) != 1 || got.Windows[0].Window.Start != grantedOn {
		t.Fatalf("under the grant-date rule, windows = %+v, want one opening 2026-06-01", got.Windows)
	}

	e.Exec(`UPDATE engagements SET on_call_start_rule = 'gestational_week', on_call_start_week = 38 WHERE id = $1`, e.Birth.ID)
	if got := e.roster(octFirst, octLast); got.Windows[0].Window.Start != "2026-10-16" {
		t.Fatalf("with a 38-week override, window starts %s, want 2026-10-16", got.Windows[0].Window.Start)
	}
}

// TestRoster_AnIntakeBirthWithADoulaIsOnTheRoster is #1616: the Attachment
// is the fact that somebody is on the birth, so the roster shows an
// `intake` birth, and only `completed` takes it off.
func TestRoster_AnIntakeBirthWithADoulaIsOnTheRoster(t *testing.T) {
	e := newEnv(t)
	e.Exec(`UPDATE engagements SET status = 'intake' WHERE id = $1`, e.Birth.ID)

	got := e.roster(octFirst, octLast)
	if len(got.Windows) != 1 || got.Windows[0].Window != (oncall.Window{Start: windowStart, End: windowEnd}) {
		t.Fatalf("an intake birth: windows = %+v, want one 2026-10-09..2026-11-13", got.Windows)
	}

	e.Exec(`UPDATE engagements SET status = 'completed', ending_reason = 'care_complete', birth_outcome = 'unknown' WHERE id = $1`, e.Birth.ID)
	if got := e.roster(octFirst, octLast); len(got.Windows) != 0 {
		t.Fatalf("a completed birth reached the roster: %+v", got.Windows)
	}
}

func TestRoster_APostpartumEngagementNeverHasAWindow(t *testing.T) {
	e := newEnv(t)
	e.Exec(`UPDATE engagements SET kind = 'postpartum' WHERE id = $1`, e.Birth.ID)

	got := e.roster(octFirst, octLast)

	if len(got.Windows) != 0 || len(got.NoWindow) != 0 {
		t.Fatalf("a postpartum Engagement reached the roster: %+v %+v", got.Windows, got.NoWindow)
	}
}

func TestRoster_NarrowingsAndHoles(t *testing.T) {
	e := newEnv(t)
	backup := e.backup()

	// Primary through the 22nd, backup from the 23rd: no hole.
	e.Exec(`UPDATE engagement_attachments SET on_call_to = '2026-10-22' WHERE staff_id = $1`, e.Doula.ID)
	e.Exec(`UPDATE engagement_attachments SET on_call_from = '2026-10-23' WHERE staff_id = $1`, backup.ID)

	w := e.roster(octTwenty, "2026-10-25").Windows[0]
	if len(w.OnCall) != 2 || w.Uncovered || len(w.UnstaffedDays) != 0 {
		t.Fatalf("primary-then-backup = %+v, want both on call and no hole", w)
	}
	if !w.OnCall[0].Narrowed {
		t.Error("a narrowed Doula reads as on call for the whole window")
	}

	// A single day seen from the primary's side only lists the primary.
	if got := e.roster("2026-10-21", "2026-10-21"); len(got.Windows[0].OnCall) != 1 || got.Windows[0].OnCall[0].StaffID != e.Doula.ID {
		t.Fatalf("on the 21st, onCall = %+v, want only the primary", got.Windows[0].OnCall)
	}

	// Moving the backup's start leaves the 23rd and 24th with nobody.
	e.Exec(`UPDATE engagement_attachments SET on_call_from = '2026-10-25' WHERE staff_id = $1`, backup.ID)
	w = e.roster(octTwenty, octLast).Windows[0]
	if !w.Uncovered || len(w.UnstaffedDays) != 1 || w.UnstaffedDays[0] != (oncall.Window{Start: octTwentyThree, End: "2026-10-24"}) {
		t.Fatalf("with a hole, window = %+v, want the 23rd..24th unstaffed", w)
	}
}

func TestRoster_AGapWithNobodyCoveringItIsAHole(t *testing.T) {
	e := newEnv(t)
	e.Exec(`INSERT INTO engagement_coverage_gaps (engagement_id, staff_id, starts_at, ends_at, created_by)
		VALUES ($1, $2, '2026-10-17T22:00:00Z', '2026-10-18T10:00:00Z', $2)`, e.Birth.ID, e.Doula.ID)

	w := e.roster("2026-10-17", "2026-10-17").Windows[0]
	if !w.Uncovered || len(w.Gaps) != 1 || w.Gaps[0].CoveringStaffID != nil {
		t.Fatalf("an uncovered gap = %+v, want it shown and the window uncovered", w)
	}
	if w.Gaps[0].StaffName != "Maya Primary" {
		t.Errorf("gap staffName = %q, want the Doula who cannot be reached", w.Gaps[0].StaffName)
	}

	backup := e.backup()
	e.Exec(`UPDATE engagement_coverage_gaps SET covering_staff_id = $1`, backup.ID)
	w = e.roster("2026-10-17", "2026-10-17").Windows[0]
	if w.Uncovered || w.Gaps[0].CoveringStaffName == nil || *w.Gaps[0].CoveringStaffName != backupName {
		t.Fatalf("a covered gap = %+v, want it shown as covered by Bo Backup", w)
	}

	e.Exec(`UPDATE engagement_coverage_gaps SET cleared_at = now(), cleared_by = $1`, backup.ID)
	if w := e.roster("2026-10-17", "2026-10-17").Windows[0]; len(w.Gaps) != 0 {
		t.Fatalf("a cleared gap is still on the roster: %+v", w.Gaps)
	}
}

func TestRoster_RefusesAMalformedRange(t *testing.T) {
	e := newEnv(t)

	tests := []struct {
		name  string
		query string
		want  string
	}{
		{"a from that is not a date", "?from=tomorrow", oncall.MsgInvalidFrom},
		{"a to that is not a date", "?from=2026-10-01&to=soon", oncall.MsgInvalidTo},
		{"a range that ends before it starts", "?from=2026-10-10&to=2026-10-01", oncall.MsgEmptyRange},
		{"a range longer than the roster reads", "?from=2026-01-01&to=2026-12-31", oncall.MsgRangeTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reqtest.Expect[refusal](t, http.MethodGet, e.PracticeURL("/on-call"+tt.query), http.StatusBadRequest, e.SignedIn)
			if got.Message != tt.want {
				t.Errorf("message = %v, want %q", got.Message, tt.want)
			}
		})
	}
}

func TestRoster_DefaultsToTodayInThePracticeZone(t *testing.T) {
	e := newEnv(t)
	loc, _ := time.LoadLocation(zone)
	today := time.Now().In(loc).Format("2006-01-02")

	got := reqtest.Expect[oncall.RosterResponse](t, http.MethodGet, e.PracticeURL("/on-call"), http.StatusOK, e.SignedIn)

	if got.From != today || got.To != today {
		t.Fatalf("range = %s..%s, want today %s", got.From, got.To, today)
	}
}
