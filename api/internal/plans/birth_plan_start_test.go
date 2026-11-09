package plans_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/plans"
	"doula-cloud/api/internal/testdb"
)

// TestPostInstanceHandler_RefusesBirthPlanItDoesNotOffer proves ADR-0015's
// create side at the write (#2277, #2317): a postpartum Engagement and a
// birth Engagement with any recorded outcome get no new Birth Plan, and
// nothing is inserted. A Care Plan on the same Engagement still starts.
func TestPostInstanceHandler_RefusesBirthPlanItDoesNotOffer(t *testing.T) {
	cases := []struct {
		name    string
		kind    string
		outcome string
		wantMsg string
	}{
		{"postpartum", fixture.KindPostpartum, "", plans.MsgBirthPlanOnPostpartum},
		{"loss", fixture.KindBirth, "loss", plans.MsgBirthPlanAfterOutcome},
		{"unknown", fixture.KindBirth, "unknown", plans.MsgBirthPlanAfterOutcome},
		{"live birth", fixture.KindBirth, "live_birth", plans.MsgBirthPlanAfterOutcome},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, withEngagement(fixture.Kind(tc.kind)))
			if tc.outcome != "" {
				endedOn := "2026-10-24"
				e.recordOutcome(tc.outcome, &endedOn)
			}
			e.seedTemplate(birthPlanType, f1Fields)
			e.seedTemplate(carePlanType, f1Fields)

			body := apierrtest.Expect(t, http.MethodPost, e.instanceURL(birthPlanType, ""), http.StatusConflict, e.SignedIn)
			if body.Code != apierr.CodeConflict || body.Message != tc.wantMsg {
				t.Fatalf("body = %+v, want conflict %q", body, tc.wantMsg)
			}

			count := testdb.Scalar[int](t, e.DB.Admin, `SELECT count(*) FROM plan_instances WHERE engagement_id = $1 AND plan_type = 'birth_plan'`, e.Engagement.ID)
			if count != 0 {
				t.Fatalf("birth plan instances = %d, want 0", count)
			}

			e.start(carePlanType, http.StatusCreated)
		})
	}
}
