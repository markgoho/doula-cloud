package oncall_test

import (
	"net/http"
	"strings"
	"testing"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/oncall"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// TestWrites_RefuseAMalformedPathOrBody walks every on-call write with a
// path segment that is not a uuid and with a body that is not JSON, so
// no handler answers 500 for something a typo can produce.
func TestWrites_RefuseAMalformedPathOrBody(t *testing.T) {
	e := newEnv(t)
	bad := e.PracticeURL("/engagements/not-a-uuid")

	tests := []struct {
		name   string
		method string
		url    string
		body   string
	}{
		{"a gap is recorded with a body that is not JSON", http.MethodPost, e.gapsURL(), "{"},
		{"a gap is edited with a body that is not JSON", http.MethodPut, e.gapsURL() + "/" + e.Birth.ID, "{"},
		{"a rule is set with a body that is not JSON", http.MethodPut, e.ruleURL(), "{"},
		{"a narrowing is set with a body that is not JSON", http.MethodPut, e.narrowURL(e.Doula.ID), "{"},
		{"the settings are set with a body that is not JSON", http.MethodPut, e.settingsURL(), "{"},
		{"a gap is recorded on an Engagement id that is not a uuid", http.MethodPost, bad + "/coverage-gaps", "{}"},
		{"a rule is set on an Engagement id that is not a uuid", http.MethodPut, bad + "/on-call-rule", "{}"},
		{"a narrowing is set on an Engagement id that is not a uuid", http.MethodPut, bad + "/attachments/" + e.Doula.ID + "/on-call", "{}"},
		{"a narrowing is set for a Staff id that is not a uuid", http.MethodPut, e.narrowURL("nope"), "{}"},
		{"a gap is edited with an id that is not a uuid", http.MethodPut, e.gapsURL() + "/nope", "{}"},
		{"a gap is cleared with an id that is not a uuid", http.MethodDelete, e.gapsURL() + "/nope", ""},
		{"the panel is read on an Engagement id that is not a uuid", http.MethodGet, bad + "/on-call", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := []reqtest.Option{e.SignedIn}
			if tt.body != "" {
				opts = append(opts, reqtest.RawJSON([]byte(tt.body)))
			}
			reqtest.ExpectStatus(t, tt.method, tt.url, http.StatusBadRequest, opts...)
		})
	}
}

func TestWrites_RefuseWhatHasNoWindowOrNoEngagement(t *testing.T) {
	e := newEnv(t)
	e.Exec(`UPDATE engagements SET due_date = NULL WHERE id = $1`, e.Birth.ID)
	gapURL := e.gapsURL() + "/" + e.Birth.ID

	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		t.Run(method+" on a birth with no window", func(t *testing.T) {
			reqtest.ExpectStatus(t, method, gapURL, http.StatusConflict, e.SignedIn, reqtest.JSON(gapBody(e.Doula.ID, nil)))
		})
	}

	granted := oncall.StartAttachmentGranted
	unknown := e.PracticeURL("/engagements/00000000-0000-0000-0000-000000000000/on-call-rule")
	reqtest.ExpectStatus(t, http.MethodPut, unknown, http.StatusNotFound, e.SignedIn, reqtest.JSON(oncall.RuleRequest{StartRule: &granted}))
}

func TestEngagementRule_AWeekChangeUnderTheSameRuleIsRecorded(t *testing.T) {
	e := newEnv(t)

	rule := oncall.StartGestationalWeek
	for _, week := range []int{38, 39, 39} {
		e.putRule(oncall.RuleRequest{StartRule: &rule, StartWeek: &week}, http.StatusOK)
	}

	if n := len(e.activityActors("on_call_rule_changed")); n != 2 {
		t.Fatalf("rule entries = %d, want 2 -- the repeat of week 39 records nothing", n)
	}
}

