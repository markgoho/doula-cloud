package oncall_test

import (
	"net/http"
	"strings"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/oncall"
	"doula-cloud/api/internal/testdb"
)

// postRaw sends a body this package's DTOs cannot produce -- malformed
// JSON -- which decode helpers refuse before any handler logic runs.
func postRaw(t *testing.T, session, method, url, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", t.Name()+method+url)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// TestWrites_RefuseAMalformedPathOrBody walks every on-call write with a
// path segment that is not a uuid and with a body that is not JSON, so
// no handler answers 500 for something a typo can produce.
func TestWrites_RefuseAMalformedPathOrBody(t *testing.T) {
	db := testdb.New(t)
	f := newSoloFixture(t, db, "refuse-shape")
	srv, session := newServer(t, db, f.ownerUID)
	defer srv.Close()

	base := srv.URL + "/api/practices/" + f.practiceID
	engagement := base + "/engagements/" + f.engagementID
	bad := base + "/engagements/not-a-uuid"

	malformed := map[string]string{
		http.MethodPost + " " + engagement + "/coverage-gaps":                        "{",
		http.MethodPut + " " + engagement + "/coverage-gaps/" + f.engagementID:       "{",
		http.MethodPut + " " + engagement + "/on-call-rule":                          "{",
		http.MethodPut + " " + engagement + "/attachments/" + f.doulaID + "/on-call": "{",
		http.MethodPut + " " + base + "/on-call-settings":                            "{",
	}
	for call, body := range malformed {
		method, url, _ := strings.Cut(call, " ")
		resp := postRaw(t, session, method, url, body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s with a malformed body = %d, want 400", call, resp.StatusCode)
		}
	}

	badPaths := map[string]string{
		http.MethodPost + " " + bad + "/coverage-gaps":                        "{}",
		http.MethodPut + " " + bad + "/on-call-rule":                          "{}",
		http.MethodPut + " " + bad + "/attachments/" + f.doulaID + "/on-call": "{}",
		http.MethodPut + " " + engagement + "/attachments/nope/on-call":       "{}",
		http.MethodPut + " " + engagement + "/coverage-gaps/nope":             "{}",
		http.MethodDelete + " " + engagement + "/coverage-gaps/nope":          "",
		http.MethodGet + " " + bad + "/on-call":                               "",
	}
	for call, body := range badPaths {
		method, url, _ := strings.Cut(call, " ")
		resp := postRaw(t, session, method, url, body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s with a malformed id = %d, want 400", call, resp.StatusCode)
		}
	}
}

func TestWrites_RefuseWhatHasNoWindowOrNoEngagement(t *testing.T) {
	db := testdb.New(t)
	f := newSoloFixture(t, db, "refuse-window")
	srv, session := newServer(t, db, f.ownerUID)
	defer srv.Close()

	gapURL := gapsURL(srv.URL, f.practiceID, f.engagementID) + "/" + f.engagementID
	exec(t, db, `UPDATE engagements SET due_date = NULL WHERE id = $1`, f.engagementID)
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		resp := authedBody(t, session, method, gapURL, gapBody(f.doulaID, nil))
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusConflict {
			t.Errorf("%s on a birth with no window = %d, want 409", method, resp.StatusCode)
		}
	}

	const unknown = "00000000-0000-0000-0000-000000000000"
	granted := oncall.StartAttachmentGranted
	doJSON[map[string]any](t, session, http.MethodPut, ruleURL(srv.URL, f.practiceID, unknown), oncall.RuleRequest{StartRule: &granted}, http.StatusNotFound)
}

func TestEngagementRule_AWeekChangeUnderTheSameRuleIsRecorded(t *testing.T) {
	db := testdb.New(t)
	f := newSoloFixture(t, db, "rule-week")
	srv, session := newServer(t, db, f.ownerUID)
	defer srv.Close()

	rule := oncall.StartGestationalWeek
	for _, week := range []int{38, 39, 39} {
		doJSON[oncall.RuleRequest](t, session, http.MethodPut, ruleURL(srv.URL, f.practiceID, f.engagementID)+"?w="+string(rune('0'+week%10)),
			oncall.RuleRequest{StartRule: &rule, StartWeek: &week}, http.StatusOK)
	}
	if n := len(activityActors(t, db, f.engagementID, "on_call_rule_changed")); n != 2 {
		t.Fatalf("rule entries = %d, want 2 -- the repeat of week 39 records nothing", n)
	}
}

