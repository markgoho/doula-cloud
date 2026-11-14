package oncall_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/oncall"
	"doula-cloud/api/internal/reqtest"
)

const (
	octFifth       = "2026-10-05"
	octEighth      = "2026-10-08"
	octNinth       = "2026-10-09"
	octTwelfthDay  = "2026-10-12"
	octSeventeen   = "2026-10-17"
	octTwentyEight = "2026-10-28"
	tonightDue     = "2026-10-22"
	novThird       = "2026-11-03"
)

// birthDue is another active birth for a Client named name, due on due
// (none when empty), with who on it.
func (e *env) birthDue(name, due string, who ...fixture.StaffRecord) fixture.EngagementRecord {
	e.T.Helper()
	opts := []fixture.EngagementOption{fixture.NewClient(fixture.ClientName(name)), fixture.Status(fixture.StatusActive)}
	if due != "" {
		opts = append(opts, fixture.DueDate(due))
	}
	for _, d := range who {
		opts = append(opts, fixture.WithDoula(d))
	}
	return e.AnotherEngagement(opts...)
}

// gapOn records a live gap for staffID on engagementID, covered by
// covering when that is not empty.
func (e *env) gapOn(engagementID, staffID, startsAt, endsAt, covering string) {
	e.T.Helper()
	var cover any
	if covering != "" {
		cover = covering
	}
	e.Exec(`INSERT INTO engagement_coverage_gaps (engagement_id, staff_id, starts_at, ends_at, covering_staff_id, reason, created_by)
		VALUES ($1, $2, $3, $4, $5, 'Sister''s wedding', $2)`, engagementID, staffID, startsAt, endsAt, cover)
}

func rowOf(t *testing.T, resp oncall.RosterResponse, id string) oncall.RosterWindow {
	t.Helper()
	for _, w := range resp.Windows {
		if w.EngagementID == id {
			return w
		}
	}
	t.Fatalf("no window for %s among %v", id, windowIDs(resp))
	return oncall.RosterWindow{}
}

func doulaOf(t *testing.T, resp oncall.RosterResponse, id string) oncall.RosterDoula {
	t.Helper()
	for _, d := range resp.Doulas {
		if d.StaffID == id {
			return d
		}
	}
	t.Fatalf("no Doula %s among the roster's Doulas", id)
	return oncall.RosterDoula{}
}

func orderOf(resp oncall.RosterResponse) []string {
	ids := []string{}
	for _, w := range resp.Windows {
		ids = append(ids, w.EngagementID)
	}
	return ids
}

func TestRoster_SaysTodayAndTheRuleInForce(t *testing.T) {
	e := newEnv(t)

	got := e.roster(octFirst, octTwenty)

	if got.Today != octFirst {
		t.Errorf("today = %s, want the Practice's day %s", got.Today, octFirst)
	}
	if got.Rule != (oncall.Settings{StartRule: oncall.StartGestationalWeek, StartWeek: 37, GraceDays: 14}) {
		t.Errorf("rule = %+v, want the Practice's 37 weeks and 14 days", got.Rule)
	}
}

func TestRoster_TodayFollowsThePracticeZoneNotUTC(t *testing.T) {
	e := newEnv(t)
	e.Exec(`UPDATE practices SET timezone = 'Pacific/Auckland' WHERE id = $1`, e.Practice.ID)

	got := e.rosterQuery("")

	if got.Today != "2026-10-02" || got.From != "2026-10-02" {
		t.Errorf("today = %s, from = %s, want 2026-10-02 where it is already the next day", got.Today, got.From)
	}
}