func TestEngagementRule_RefusesEveryMalformedOverride(t *testing.T) {
	e := newEnv(t)
	week, tooEarly := 39, 12
	gestational, unknown := oncall.StartGestationalWeek, oncall.StartRule("whenever")

	tests := []struct {
		name  string
		req   oncall.RuleRequest
		field string
	}{
		{"a week with no rule to apply it to", oncall.RuleRequest{StartWeek: &week}, fieldWeek},
		{"a week outside the bounds", oncall.RuleRequest{StartRule: &gestational, StartWeek: &tooEarly}, fieldWeek},
		{"a gestational rule with no week", oncall.RuleRequest{StartRule: &gestational}, fieldWeek},
		{"a rule name the product does not do", oncall.RuleRequest{StartRule: &unknown}, "startRule"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reqtest.Expect[refusal](t, http.MethodPut, e.ruleURL(), http.StatusBadRequest, e.SignedIn, reqtest.JSON(tt.req))
			if got.Details[tt.field] == nil {
				t.Fatalf("details = %v, want a refusal under %q", got.Details, tt.field)
			}
		})
	}
}

// TestUpdateGap_RefusesTheSameThingsACreateDoes: the edit is a full
// replace, so it is held to the same rules, including who may be named.
func TestUpdateGap_RefusesTheSameThingsACreateDoes(t *testing.T) {
	e := newGapEnv(t)
	noEnd := gapBody(e.Doula.ID, nil)
	noEnd.EndsAt = nil

	got := reqtest.Expect[refusal](t, http.MethodPut, e.gapURL(), http.StatusBadRequest, e.SignedIn, reqtest.JSON(noEnd))
	if got.Details[fieldEndsAt] != oncall.MsgEndsAtRequired {
		t.Fatalf("details = %v, want the missing end refused", got.Details)
	}

	// The gap's own Doula may edit it, and may not hand it to a colleague.
	asDoula := e.As(e.Doula)
	e.save(asDoula, gapBody(e.Doula.ID, &e.Backup.ID))
	reqtest.ExpectStatus(t, http.MethodPut, e.gapURL(), http.StatusForbidden, asDoula, reqtest.JSON(gapBody(e.Backup.ID, nil)))
}

// TestPanel_ShowsTheGapsOnTheBirth: the Engagement's own panel carries
// the live gaps, so a Doula can see her cover from the birth's page.
func TestPanel_ShowsTheGapsOnTheBirth(t *testing.T) {
	e := newGapEnv(t)

	if panel := e.panel(); len(panel.Gaps) != 1 || panel.Gaps[0].ID != e.GapID {
		t.Fatalf("panel gaps = %+v, want the one saved gap", panel.Gaps)
	}
}

func TestCreateGap_RefusesALongReasonAndTakesABlankCover(t *testing.T) {
	e := newEnv(t)

	long := strings.Repeat("a", 501)
	tooLong := gapBody(e.Doula.ID, nil)
	tooLong.Reason = &long
	reqtest.ExpectStatus(t, http.MethodPost, e.gapsURL(), http.StatusBadRequest, e.SignedIn, reqtest.JSON(tooLong))

	blank := ""
	blankCover := gapBody(e.Doula.ID, &blank)
	blankCover.Reason = nil
	gap := e.recordGap(e.SignedIn, blankCover)
	if gap.CoveringStaffID != nil || gap.Reason != nil {
		t.Fatalf("gap = %+v, want a blank cover and reason read as none", gap)
	}
}

// departedCoverEnv is the env plus a backup Doula covering one gap,
// before she leaves -- the setup both departure tests share, each then
// ending her Membership its own way.
func newDepartedCoverEnv(t *testing.T) (*env, string) {
	t.Helper()
	e := newEnv(t)
	backup := e.backup()
	e.Exec(`INSERT INTO engagement_coverage_gaps (engagement_id, staff_id, covering_staff_id, starts_at, ends_at, created_by)
		VALUES ($1, $2, $3, '2026-10-17T22:00:00Z', '2026-10-18T10:00:00Z', $2)`, e.Birth.ID, e.Doula.ID, backup.ID)
	return e, backup.ID
}

