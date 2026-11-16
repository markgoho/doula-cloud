package oncall_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/oncall"
	"doula-cloud/api/internal/reqtest"
)

// #2755: the Engagement read's glance and the on-call panel are one
// derivation. A birth nobody is on gets the same window and the same
// "nobody holds it tonight" from both, under either start rule.
func TestPanelAndGlance_AgreeOnABirthNobodyIsOn(t *testing.T) {
	cases := []struct {
		name       string
		rule       string
		wantWindow oncall.Window
	}{
		{"gestational week", "gestational_week", oncall.Window{Start: "2026-09-29", End: novThird}},
		{"attachment granted", "attachment_granted", oncall.Window{Start: octFirst, End: novThird}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			orphan := e.birthDue("Orla Unstaffed", "2026-10-20")
			e.Exec(`UPDATE practices SET on_call_start_rule = $2 WHERE id = $1`, e.Practice.ID, tc.rule)

			panel := reqtest.Expect[oncall.EngagementOnCall](t, http.MethodGet,
				e.PracticeURL("/engagements/"+orphan.ID+"/on-call"), http.StatusOK, e.SignedIn)

			tx := e.DB.BeginAdmin(t)
			ctx := clock.Into(context.Background(), func() time.Time { return sceneNow })
			window, tonight, err := oncall.EngagementGlance(ctx, tx, e.Practice.ID, orphan.ID)
			if err != nil {
				t.Fatal(err)
			}

			if panel.Window == nil || *panel.Window != tc.wantWindow {
				t.Errorf("panel window = %+v, want %+v", panel.Window, tc.wantWindow)
			}
			if window == nil || *window != tc.wantWindow {
				t.Errorf("glance window = %+v, want %+v", window, tc.wantWindow)
			}
			if panel.Tonight == nil || !panel.Tonight.IsNobody {
				t.Errorf("panel tonight = %+v, want nobody holding it", panel.Tonight)
			}
			if tonight == nil || !tonight.IsNobody {
				t.Errorf("glance tonight = %+v, want nobody holding it", tonight)
			}
			if panel.NoWindowReason != "" {
				t.Errorf("panel reason = %q, want none", panel.NoWindowReason)
			}
		})
	}
}

// #2755: a birth with no due date and nobody on it has no window, so the
// glance gives the Engagement read neither a window nor a tonight.
func TestGlance_NoWindowGivesNothing(t *testing.T) {
	e := newEnv(t)
	undated := e.birthDue("Una Undated", "")

	tx := e.DB.BeginAdmin(t)
	ctx := clock.Into(context.Background(), func() time.Time { return sceneNow })
	window, tonight, err := oncall.EngagementGlance(ctx, tx, e.Practice.ID, undated.ID)
	if err != nil {
		t.Fatal(err)
	}
	if window != nil || tonight != nil {
		t.Errorf("glance = %+v, %+v; want no window and no tonight", window, tonight)
	}
}