func TestRoster_CarriesNoGapReason(t *testing.T) {
	e := newEnv(t)
	e.gapOn(e.Birth.ID, e.Doula.ID, "2026-10-17T22:00:00Z", "2026-10-18T10:00:00Z", "")

	body := string(reqtest.ExpectBody(t, http.MethodGet, e.rosterURL(octSeventeen, octSeventeen), http.StatusOK, e.SignedIn))

	if strings.Contains(body, "wedding") || strings.Contains(body, `"reason"`) {
		t.Errorf("the roster carries a gap's reason: %s", body)
	}
	if panel := e.panel(); len(panel.Gaps) != 1 || panel.Gaps[0].Reason == nil || *panel.Gaps[0].Reason != "Sister's wedding" {
		t.Errorf("panel gaps = %+v, want the Engagement panel to keep the reason", panel.Gaps)
	}
}

func TestRoster_NamesABirthsOwnStartWhereItHasOne(t *testing.T) {
	e := newEnv(t)
	other := e.birthDue("Own Granted", "2026-10-28", e.Doula)
	e.Exec(`UPDATE engagements SET on_call_start_rule = 'gestational_week', on_call_start_week = 38 WHERE id = $1`, e.Birth.ID)
	e.Exec(`UPDATE engagements SET on_call_start_rule = 'attachment_granted' WHERE id = $1`, other.ID)
	e.Exec(`UPDATE engagement_attachments SET attached_at = '2026-10-03T15:00:00Z' WHERE engagement_id = $1`, other.ID)
	third := e.birthDue("Follows Rule", "2026-10-28", e.Doula)

	got := e.roster(octFirst, octLast)

	own := rowOf(t, got, e.Birth.ID).OwnStart
	if own == nil || own.StartRule != oncall.StartGestationalWeek || own.StartWeek == nil || *own.StartWeek != 38 {
		t.Errorf("ownStart = %+v, want gestational week 38", own)
	}
	granted := rowOf(t, got, other.ID)
	if granted.OwnStart == nil || granted.OwnStart.StartRule != oncall.StartAttachmentGranted || granted.OwnStart.StartWeek != nil {
		t.Errorf("ownStart = %+v, want the grant-day rule with no week", granted.OwnStart)
	}
	if granted.Window.Start != "2026-10-03" {
		t.Errorf("window starts %s, want the grant day", granted.Window.Start)
	}
	if rowOf(t, got, third.ID).OwnStart != nil {
		t.Error("a birth that follows the Practice's rule carries an ownStart")
	}
}

func TestRoster_ABirthWithNoDoulaIsListedOverTheDaysItsRuleWouldGive(t *testing.T) {
	e := newEnv(t)
	orphan := e.birthDue("Orla Unstaffed", "2026-10-20")

	got := e.roster(octFirst, octLast)

	row := rowOf(t, got, orphan.ID)
	if !row.NoDoula || len(row.OnCall) != 0 || !row.Uncovered {
		t.Errorf("row = %+v, want no Doula, nobody on call, and needing cover", row)
	}
	if row.Window != (oncall.Window{Start: "2026-09-29", End: novThird}) {
		t.Errorf("window = %+v, want the week rule's 2026-09-29..2026-11-03", row.Window)
	}
	if len(row.UnstaffedDays) != 1 || row.UnstaffedDays[0] != (oncall.Window{Start: octFirst, End: novThird}) {
		t.Errorf("unstaffed days = %+v, want the whole run, from today to the birth's last day", row.UnstaffedDays)
	}
	if row.Tonight == nil || !row.Tonight.IsNobody || len(row.Tonight.Holders) != 0 {
		t.Errorf("tonight = %+v, want nobody holding it", row.Tonight)
	}
	if rowOf(t, got, e.Birth.ID).NoDoula {
		t.Error("a birth with a Doula reads as having none")
	}

	e.Exec(`UPDATE practices SET on_call_start_rule = 'attachment_granted' WHERE id = $1`, e.Practice.ID)
	if got := rowOf(t, e.roster(octFirst, octLast), orphan.ID); got.Window != (oncall.Window{Start: octFirst, End: novThird}) {
		t.Errorf("under the grant-day rule window = %+v, want it to open today and end at the due date plus grace", got.Window)
	}
}

