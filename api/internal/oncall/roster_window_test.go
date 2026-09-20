package oncall_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/oncall"
	"doula-cloud/api/internal/testdb"
)

// soloFixture is one Practice, one Owner reading, one employee Doula
// and one active birth due 2026-10-30, whose default window is
// 2026-10-09..2026-11-13.
type soloFixture struct {
	practiceID, ownerUID, doulaID, engagementID string
}

func newSoloFixture(t *testing.T, db *testdb.DB, prefix string) soloFixture {
	t.Helper()
	f := soloFixture{ownerUID: prefix + "-owner"}
	f.practiceID, _ = testdb.SeedStaffAtNewPractice(t, db, f.ownerUID, []string{ownerRole}, employeeType)
	testdb.SetPracticeTimezone(t, db, f.practiceID, zone)
	f.doulaID = testdb.SeedNamedStaffAtPractice(t, db, f.practiceID, prefix+"-doula", "Maya Primary", []string{doulaRole}, employeeType)
	f.engagementID = seedBirth(t, db, f.practiceID, "Ada Whitfield", "2026-10-30")
	testdb.SeedGrantedAttachment(t, db, f.engagementID, f.doulaID)
	return f
}

func (f soloFixture) roster(t *testing.T, db *testdb.DB, from, to string) oncall.RosterResponse {
	t.Helper()
	srv, session := newServer(t, db, f.ownerUID)
	defer srv.Close()
	return decode[oncall.RosterResponse](t, authedGet(t, session, rosterURL(srv.URL, f.practiceID, from, to)), http.StatusOK)
}

func TestRoster_WindowIsDerivedAndMovesWithTheDueDate(t *testing.T) {
	db := testdb.New(t)
	f := newSoloFixture(t, db, "roster-moves")

	got := f.roster(t, db, "2026-10-01", "2026-10-31")
	if len(got.Windows) != 1 || got.Windows[0].Window != (oncall.Window{Start: "2026-10-09", End: "2026-11-13"}) {
		t.Fatalf("windows = %+v, want one 2026-10-09..2026-11-13", got.Windows)
	}

	exec(t, db, `UPDATE engagements SET due_date = '2026-11-20' WHERE id = $1`, f.engagementID)
	got = f.roster(t, db, "2026-10-01", "2026-12-31")
	if got.Windows[0].Window != (oncall.Window{Start: "2026-10-30", End: "2026-12-04"}) {
		t.Fatalf("after correcting the due date, window = %+v, want 2026-10-30..2026-12-04", got.Windows[0].Window)
	}

	exec(t, db, `UPDATE engagements SET birth_outcome = 'live_birth', pregnancy_ended_on = '2026-11-18' WHERE id = $1`, f.engagementID)
	got = f.roster(t, db, "2026-10-01", "2026-12-31")
	if got.Windows[0].Window.End != "2026-11-18" {
		t.Fatalf("after recording the birth, window ends %s, want 2026-11-18", got.Windows[0].Window.End)
	}

	got = f.roster(t, db, "2026-11-19", "2026-11-30")
	if len(got.Windows) != 0 {
		t.Fatalf("a range after the birth still shows %+v", got.Windows)
	}
}

func TestRoster_TheGrantDateRuleAndAPerEngagementOverride(t *testing.T) {
	db := testdb.New(t)
	f := newSoloFixture(t, db, "roster-rule")
	exec(t, db, `UPDATE engagement_attachments SET attached_at = '2026-06-01T15:00:00Z' WHERE engagement_id = $1`, f.engagementID)

	exec(t, db, `UPDATE practices SET on_call_start_rule = 'attachment_granted' WHERE id = $1`, f.practiceID)
	if got := f.roster(t, db, "2026-06-01", "2026-06-30"); len(got.Windows) != 1 || got.Windows[0].Window.Start != "2026-06-01" {
		t.Fatalf("under the grant-date rule, windows = %+v, want one opening 2026-06-01", got.Windows)
	}

	exec(t, db, `UPDATE engagements SET on_call_start_rule = 'gestational_week', on_call_start_week = 38 WHERE id = $1`, f.engagementID)
	if got := f.roster(t, db, "2026-10-01", "2026-10-31"); got.Windows[0].Window.Start != "2026-10-16" {
		t.Fatalf("with a 38-week override, window starts %s, want 2026-10-16", got.Windows[0].Window.Start)
	}
}

func TestRoster_APostpartumEngagementNeverHasAWindow(t *testing.T) {
	db := testdb.New(t)
	f := newSoloFixture(t, db, "roster-postpartum")
	exec(t, db, `UPDATE engagements SET kind = 'postpartum' WHERE id = $1`, f.engagementID)

	got := f.roster(t, db, "2026-10-01", "2026-10-31")
	if len(got.Windows) != 0 || len(got.NoWindow) != 0 {
		t.Fatalf("a postpartum Engagement reached the roster: %+v %+v", got.Windows, got.NoWindow)
	}
}