// TestEngagementRule_RefusesEveryMalformedOverride walks the shapes a
// start-rule override cannot take.
func TestEngagementRule_RefusesEveryMalformedOverride(t *testing.T) {
	db := testdb.New(t)
	f := newSoloFixture(t, db, "rule-malformed")
	srv, session := newServer(t, db, f.ownerUID)
	defer srv.Close()

	week, tooEarly := 39, 12
	gestational, unknown := oncall.StartGestationalWeek, oncall.StartRule("whenever")
	tests := map[string]struct {
		req   oncall.RuleRequest
		field string
	}{
		"a week with no rule to apply it to":  {oncall.RuleRequest{StartWeek: &week}, fieldWeek},
		"a week outside the bounds":           {oncall.RuleRequest{StartRule: &gestational, StartWeek: &tooEarly}, fieldWeek},
		"a gestational rule with no week":     {oncall.RuleRequest{StartRule: &gestational}, fieldWeek},
		"a rule name the product does not do": {oncall.RuleRequest{StartRule: &unknown}, "startRule"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			body := doJSON[map[string]any](t, session, http.MethodPut, ruleURL(srv.URL, f.practiceID, f.engagementID), tt.req, http.StatusBadRequest)
			if details, _ := body["details"].(map[string]any); details[tt.field] == nil {
				t.Fatalf("details = %v, want a refusal under %q", body["details"], tt.field)
			}
		})
	}
}

// TestUpdateGap_RefusesTheSameThingsACreateDoes: the edit is a full
// replace, so it is held to the same rules, including who may be named.
func TestUpdateGap_RefusesTheSameThingsACreateDoes(t *testing.T) {
	db := testdb.New(t)
	f := newGapEditFixture(t, db, "gap-edit-refuse")
	srv, session := newServer(t, db, f.ownerUID)
	defer srv.Close()

	noEnd := gapBody(f.doulaID, nil)
	noEnd.EndsAt = nil
	body := doJSON[map[string]any](t, session, http.MethodPut, f.gapURL(srv.URL), noEnd, http.StatusBadRequest)
	if details, _ := body["details"].(map[string]any); details["endsAt"] != oncall.MsgEndsAtRequired {
		t.Fatalf("details = %v, want the missing end refused", body["details"])
	}

	// The gap's own Doula may edit it, and may not hand it to a colleague.
	doulaSrv, doulaSession := newServer(t, db, "gap-edit-refuse-doula")
	defer doulaSrv.Close()
	doJSON[map[string]any](t, doulaSession, http.MethodPut, f.gapURL(doulaSrv.URL), gapBody(f.backupID, nil), http.StatusForbidden)
	doJSON[oncall.Gap](t, doulaSession, http.MethodPut, f.gapURL(doulaSrv.URL)+"?own", gapBody(f.doulaID, &f.backupID), http.StatusOK)
}

// TestPanel_ShowsTheGapsOnTheBirth: the Engagement's own panel carries
// the live gaps, so a Doula can see her cover from the birth's page.
func TestPanel_ShowsTheGapsOnTheBirth(t *testing.T) {
	db := testdb.New(t)
	f := newGapEditFixture(t, db, "panel-gaps")
	srv, session := newServer(t, db, f.ownerUID)
	defer srv.Close()

	panel := getJSON[oncall.EngagementOnCall](t, session, panelURL(srv.URL, f.practiceID, f.engagementID), http.StatusOK)
	if len(panel.Gaps) != 1 || panel.Gaps[0].ID != f.gapID {
		t.Fatalf("panel gaps = %+v, want the one saved gap", panel.Gaps)
	}
}