func TestRoster_ABirthWithNoDoulaAndNoDueDateSaysSo(t *testing.T) {
	e := newEnv(t)
	orphan := e.birthDue("Orla Unstaffed", "")
	e.attachNewBirth("Zed Attached", e.Doula)
	e.birthDue("Post Partum", "2026-10-20")
	e.AnotherEngagement(fixture.NewClient(fixture.ClientName("Pia Postpartum")), fixture.Kind("postpartum"), fixture.Status(fixture.StatusActive))
	e.AnotherEngagement(fixture.NewClient(fixture.ClientName("Cora Completed")), fixture.Status(fixture.StatusCompleted), fixture.DueDate("2026-10-20"))

	got := e.roster(octFirst, octLast)

	var reasons = map[string]oncall.NoWindowRow{}
	for _, n := range got.NoWindow {
		reasons[n.ClientName] = n
	}
	if len(got.NoWindow) != 2 {
		t.Fatalf("noWindow = %+v, want the two births with no due date only", got.NoWindow)
	}
	none := reasons["Orla Unstaffed"]
	if none.EngagementID != orphan.ID || none.Reason != oncall.NoWindowNoDueDateNoDoula || len(none.Doulas) != 0 {
		t.Errorf("no-Doula row = %+v, want reason no_due_date_no_doula and nobody on it", none)
	}
	attached := reasons["Zed Attached"]
	if attached.Reason != oncall.NoWindowNoDueDate || len(attached.Doulas) != 1 || attached.Doulas[0].StaffID != e.Doula.ID || attached.Doulas[0].Name != e.Doula.Name {
		t.Errorf("attached row = %+v, want reason no_due_date and the Doula on it", attached)
	}
}

func TestRoster_CoverNeededKeepsOnlyTheBirthsNeedingCover(t *testing.T) {
	e := newEnv(t)
	orphan := e.birthDue("Orla Unstaffed", "2026-10-20")
	holed := e.birthDue("Holly Holed", tonightDue, e.Doula)
	e.Exec(`UPDATE engagement_attachments SET on_call_from = '2026-10-10' WHERE engagement_id = $1`, holed.ID)
	e.attachNewBirth("Zed Attached", e.Doula)

	all := e.roster(octFirst, octLast)
	got := e.rosterQuery("?from=" + octFirst + "&to=" + octLast + "&cover=needed")

	if want := sorted(orphan.ID, holed.ID); !slices.Equal(windowIDs(got), want) {
		t.Errorf("windows = %v, want %v", windowIDs(got), want)
	}
	if len(got.NoWindow) != 0 || len(all.NoWindow) != 1 {
		t.Errorf("noWindow = %d under cover=needed and %d without, want 0 and 1", len(got.NoWindow), len(all.NoWindow))
	}
	if len(all.Windows) != 3 {
		t.Errorf("without the narrowing windows = %v, want all three", windowIDs(all))
	}
}