func TestRoster_NarrowingsAndHoles(t *testing.T) {
	db := testdb.New(t)
	f := newSoloFixture(t, db, "roster-narrow")
	backupID := testdb.SeedNamedStaffAtPractice(t, db, f.practiceID, "roster-narrow-backup", "Bo Backup", []string{doulaRole}, employeeType)
	testdb.SeedGrantedAttachment(t, db, f.engagementID, backupID)

	// Primary through the 22nd, backup from the 23rd: no hole.
	exec(t, db, `UPDATE engagement_attachments SET on_call_to = '2026-10-22' WHERE staff_id = $1`, f.doulaID)
	exec(t, db, `UPDATE engagement_attachments SET on_call_from = '2026-10-23' WHERE staff_id = $1`, backupID)

	got := f.roster(t, db, "2026-10-20", "2026-10-25")
	w := got.Windows[0]
	if len(w.OnCall) != 2 || w.Uncovered || len(w.UnstaffedDays) != 0 {
		t.Fatalf("primary-then-backup = %+v, want both on call and no hole", w)
	}
	if !w.OnCall[0].Narrowed {
		t.Error("a narrowed Doula reads as on call for the whole window")
	}

	// A single day seen from the primary's side only lists the primary.
	if got := f.roster(t, db, "2026-10-21", "2026-10-21"); len(got.Windows[0].OnCall) != 1 || got.Windows[0].OnCall[0].StaffID != f.doulaID {
		t.Fatalf("on the 21st, onCall = %+v, want only the primary", got.Windows[0].OnCall)
	}

	// Moving the backup's start leaves the 23rd and 24th with nobody.
	exec(t, db, `UPDATE engagement_attachments SET on_call_from = '2026-10-25' WHERE staff_id = $1`, backupID)
	w = f.roster(t, db, "2026-10-20", "2026-10-31").Windows[0]
	if !w.Uncovered || len(w.UnstaffedDays) != 1 || w.UnstaffedDays[0] != (oncall.Window{Start: "2026-10-23", End: "2026-10-24"}) {
		t.Fatalf("with a hole, window = %+v, want the 23rd..24th unstaffed", w)
	}
}

func TestRoster_AGapWithNobodyCoveringItIsAHole(t *testing.T) {
	db := testdb.New(t)
	f := newSoloFixture(t, db, "roster-gap")
	exec(t, db, `INSERT INTO engagement_coverage_gaps (engagement_id, staff_id, starts_at, ends_at, created_by)
		VALUES ($1, $2, '2026-10-17T22:00:00Z', '2026-10-18T10:00:00Z', $2)`, f.engagementID, f.doulaID)

	w := f.roster(t, db, "2026-10-17", "2026-10-17").Windows[0]
	if !w.Uncovered || len(w.Gaps) != 1 || w.Gaps[0].CoveringStaffID != nil {
		t.Fatalf("an uncovered gap = %+v, want it shown and the window uncovered", w)
	}
	if w.Gaps[0].StaffName != "Maya Primary" {
		t.Errorf("gap staffName = %q, want the Doula who cannot be reached", w.Gaps[0].StaffName)
	}

	backupID := testdb.SeedNamedStaffAtPractice(t, db, f.practiceID, "roster-gap-backup", "Bo Backup", []string{doulaRole}, employeeType)
	testdb.SeedGrantedAttachment(t, db, f.engagementID, backupID)
	exec(t, db, `UPDATE engagement_coverage_gaps SET covering_staff_id = $1`, backupID)
	w = f.roster(t, db, "2026-10-17", "2026-10-17").Windows[0]
	if w.Uncovered || w.Gaps[0].CoveringStaffName == nil || *w.Gaps[0].CoveringStaffName != "Bo Backup" {
		t.Fatalf("a covered gap = %+v, want it shown as covered by Bo Backup", w)
	}

	exec(t, db, `UPDATE engagement_coverage_gaps SET cleared_at = now(), cleared_by = $1`, backupID)
	if w := f.roster(t, db, "2026-10-17", "2026-10-17").Windows[0]; len(w.Gaps) != 0 {
		t.Fatalf("a cleared gap is still on the roster: %+v", w.Gaps)
	}
}

func TestRoster_RefusesAMalformedRange(t *testing.T) {
	db := testdb.New(t)
	f := newSoloFixture(t, db, "roster-range")
	srv, session := newServer(t, db, f.ownerUID)
	defer srv.Close()

	base := srv.URL + "/api/practices/" + f.practiceID + "/on-call"
	for query, want := range map[string]string{
		"?from=tomorrow":                 oncall.MsgInvalidFrom,
		"?from=2026-10-01&to=soon":       oncall.MsgInvalidTo,
		"?from=2026-10-10&to=2026-10-01": oncall.MsgEmptyRange,
		"?from=2026-01-01&to=2026-12-31": oncall.MsgRangeTooLong,
	} {
		resp := authedGet(t, session, base+query)
		body := decode[map[string]any](t, resp, http.StatusBadRequest)
		if body["message"] != want {
			t.Errorf("%s: message = %v, want %q", query, body["message"], want)
		}
	}
}

func TestRoster_DefaultsToTodayInThePracticeZone(t *testing.T) {
	db := testdb.New(t)
	f := newSoloFixture(t, db, "roster-today")
	srv, session := newServer(t, db, f.ownerUID)
	defer srv.Close()

	loc, _ := time.LoadLocation(zone)
	today := time.Now().In(loc).Format("2006-01-02")
	got := decode[oncall.RosterResponse](t, authedGet(t, session, srv.URL+"/api/practices/"+f.practiceID+"/on-call"), http.StatusOK)
	if got.From != today || got.To != today {
		t.Fatalf("range = %s..%s, want today %s", got.From, got.To, today)
	}
}
