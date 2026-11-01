package plans_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/plans"
	"doula-cloud/api/internal/testdb"
)

const birthKind = "birth"

const birthPlanStartFields = `[{"id":"f1","type":"short_text","label":"Name","order":0}]`

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
		{"postpartum", "postpartum", "", plans.MsgBirthPlanOnPostpartum},
		{"loss", birthKind, "loss", plans.MsgBirthPlanAfterOutcome},
		{"unknown", birthKind, "unknown", plans.MsgBirthPlanAfterOutcome},
		{"live birth", birthKind, "live_birth", plans.MsgBirthPlanAfterOutcome},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			const uid = "post-birth-plan-refused"
			practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
			_, engagementID := testdb.SeedEngagementWithKind(t, db, practiceID, "Camille Boyd", "camille@example.com", tc.kind)
			if tc.outcome != "" {
				endedOn := "2026-10-24"
				recordOutcome(t, db, engagementID, tc.outcome, &endedOn)
			}
			seedPlanTemplate(t, db, practiceID, birthPlanType, birthPlanStartFields)
			seedPlanTemplate(t, db, practiceID, carePlanType, birthPlanStartFields)

			srv, session := newPlanServer(t, db, uid)
			defer srv.Close()

			resp := postInstance(t, srv, session, practiceID, engagementID, birthPlanType)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusConflict {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
			}
			var body apierr.APIError
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if body.Code != apierr.CodeConflict || body.Message != tc.wantMsg {
				t.Fatalf("body = %+v, want conflict %q", body, tc.wantMsg)
			}

			var count int
			if err := db.Admin.QueryRowContext(t.Context(),
				`SELECT count(*) FROM plan_instances WHERE engagement_id = $1 AND plan_type = 'birth_plan'`, engagementID,
			).Scan(&count); err != nil {
				t.Fatalf("count: %v", err)
			}
			if count != 0 {
				t.Fatalf("birth plan instances = %d, want 0", count)
			}

			care := postInstance(t, srv, session, practiceID, engagementID, carePlanType)
			defer care.Body.Close()
			if care.StatusCode != http.StatusCreated {
				t.Fatalf("care plan status = %d, want %d", care.StatusCode, http.StatusCreated)
			}
		})
	}
}