func TestRoster_StaffIDKeepsTheBirthsADoulaIsOnCallForOrCoversAndNoFlagMoves(t *testing.T) {
	e := newEnv(t)
	backup := e.backup()
	e.Exec(`UPDATE engagement_attachments SET on_call_from = '2026-10-25' WHERE staff_id = $1`, backup.ID)
	stranger := e.stranger()
	elsewhere := e.birthDue("Elle Elsewhere", octTwentyEight, stranger)
	e.gapOn(e.Birth.ID, e.Doula.ID, "2026-10-17T22:00:00Z", "2026-10-18T10:00:00Z", backup.ID)
	e.attachNewBirth("Zed Attached", e.Doula)
	query := "?from=2026-10-10&to=2026-10-20"
	whole := e.rosterQuery(query)
	if n := len(e.rosterQuery(query + "&staffId=" + e.Doula.ID).NoWindow); n != 1 {
		t.Errorf("noWindow for the Doula on it = %d, want 1", n)
	}
	if n := len(e.rosterQuery(query + "&staffId=" + stranger.ID).NoWindow); n != 0 {
		t.Errorf("noWindow for a Doula who is not on it = %d, want 0", n)
	}

	for name, tt := range map[string]struct {
		who  string
		want []string
	}{
		"the Doula on call":           {e.Doula.ID, []string{e.Birth.ID}},
		"a Doula only covering a gap": {backup.ID, []string{e.Birth.ID}},
		"a Doula on another birth":    {stranger.ID, []string{elsewhere.ID}},
		"a Doula who is on nothing":   {"00000000-0000-0000-0000-000000000000", nil},
	} {
		t.Run(name, func(t *testing.T) {
			got := e.rosterQuery(query + "&staffId=" + tt.who)

			if !slices.Equal(windowIDs(got), sorted(tt.want...)) {
				t.Errorf("windows = %v, want %v", windowIDs(got), tt.want)
			}
			if len(got.Doulas) != len(whole.Doulas) || len(doulaOf(t, got, e.Doula.ID).Births) != len(doulaOf(t, whole, e.Doula.ID).Births) {
				t.Error("staffId changed the Doula list, which is computed over the whole answer")
			}
		})
	}
}

func TestRoster_OrdersTonightFirstThenDueDateAndNeedingCoverFirstOnRequest(t *testing.T) {
	e := newEnv(t)
	tonight := e.birthDue("Tessa Tonight", tonightDue, e.Doula)
	later := e.birthDue("Lara Later", octFifth, e.Doula)
	e.Exec(`UPDATE engagements SET on_call_start_rule = 'gestational_week', on_call_start_week = 40 WHERE id = $1`, later.ID)
	e.Exec(`UPDATE engagement_attachments SET on_call_from = '2026-10-10' WHERE engagement_id = $1`, later.ID)
	query := "?from=" + octFirst + "&to=" + octLast

	byDefault := e.rosterQuery(query)
	byCover := e.rosterQuery(query + "&order=cover")

	if want := []string{tonight.ID, later.ID, e.Birth.ID}; !slices.Equal(orderOf(byDefault), want) {
		t.Errorf("default order = %v, want tonight first then due date %v", orderOf(byDefault), want)
	}
	if want := []string{later.ID, tonight.ID, e.Birth.ID}; !slices.Equal(orderOf(byCover), want) {
		t.Errorf("order=cover = %v, want the one needing cover first %v", orderOf(byCover), want)
	}
}

func TestRoster_ABirthWithNobodyTonightComesBeforeTheOtherBirthsOfTonight(t *testing.T) {
	e := newEnv(t)
	tonight := e.birthDue("Tessa Tonight", tonightDue, e.Doula)
	nobody := e.birthDue("Zed Nobody", tonightDue, e.Doula)
	e.Exec(`UPDATE engagement_attachments SET on_call_from = '2026-10-10' WHERE engagement_id = $1`, nobody.ID)
	later := e.birthDue("Lara Later", octFifth, e.Doula)
	e.Exec(`UPDATE engagements SET on_call_start_rule = 'gestational_week', on_call_start_week = 40 WHERE id = $1`, later.ID)
	e.Exec(`UPDATE engagement_attachments SET on_call_from = '2026-10-10' WHERE engagement_id = $1`, later.ID)
	query := "?from=" + octFirst + "&to=" + octLast

	got := e.rosterQuery(query)

	if want := []string{nobody.ID, tonight.ID, later.ID, e.Birth.ID}; !slices.Equal(orderOf(got), want) {
		t.Errorf("default order = %v, want the birth with nobody tonight first %v", orderOf(got), want)
	}
	if want := []string{nobody.ID, later.ID, tonight.ID, e.Birth.ID}; !slices.Equal(orderOf(e.rosterQuery(query+"&order=cover")), want) {
		t.Errorf("order=cover = %v, want %v", orderOf(e.rosterQuery(query+"&order=cover")), want)
	}
}