// TestRoster_ACoveringColleagueWhoLeftIsStillNamed: a Doula whose
// Membership has ended is still a row this Practice can read, through
// the 'removed' event her removal wrote (00116, #1455), so the roster
// keeps her name.
func TestRoster_ACoveringColleagueWhoLeftIsStillNamed(t *testing.T) {
	e, backupID := newDepartedCoverEnv(t)
	testdb.EndMembership(t, e.DB, e.Practice.ID, backupID, e.Staff.ID)

	w := e.roster("2026-10-17", "2026-10-17").Windows[0]

	if w.Gaps[0].CoveringStaffName == nil || *w.Gaps[0].CoveringStaffName != backupName {
		t.Fatalf("covering name = %v, want %q", w.Gaps[0].CoveringStaffName, backupName)
	}
}

// TestRoster_ADepartedCoveringColleagueIsNamedAsOne: a Doula who deleted
// her own login (ADR-0033) is no longer a row this Practice can read --
// 00116 refuses a redacted row -- so the roster says so in words rather
// than printing a bare id.
func TestRoster_ADepartedCoveringColleagueIsNamedAsOne(t *testing.T) {
	e, backupID := newDepartedCoverEnv(t)
	testdb.EndMembership(t, e.DB, e.Practice.ID, backupID, backupID)
	testdb.RedactDeletedLogin(t, e.DB, backupID)

	w := e.roster("2026-10-17", "2026-10-17").Windows[0]

	if w.Gaps[0].CoveringStaffName == nil || *w.Gaps[0].CoveringStaffName != "a former colleague" {
		t.Fatalf("covering name = %v, want the words for a colleague this Practice can no longer read", w.Gaps[0].CoveringStaffName)
	}
}

// TestRoster_SortsAndSkipsWhatItShould covers the two orderings the
// screen depends on and a narrowing that misses the window entirely.
func TestRoster_SortsAndSkipsWhatItShould(t *testing.T) {
	e := newEnv(t)
	e.attachNewBirth("Zeb Nakamura", e.Doula)
	e.attachNewBirth("Ada Second", e.Doula)
	// A narrowing that misses the window: she is attached and on call for
	// none of it, so the whole window reads as unstaffed.
	e.Exec(`UPDATE engagement_attachments SET on_call_from = '2027-01-01' WHERE engagement_id = $1`, e.Birth.ID)

	got := e.roster(windowStart, octLast)

	if len(got.NoWindow) != 2 || got.NoWindow[0].ClientName != "Ada Second" {
		t.Fatalf("noWindow = %+v, want two rows, Ada first", got.NoWindow)
	}
	w := got.Windows[0]
	if len(w.OnCall) != 0 || !w.Uncovered || len(w.UnstaffedDays) != 1 {
		t.Fatalf("window = %+v, want nobody on call and the whole range unstaffed", w)
	}
}

// TestGapNoticeWorker_NoOwnerOrAdminLeftMailsNobody: the roster moves,
// and a Practice with nobody left to tell is a row addressed to nobody
// rather than a row that never resolves.
func TestGapNoticeWorker_NoOwnerOrAdminLeftMailsNobody(t *testing.T) {
	e := newNoticeEnv(t)
	e.Exec(`DELETE FROM practice_memberships WHERE practice_id = $1 AND (roles && ARRAY['owner','admin']::practice_role[])`, e.Practice.ID)

	e.runWorker(&mail.FakeSender{})

	if got := e.noticeStatus(); got != statusSent {
		t.Fatalf("status = %q, want sent", got)
	}
	if n := e.count(`SELECT count(*) FROM coverage_gap_outbox WHERE gap_id = $1 AND notified_staff_ids = '{}'`, e.GapID); n != 1 {
		t.Fatal("the row does not record that it addressed nobody")
	}
}
