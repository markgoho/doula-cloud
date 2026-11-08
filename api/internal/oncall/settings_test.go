package oncall_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/oncall"
	"doula-cloud/api/internal/reqtest"
)

func (e *env) putSettings(s oncall.Settings) {
	e.t.Helper()
	reqtest.ExpectStatus(e.t, http.MethodPut, e.settingsURL(), http.StatusOK, e.asOwner, reqtest.JSON(s))
}

func (e *env) settings() oncall.Settings {
	e.t.Helper()
	return reqtest.Expect[oncall.Settings](e.t, http.MethodGet, e.settingsURL(), http.StatusOK, e.asOwner)
}

func (e *env) putRule(r oncall.RuleRequest, want int) {
	e.t.Helper()
	reqtest.ExpectStatus(e.t, http.MethodPut, e.ruleURL(), want, e.asOwner, reqtest.JSON(r))
}

func TestSettings_AnOwnerReadsAndStatesTheRule(t *testing.T) {
	e := newEnv(t)

	if got := e.settings(); got != (oncall.Settings{StartRule: oncall.StartGestationalWeek, StartWeek: 37, GraceDays: 14}) {
		t.Fatalf("default settings = %+v, want 37 weeks and 14 days", got)
	}

	want := oncall.Settings{StartRule: oncall.StartAttachmentGranted, StartWeek: 37, GraceDays: 10}
	e.putSettings(want)
	e.putSettings(want)

	if got := e.settings(); got != want {
		t.Fatalf("settings = %+v, want %+v", got, want)
	}
	if n := e.count(`SELECT count(*) FROM activity WHERE subject_id = $1 AND action = 'practice_on_call_changed'`, e.Practice.ID); n != 1 {
		t.Fatalf("activity entries = %d, want 1 (a same-body retry records nothing)", n)
	}
}

func TestSettings_RefusesOutOfRangeValuesAndADoula(t *testing.T) {
	e := newEnv(t)

	got := reqtest.Expect[refusal](t, http.MethodPut, e.settingsURL(), http.StatusBadRequest, e.asOwner,
		reqtest.JSON(oncall.Settings{StartRule: "whenever", StartWeek: 12, GraceDays: 60}))
	for _, field := range []string{"startRule", fieldWeek, "graceDays"} {
		if _, ok := got.Details[field]; !ok {
			t.Errorf("no refusal under %q: %v", field, got.Details)
		}
	}

	reqtest.ExpectStatus(t, http.MethodGet, e.settingsURL(), http.StatusForbidden, e.as(e.Doula))
}

func TestEngagementRule_OverridesAndClears(t *testing.T) {
	e := newEnv(t)

	week, rule := 39, oncall.StartGestationalWeek
	e.putRule(oncall.RuleRequest{StartRule: &rule, StartWeek: &week}, http.StatusOK)
	if panel := e.panel(); !panel.Rule.Overridden || panel.Rule.StartWeek != 39 || panel.Window.Start != octTwentyThree {
		t.Fatalf("panel = %+v, want a 39-week override opening 2026-10-23", panel)
	}

	e.putRule(oncall.RuleRequest{}, http.StatusOK)
	if panel := e.panel(); panel.Rule.Overridden || panel.Window.Start != windowStart {
		t.Fatalf("after clearing, panel = %+v, want the Practice's rule back", panel)
	}
	if n := len(e.activityActors("on_call_rule_changed")); n != 2 {
		t.Fatalf("rule entries = %d, want 2", n)
	}

	granted := oncall.StartAttachmentGranted
	got := reqtest.Expect[refusal](t, http.MethodPut, e.ruleURL(), http.StatusBadRequest, e.asOwner,
		reqtest.JSON(oncall.RuleRequest{StartRule: &granted, StartWeek: &week}))
	if got.Details[fieldWeek] != oncall.MsgWeekWithoutRule {
		t.Fatalf("details = %v, want a week refused beside the grant-date rule", got.Details)
	}
}

func TestEngagementRule_RefusesAPostpartumEngagement(t *testing.T) {
	e := newEnv(t)
	e.exec(`UPDATE engagements SET kind = 'postpartum' WHERE id = $1`, e.Birth.ID)

	granted := oncall.StartAttachmentGranted
	e.putRule(oncall.RuleRequest{StartRule: &granted}, http.StatusConflict)

	if panel := e.panel(); panel.Window != nil || panel.NoWindowReason != oncall.NoWindowPostpartum {
		t.Fatalf("panel = %+v, want no window because it is postpartum", panel)
	}
}