func TestRoster_TheSameExtentsReadTheSameUnderEveryRange(t *testing.T) {
	e := newEnv(t)
	nobody := e.birthDue("Zed Nobody", tonightDue, e.Doula)
	e.Exec(`UPDATE engagement_attachments SET on_call_from = '2026-10-10' WHERE engagement_id = $1`, nobody.ID)
	second := e.birthDue("Quinn Second", octTwentyEight, e.Doula)

	today, wide := e.roster(octFirst, octFirst), e.roster(octFirst, octLast)
	if a, b := rowOf(t, today, nobody.ID).UnstaffedDays, rowOf(t, wide, nobody.ID).UnstaffedDays; !slices.Equal(a, b) || !slices.Equal(a, []oncall.Window{{Start: octFirst, End: "2026-10-09"}}) {
		t.Errorf("unstaffed days = %+v under one day and %+v under the month, want the whole run from today to the 9th under both", a, b)
	}

	oneDay := e.roster(octTwelfthDay, octTwelfthDay)
	a, b := doulaOf(t, oneDay, e.Doula.ID).AtOnce, doulaOf(t, wide, e.Doula.ID).AtOnce
	if len(a) == 0 || !slices.EqualFunc(a, b, func(x, y oncall.DoulaAtOnce) bool {
		return x.Days == y.Days && slices.Equal(x.EngagementIDs, y.EngagementIDs)
	}) {
		t.Errorf("the Doula's atOnce = %+v under one day and %+v under the month, want the same shared days", a, b)
	}
	for _, id := range []string{nobody.ID, second.ID, e.Birth.ID} {
		if x, y := rowOf(t, oneDay, id).OnCall[0].AtOnce, rowOf(t, wide, id).OnCall[0].AtOnce; !slices.Equal(x, y) {
			t.Errorf("a birth's atOnce differs by range: %+v against %+v", x, y)
		}
	}
}

func TestRoster_TonightNamesWhoHoldsTheBirth(t *testing.T) {
	e := newEnv(t, dueOn(octEighth))
	backup := e.backup()

	t.Run("a Doula on call with no gap holds it", func(t *testing.T) {
		row := rowOf(t, e.rosterQuery("?to="+octFirst), e.Birth.ID)
		if row.Tonight == nil || row.Tonight.IsNobody || len(row.Tonight.Holders) != 2 || row.Tonight.Holders[0].StaffID != e.Doula.ID || row.Tonight.Holders[0].CoveringFor != nil {
			t.Errorf("tonight = %+v, want both Doulas holding it", row.Tonight)
		}
	})

	t.Run("a covered gap tonight puts the covering Doula in her place", func(t *testing.T) {
		e.gapOn(e.Birth.ID, e.Doula.ID, "2026-10-01T22:00:00Z", "2026-10-02T10:00:00Z", backup.ID)
		got := e.rosterQuery("")
		row := rowOf(t, got, e.Birth.ID)
		if row.Tonight == nil || row.Tonight.IsNobody || len(row.Tonight.Holders) != 1 {
			t.Fatalf("tonight = %+v, want the covering Doula alone", row.Tonight)
		}
		holder := row.Tonight.Holders[0]
		if holder.StaffID != backup.ID || holder.CoveringFor == nil || *holder.CoveringFor != e.Doula.Name {
			t.Errorf("holder = %+v, want %s covering for %s", holder, backupName, e.Doula.Name)
		}
		if !doulaOf(t, got, backup.ID).IsOnCallTonight || doulaOf(t, got, e.Doula.ID).IsOnCallTonight {
			t.Error("isOnCallTonight does not follow who holds the birth")
		}
	})

	t.Run("an uncovered gap tonight leaves a birth with one Doula held by nobody", func(t *testing.T) {
		e.Exec(`UPDATE engagement_attachments SET ended_at = now(), ended_by = staff_id, ended_reason = 'taken_off' WHERE staff_id = $1`, backup.ID)
		e.Exec(`UPDATE engagement_coverage_gaps SET covering_staff_id = NULL`)
		row := rowOf(t, e.rosterQuery(""), e.Birth.ID)
		if row.Tonight == nil || !row.Tonight.IsNobody || len(row.Tonight.Holders) != 0 || !row.Uncovered {
			t.Errorf("tonight = %+v uncovered = %v, want nobody and needing cover", row.Tonight, row.Uncovered)
		}
	})

	t.Run("a narrowing that starts later leaves nobody tonight", func(t *testing.T) {
		e.Exec(`UPDATE engagement_coverage_gaps SET cleared_at = now(), cleared_by = $1`, e.Staff.ID)
		e.Exec(`UPDATE engagement_attachments SET on_call_from = '2026-10-10' WHERE staff_id = $1`, e.Doula.ID)
		row := rowOf(t, e.rosterQuery(""), e.Birth.ID)
		if row.Tonight == nil || !row.Tonight.IsNobody {
			t.Errorf("tonight = %+v, want nobody", row.Tonight)
		}
	})

	t.Run("a range that starts after today has no tonight", func(t *testing.T) {
		row := rowOf(t, e.rosterQuery("?from=2026-10-02&to=2026-10-05"), e.Birth.ID)
		if row.Tonight != nil {
			t.Errorf("tonight = %+v, want none", row.Tonight)
		}
	})
}