func TestCreateGap_RefusesALongReasonAndTakesABlankCover(t *testing.T) {
	db := testdb.New(t)
	f := newSoloFixture(t, db, "gap-reason")
	srv, session := newServer(t, db, f.ownerUID)
	defer srv.Close()

	long := strings.Repeat("a", 501)
	body := gapBody(f.doulaID, nil)
	body.Reason = &long
	doJSON[map[string]any](t, session, http.MethodPost, gapsURL(srv.URL, f.practiceID, f.engagementID), body, http.StatusBadRequest)

	blank := ""
	body = gapBody(f.doulaID, &blank)
	body.Reason = nil
	gap := doJSON[oncall.Gap](t, session, http.MethodPost, gapsURL(srv.URL, f.practiceID, f.engagementID)+"?blank", body, http.StatusCreated)
	if gap.CoveringStaffID != nil || gap.Reason != nil {
		t.Fatalf("gap = %+v, want a blank cover and reason read as none", gap)
	}
}

// TestRoster_ADepartedCoveringColleagueIsNamedAsOne: a Doula whose
// Membership has ended is no longer a row this Practice can read, so the
// roster says so in words rather than printing a bare id.
func TestRoster_ADepartedCoveringColleagueIsNamedAsOne(t *testing.T) {
	db := testdb.New(t)
	f := newSoloFixture(t, db, "roster-departed")
	backupID := testdb.SeedNamedStaffAtPractice(t, db, f.practiceID, "roster-departed-backup", backupName, []string{doulaRole}, employeeType)
	testdb.SeedGrantedAttachment(t, db, f.engagementID, backupID)
	exec(t, db, `INSERT INTO engagement_coverage_gaps (engagement_id, staff_id, covering_staff_id, starts_at, ends_at, created_by)
		VALUES ($1, $2, $3, '2026-10-17T22:00:00Z', '2026-10-18T10:00:00Z', $2)`, f.engagementID, f.doulaID, backupID)
	testdb.RemoveMembership(t, db, backupID)

	w := f.roster(t, db, "2026-10-17", "2026-10-17").Windows[0]
	if w.Gaps[0].CoveringStaffName == nil || *w.Gaps[0].CoveringStaffName != "a former colleague" {
		t.Fatalf("covering name = %v, want the words for a colleague this Practice can no longer read", w.Gaps[0].CoveringStaffName)
	}
}

// TestRoster_SortsAndSkipsWhatItShould covers the two orderings the
// screen depends on and a narrowing that misses the window entirely.
func TestRoster_SortsAndSkipsWhatItShould(t *testing.T) {
	db := testdb.New(t)
	f := newSoloFixture(t, db, "roster-sort")
	zeb := seedBirth(t, db, f.practiceID, "Zeb Nakamura", "")
	ada := seedBirth(t, db, f.practiceID, "Ada Second", "")
	testdb.SeedGrantedAttachment(t, db, zeb, f.doulaID)
	testdb.SeedGrantedAttachment(t, db, ada, f.doulaID)
	// A narrowing that misses the window: she is attached and on call for
	// none of it, so the whole window reads as unstaffed.
	exec(t, db, `UPDATE engagement_attachments SET on_call_from = '2027-01-01' WHERE engagement_id = $1`, f.engagementID)

	got := f.roster(t, db, windowStart, octLast)
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
	db := testdb.New(t)
	f := newNoticeFixture(t, db, "notice-nobody")
	exec(t, db, `DELETE FROM practice_memberships WHERE practice_id = $1 AND (roles && ARRAY['owner','admin']::practice_role[])`, f.practiceID)

	runWorker(t, db, &mail.FakeSender{})
	if got := noticeStatus(t, db, f.gapID); got != statusSent {
		t.Fatalf("status = %q, want sent", got)
	}
	if n := countRows(t, db, `SELECT count(*) FROM coverage_gap_outbox WHERE gap_id = $1 AND notified_staff_ids = '{}'`, f.gapID); n != 1 {
		t.Fatal("the row does not record that it addressed nobody")
	}
}
