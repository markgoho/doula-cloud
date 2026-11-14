package oncall_test

import (
	"net/http"
	"slices"
	"testing"

	"doula-cloud/api/internal/oncall"
	"doula-cloud/api/internal/reqtest"
)

func (e *env) effectURL(query string) string {
	return e.PracticeURL("/on-call-settings/effect" + query)
}

func (e *env) effect(query string) oncall.Effect {
	e.T.Helper()
	return reqtest.Expect[oncall.Effect](e.T, http.MethodGet, e.effectURL(query), http.StatusOK, e.SignedIn)
}

func names(births []oncall.EffectBirth) []string {
	out := []string{}
	for _, b := range births {
		out = append(out, b.ClientName)
	}
	return out
}

func TestSettings_LastChangedNamesWhoChangedTheRuleAndWhen(t *testing.T) {
	e := newEnv(t)
	if got := e.settingsRead(); got.LastChanged != nil {
		t.Fatalf("lastChanged = %+v, want none while the rule is as the Practice was set up", got.LastChanged)
	}

	want := oncall.Settings{StartRule: oncall.StartGestationalWeek, StartWeek: 38, GraceDays: 14}
	put := reqtest.Expect[oncall.SettingsResponse](t, http.MethodPut, e.settingsURL(), http.StatusOK, e.SignedIn, reqtest.JSON(want))
	if put.LastChanged == nil || put.LastChanged.StaffName != e.Staff.Name || put.LastChanged.At.IsZero() {
		t.Fatalf("put lastChanged = %+v, want %s and a time", put.LastChanged, e.Staff.Name)
	}

	read := e.settingsRead()
	if read.Settings != want || read.LastChanged == nil || !read.LastChanged.At.Equal(put.LastChanged.At) || read.LastChanged.StaffName != e.Staff.Name {
		t.Errorf("read = %+v, want the rule and the same stamp the write answered", read)
	}

	again := reqtest.Expect[oncall.SettingsResponse](t, http.MethodPut, e.settingsURL(), http.StatusOK, e.SignedIn, reqtest.JSON(want))
	if again.LastChanged == nil || !again.LastChanged.At.Equal(put.LastChanged.At) {
		t.Errorf("a retry moved lastChanged to %+v, want it left where it was", again.LastChanged)
	}
}

func TestEffect_NamesTheBirthsASavedRuleWouldMoveToday(t *testing.T) {
	e := newEnv(t, dueOn(octEighth))
	e.birthDue("Fiona Future", "2026-10-30", e.Doula)
	own := e.birthDue("Quinn Own", "2026-10-10", e.Doula)
	e.Exec(`UPDATE engagements SET on_call_start_rule = 'gestational_week', on_call_start_week = 38 WHERE id = $1`, own.ID)
	e.birthDue("Orla Unstaffed", "2026-10-09")
	e.birthDue("Bea Both", octEighth, e.Doula)

	t.Run("a later start takes today's birth off call", func(t *testing.T) {
		got := e.effect("?startRule=gestational_week&startWeek=40&graceDays=14")
		if !equalNames(names(got.StopToday), adaWhitfield, "Bea Both") || len(got.StartToday) != 0 || !equalNames(names(got.KeepOwnStart), "Quinn Own") {
			t.Errorf("effect = %+v, want Ada and Bea off call and Quinn keeping its own start", got)
		}
	})

	t.Run("an earlier start puts a birth on call today", func(t *testing.T) {
		got := e.effect("?startRule=gestational_week&startWeek=20&graceDays=14")
		if !equalNames(names(got.StartToday), "Fiona Future") || len(got.StopToday) != 0 || !equalNames(names(got.KeepOwnStart), "Quinn Own") {
			t.Errorf("effect = %+v, want Fiona put on call and Quinn keeping its own start", got)
		}
	})

	t.Run("the rule already in force changes nothing", func(t *testing.T) {
		got := e.effect("?startRule=gestational_week&startWeek=37&graceDays=14")
		if len(got.StopToday) != 0 || len(got.StartToday) != 0 {
			t.Errorf("effect = %+v, want no birth moved", got)
		}
	})

	t.Run("nothing is saved", func(t *testing.T) {
		if got := e.settings(); got != (oncall.Settings{StartRule: oncall.StartGestationalWeek, StartWeek: 37, GraceDays: 14}) {
			t.Errorf("settings = %+v, want the rule untouched", got)
		}
		if n := e.count(`SELECT count(*) FROM activity WHERE action = 'practice_on_call_changed'`); n != 0 {
			t.Errorf("activity entries = %d, want none from a preview", n)
		}
	})
}

func equalNames(got []string, want ...string) bool { return slices.Equal(got, want) }

func TestEffect_RefusesWhatTheWriteRefusesUnderTheSameKeys(t *testing.T) {
	e := newEnv(t)

	for name, query := range map[string]string{
		"values out of range":         "?startRule=whenever&startWeek=50&graceDays=60",
		"values that are not numbers": "?startRule=gestational_week&startWeek=abc&graceDays=",
		"no values at all":            "",
	} {
		t.Run(name, func(t *testing.T) {
			got := reqtest.Expect[refusal](t, http.MethodGet, e.effectURL(query), http.StatusBadRequest, e.SignedIn)

			if got.Message != oncall.MsgEffectNotShown {
				t.Errorf("message = %q, want %q", got.Message, oncall.MsgEffectNotShown)
			}
			if query == "?startRule=whenever&startWeek=50&graceDays=60" {
				if got.Details["startRule"] != oncall.MsgUnknownStartRule || got.Details[fieldWeek] != oncall.MsgStartWeekRange || got.Details["graceDays"] != oncall.MsgGraceDaysRange {
					t.Errorf("details = %v, want the write's three words", got.Details)
				}
			}
			if len(got.Details) == 0 {
				t.Error("no field named")
			}
		})
	}
}

func TestEffect_IsForAnOwnerOrAnAdminOnly(t *testing.T) {
	e := newEnv(t)

	reqtest.ExpectStatus(t, http.MethodGet, e.effectURL("?startRule=gestational_week&startWeek=37&graceDays=14"), http.StatusForbidden, e.As(e.Doula))
}