func TestRoster_ACoveringDoulaWhoIsAlsoOnCallIsNamedAsCoveringWhateverTheOrder(t *testing.T) {
	e := newEnv(t, dueOn(octEighth))
	backup := e.backup()
	e.Exec(`UPDATE engagement_attachments SET attached_at = attached_at - interval '1 day' WHERE staff_id = $1`, backup.ID)
	e.gapOn(e.Birth.ID, e.Doula.ID, "2026-10-01T22:00:00Z", "2026-10-02T10:00:00Z", backup.ID)

	row := rowOf(t, e.rosterQuery(""), e.Birth.ID)

	if row.Tonight == nil || len(row.Tonight.Holders) != 1 || row.Tonight.Holders[0].StaffID != backup.ID || row.Tonight.Holders[0].CoveringFor == nil {
		t.Errorf("tonight = %+v, want the Doula named once, as covering", row.Tonight)
	}
}

func TestRoster_ABirthWithNoDueDateSortsAfterOnesWithOne(t *testing.T) {
	e := newEnv(t)
	e.Exec(`UPDATE practices SET on_call_start_rule = 'attachment_granted' WHERE id = $1`, e.Practice.ID)
	for _, name := range []string{"Zoe Ended", "Amy Ended"} {
		ended := e.birthDue(name, "", e.Doula)
		e.Exec(`UPDATE engagements SET birth_outcome = 'live_birth', pregnancy_ended_on = '2026-10-15' WHERE id = $1`, ended.ID)
		e.Exec(`UPDATE engagement_attachments SET attached_at = '2026-10-03T15:00:00Z' WHERE engagement_id = $1`, ended.ID)
	}
	e.Exec(`UPDATE engagement_attachments SET attached_at = '2026-10-03T15:00:00Z' WHERE engagement_id = $1`, e.Birth.ID)

	got := e.roster(octFirst, octLast)

	names := []string{}
	for _, w := range got.Windows {
		names = append(names, w.ClientName)
	}
	if want := []string{adaWhitfield, "Amy Ended", "Zoe Ended"}; !slices.Equal(names, want) {
		t.Errorf("order = %v, want the dated birth first, then the two with none by name", names)
	}
}

func TestRoster_ABirthWhoseDaysMissTodayHasNoTonight(t *testing.T) {
	e := newEnv(t)

	row := rowOf(t, e.roster(octFirst, octLast), e.Birth.ID)

	if row.Tonight != nil {
		t.Errorf("tonight = %+v, want none for a birth whose days start on the 9th", row.Tonight)
	}
}

