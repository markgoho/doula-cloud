package oncall_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/oncall"
	"doula-cloud/api/internal/testdb"
)

func settingsURL(srvURL, practiceID string) string {
	return srvURL + "/api/practices/" + practiceID + "/on-call-settings"
}

func TestSettings_AnOwnerReadsAndStatesTheRule(t *testing.T) {
	db := testdb.New(t)
	f := newSoloFixture(t, db, "settings-owner")
	srv, session := newServer(t, db, f.ownerUID)
	defer srv.Close()

	got := decode[oncall.Settings](t, authedGet(t, session, settingsURL(srv.URL, f.practiceID)), http.StatusOK)
	if got != (oncall.Settings{StartRule: oncall.StartGestationalWeek, StartWeek: 37, GraceDays: 14}) {
		t.Fatalf("default settings = %+v, want 37 weeks and 14 days", got)
	}

	want := oncall.Settings{StartRule: oncall.StartAttachmentGranted, StartWeek: 37, GraceDays: 10}
	decode[oncall.Settings](t, authedBody(t, session, http.MethodPut, settingsURL(srv.URL, f.practiceID), want), http.StatusOK)
	decode[oncall.Settings](t, authedBody(t, session, http.MethodPut, settingsURL(srv.URL, f.practiceID)+"?again", want), http.StatusOK)
	if got := decode[oncall.Settings](t, authedGet(t, session, settingsURL(srv.URL, f.practiceID)), http.StatusOK); got != want {
		t.Fatalf("settings = %+v, want %+v", got, want)
	}
	if n := countRows(t, db, `SELECT count(*) FROM activity WHERE subject_id = $1 AND action = 'practice_on_call_changed'`, f.practiceID); n != 1 {
		t.Fatalf("activity entries = %d, want 1 (a same-body retry records nothing)", n)
	}
}

func TestSettings_RefusesOutOfRangeValuesAndADoula(t *testing.T) {
	db := testdb.New(t)
	f := newSoloFixture(t, db, "settings-refuse")
	srv, session := newServer(t, db, f.ownerUID)
	defer srv.Close()

	body := decode[map[string]any](t, authedBody(t, session, http.MethodPut, settingsURL(srv.URL, f.practiceID),
		oncall.Settings{StartRule: "whenever", StartWeek: 12, GraceDays: 60}), http.StatusBadRequest)
	details, _ := body["details"].(map[string]any)
	for _, field := range []string{"startRule", "startWeek", "graceDays"} {
		if _, ok := details[field]; !ok {
			t.Errorf("no refusal under %q: %v", field, details)
		}
	}

	doulaSrv, doulaSession := newServer(t, db, "settings-refuse-doula")
	defer doulaSrv.Close()
	resp := authedGet(t, doulaSession, settingsURL(doulaSrv.URL, f.practiceID))
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("a Doula reading the rule got %d, want 403", resp.StatusCode)
	}
}

func ruleURL(srvURL, practiceID, engagementID string) string {
	return srvURL + "/api/practices/" + practiceID + "/engagements/" + engagementID + "/on-call-rule"
}

func TestEngagementRule_OverridesAndClears(t *testing.T) {
	db := testdb.New(t)
	f := newSoloFixture(t, db, "rule-override")
	srv, session := newServer(t, db, f.ownerUID)
	defer srv.Close()

	week, rule := 39, oncall.StartGestationalWeek
	decode[oncall.RuleRequest](t, authedBody(t, session, http.MethodPut, ruleURL(srv.URL, f.practiceID, f.engagementID),
		oncall.RuleRequest{StartRule: &rule, StartWeek: &week}), http.StatusOK)
	panel := decode[oncall.EngagementOnCall](t, authedGet(t, session, panelURL(srv.URL, f.practiceID, f.engagementID)), http.StatusOK)
	if !panel.Rule.Overridden || panel.Rule.StartWeek != 39 || panel.Window.Start != "2026-10-23" {
		t.Fatalf("panel = %+v, want a 39-week override opening 2026-10-23", panel)
	}

	decode[oncall.RuleRequest](t, authedBody(t, session, http.MethodPut, ruleURL(srv.URL, f.practiceID, f.engagementID)+"?clear", oncall.RuleRequest{}), http.StatusOK)
	panel = decode[oncall.EngagementOnCall](t, authedGet(t, session, panelURL(srv.URL, f.practiceID, f.engagementID)), http.StatusOK)
	if panel.Rule.Overridden || panel.Window.Start != "2026-10-09" {
		t.Fatalf("after clearing, panel = %+v, want the Practice's rule back", panel)
	}
	if n := len(activityActors(t, db, f.engagementID, "on_call_rule_changed")); n != 2 {
		t.Fatalf("rule entries = %d, want 2", n)
	}

	granted := oncall.StartAttachmentGranted
	body := decode[map[string]any](t, authedBody(t, session, http.MethodPut, ruleURL(srv.URL, f.practiceID, f.engagementID)+"?bad",
		oncall.RuleRequest{StartRule: &granted, StartWeek: &week}), http.StatusBadRequest)
	if details, _ := body["details"].(map[string]any); details["startWeek"] != oncall.MsgWeekWithoutRule {
		t.Fatalf("details = %v, want a week refused beside the grant-date rule", details)
	}
}

func TestEngagementRule_RefusesAPostpartumEngagement(t *testing.T) {
	db := testdb.New(t)
	f := newSoloFixture(t, db, "rule-postpartum")
	exec(t, db, `UPDATE engagements SET kind = 'postpartum' WHERE id = $1`, f.engagementID)
	srv, session := newServer(t, db, f.ownerUID)
	defer srv.Close()

	granted := oncall.StartAttachmentGranted
	decode[map[string]any](t, authedBody(t, session, http.MethodPut, ruleURL(srv.URL, f.practiceID, f.engagementID),
		oncall.RuleRequest{StartRule: &granted}), http.StatusConflict)
	panel := decode[oncall.EngagementOnCall](t, authedGet(t, session, panelURL(srv.URL, f.practiceID, f.engagementID)), http.StatusOK)
	if panel.Window != nil || panel.NoWindowReason != oncall.NoWindowPostpartum {
		t.Fatalf("panel = %+v, want no window because it is postpartum", panel)
	}
}