func TestRoster_AGapThatEndedEarlierTodayIsNeitherListedNorCounted(t *testing.T) {
	e := newEnv(t, dueOn(octEighth))
	e.gapOn(e.Birth.ID, e.Doula.ID, "2026-10-01T10:00:00Z", "2026-10-01T14:00:00Z", "")

	got := e.rosterQuery("")

	row := rowOf(t, got, e.Birth.ID)
	if len(row.Gaps) != 0 || row.Uncovered {
		t.Errorf("row = %+v, want the finished gap neither listed nor making the birth need cover", row)
	}
	if len(doulaOf(t, got, e.Doula.ID).Unreachable) != 0 {
		t.Error("the Doula reads as unreachable for a gap that has ended")
	}

	e.gapOn(e.Birth.ID, e.Doula.ID, "2026-10-01T15:00:00Z", "2026-10-01T20:00:00Z", "")
	got = e.rosterQuery("")
	if row := rowOf(t, got, e.Birth.ID); len(row.Gaps) != 1 || !row.Uncovered {
		t.Errorf("row = %+v, want the gap in force now listed", row)
	}
	if unreachable := doulaOf(t, got, e.Doula.ID).Unreachable; len(unreachable) != 1 || unreachable[0].IsCovered || unreachable[0].ClientName != adaWhitfield || unreachable[0].EngagementID != e.Birth.ID {
		t.Errorf("unreachable = %+v, want the one gap, uncovered, naming its Client", unreachable)
	}
}

func TestRoster_AGapOutsideTheBirthsOwnDaysIsNeitherListedNorCountedButIsKept(t *testing.T) {
	e := newEnv(t)
	e.gapOn(e.Birth.ID, e.Doula.ID, "2026-10-17T22:00:00Z", "2026-10-18T10:00:00Z", "")
	e.Exec(`UPDATE engagements SET due_date = '2026-11-20' WHERE id = $1`, e.Birth.ID)

	got := e.roster(octFirst, "2026-12-31")

	row := rowOf(t, got, e.Birth.ID)
	if len(row.Gaps) != 0 || row.Uncovered {
		t.Errorf("row = %+v, want the gap left outside the days neither listed nor counted", row)
	}
	if len(doulaOf(t, got, e.Doula.ID).Unreachable) != 0 {
		t.Error("the Doula reads as unreachable for a gap outside the birth's days")
	}
	if n := e.count(`SELECT count(*) FROM engagement_coverage_gaps WHERE engagement_id = $1 AND cleared_at IS NULL`, e.Birth.ID); n != 1 {
		t.Errorf("live gaps = %d, want it kept, not cleared", n)
	}
}

func TestRoster_FlagsADoulaOnTwoBirthsOnlyWhereHerDaysMeetInsideTheRange(t *testing.T) {
	e := newEnv(t)
	second := e.birthDue("Quinn Second", octTwentyEight, e.Doula)
	query := "?from=" + octEighth + "&to=" + octTwelfthDay

	got := e.rosterQuery(query)

	if want := []string{second.ID, e.Birth.ID}; !slices.Equal(orderOf(got), want) {
		t.Fatalf("order = %v, want %v", orderOf(got), want)
	}
	shared := oncall.Window{Start: octNinth, End: "2026-11-11"}
	first := rowOf(t, got, second.ID).OnCall[0].AtOnce
	if len(first) != 1 || first[0].EngagementID != e.Birth.ID || first[0].ClientName != adaWhitfield || first[0].Days != shared {
		t.Errorf("on the second birth atOnce = %+v, want the first birth over %+v", first, shared)
	}
	again := rowOf(t, got, e.Birth.ID).OnCall[0].AtOnce
	if len(again) != 1 || again[0].EngagementID != second.ID || again[0].Days != shared {
		t.Errorf("on the first birth atOnce = %+v, want the second over %+v", again, shared)
	}
	pairs := doulaOf(t, got, e.Doula.ID).AtOnce
	if len(pairs) != 1 || pairs[0].Days != shared || !slices.Equal(pairs[0].EngagementIDs, []string{second.ID, e.Birth.ID}) {
		t.Errorf("the Doula's atOnce = %+v, want the pair over %+v", pairs, shared)
	}

	e.Exec(`UPDATE engagement_attachments SET on_call_to = '2026-10-08' WHERE engagement_id = $1`, second.ID)
	got = e.rosterQuery(query)
	if len(rowOf(t, got, second.ID).OnCall[0].AtOnce) != 0 || len(rowOf(t, got, e.Birth.ID).OnCall[0].AtOnce) != 0 || len(doulaOf(t, got, e.Doula.ID).AtOnce) != 0 {
		t.Error("two births whose days do not meet are flagged as at once")
	}
}

func TestRoster_TheDoulaListSaysWhatEachDoulaDoesInTheRange(t *testing.T) {
	e := newEnv(t)
	backup := e.backup()
	e.Exec(`UPDATE engagement_attachments SET on_call_from = '2026-10-25' WHERE staff_id = $1`, backup.ID)
	idle := e.stranger()
	e.gapOn(e.Birth.ID, e.Doula.ID, "2026-10-17T22:00:00Z", "2026-10-18T10:00:00Z", backup.ID)
	e.gapOn(e.Birth.ID, e.Doula.ID, "2026-10-12T22:00:00Z", "2026-10-13T10:00:00Z", "")

	got := e.roster("2026-10-10", "2026-10-20")

	primary := doulaOf(t, got, e.Doula.ID)
	if len(primary.Unreachable) != 2 || primary.Unreachable[0].IsCovered || !primary.Unreachable[1].IsCovered {
		t.Errorf("primary unreachable = %+v, want her two gaps soonest first", primary.Unreachable)
	}
	primary.Unreachable = primary.Unreachable[1:]
	if len(primary.Births) != 1 || primary.Births[0] != (oncall.DoulaBirth{EngagementID: e.Birth.ID, ClientName: adaWhitfield}) {
		t.Errorf("primary births = %+v, want the one she is on call for", primary.Births)
	}
	if len(primary.Unreachable) != 1 || !primary.Unreachable[0].IsCovered || primary.Unreachable[0].GapID == "" {
		t.Errorf("primary unreachable = %+v, want the one covered gap", primary.Unreachable)
	}
	covering := doulaOf(t, got, backup.ID)
	if len(covering.Births) != 1 || !covering.Births[0].Covering || covering.Births[0].EngagementID != e.Birth.ID {
		t.Errorf("covering births = %+v, want the birth she covers a gap on, marked covering", covering.Births)
	}
	if none := doulaOf(t, got, idle.ID); none.Births == nil || len(none.Births) != 0 || none.Unreachable == nil || none.IsOnCallTonight {
		t.Errorf("idle Doula = %+v, want empty lists and not on call tonight", none)
	}
}

func TestRoster_AnEmployeeAndAnAdminReadTheBirthsWithNoDoulaToo(t *testing.T) {
	e := newRosterEnv(t)
	orphan := e.birthDue("Orla Unstaffed", "2026-10-20")

	for name, reader := range map[string]fixture.StaffRecord{
		"an employee Doula": e.Employee,
		"an Admin":          fixture.Staff(t, e.DB, e.Practice, fixture.Admin()),
	} {
		t.Run(name, func(t *testing.T) {
			got := reqtest.Expect[oncall.RosterResponse](t, http.MethodGet, e.rosterURL(octFirst, octLast), http.StatusOK, e.As(reader))
			if !slices.Contains(windowIDs(got), orphan.ID) {
				t.Errorf("windows = %v, want the birth with no Doula", windowIDs(got))
			}
		})
	}
}
