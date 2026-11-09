package plans_test

import (
	"encoding/json/v2"
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/plans"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

const notAPlanType = "not_a_plan_type"

func TestGetTemplateHandler_Refusals(t *testing.T) {
	cases := []struct {
		name     string
		planType string
		want     int
	}{
		{"unknown plan type", notAPlanType, http.StatusBadRequest},
		{"no template yet", carePlanType, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)

			reqtest.ExpectStatus(t, http.MethodGet, e.templateURL(tc.planType), tc.want, e.SignedIn)
		})
	}
}

// TestGetTemplateHandler_AnyMemberAllowed proves every Staff role --
// Owner, Admin, an employed Doula and a contractor Doula -- can read the
// template (ADR-0008's read table): only the PUT is Owner-gated (#1407).
func TestGetTemplateHandler_AnyMemberAllowed(t *testing.T) {
	for _, tc := range []roleCase{ownerCase, adminCase, employeeDoulaCase, contractorDoulaCase} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.opts...))
			e.seedTemplate(carePlanType, f1Fields)

			out := e.template(carePlanType)

			if out.PlanType != carePlanType {
				t.Fatalf("planType = %q, want care_plan", out.PlanType)
			}
			if len(out.Fields) != 1 || out.Fields[0].ID != "f1" {
				t.Fatalf("fields = %+v, want one field with id f1", out.Fields)
			}
		})
	}
}

// TestPutTemplateHandler_NonOwnerForbidden proves the mount's OwnerOnly
// declaration (#1407) refuses both a plain Doula and an Admin -- the
// in-handler check this replaced was reader.Has("owner") alone, and the
// mount declaration must keep exactly that population, not widen it to
// Owner and Admin. The refusal also leaves the stored template untouched.
func TestPutTemplateHandler_NonOwnerForbidden(t *testing.T) {
	for _, tc := range []roleCase{employeeDoulaCase, adminCase} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.opts...))
			e.seedTemplate(carePlanType, f1Fields)

			e.putTemplate(carePlanType, []plans.Field{{ID: "f2", Type: shortTextType, Label: testFieldLabel}}, http.StatusForbidden)

			stored := testdb.Scalar[string](t, e.DB.Admin, `SELECT fields->0->>'id' FROM plan_templates WHERE practice_id = $1 AND plan_type = $2`, e.Practice.ID, carePlanType)
			if stored != "f1" {
				t.Fatalf("stored first field id = %q, want f1 (the refused PUT must not write)", stored)
			}
		})
	}
}

// TestPutTemplateHandler_Refusals table-drives an unknown plan type, a
// body that is not JSON and every normalizeFields error branch: unknown
// field type, missing id, duplicate id, missing label, a select field with
// no options, a select field with a blank option, and a non-select field
// carrying options.
func TestPutTemplateHandler_Refusals(t *testing.T) {
	fields := func(fields ...plans.Field) reqtest.Option {
		return reqtest.JSON(plans.TemplateResponse{Fields: fields})
	}
	cases := []struct {
		name     string
		planType string
		body     reqtest.Option
	}{
		{"unknown plan type", notAPlanType, fields(plans.Field{ID: "f1", Type: shortTextType, Label: testFieldLabel})},
		{"body is not JSON", carePlanType, reqtest.RawJSON([]byte("not json"))},
		{"unknown field type", carePlanType, fields(plans.Field{ID: "f1", Type: "essay", Label: testFieldLabel})},
		{"missing id", carePlanType, fields(plans.Field{ID: "", Type: shortTextType, Label: testFieldLabel})},
		{"duplicate id", carePlanType, fields(
			plans.Field{ID: "f1", Type: shortTextType, Label: testFieldLabel},
			plans.Field{ID: "f1", Type: shortTextType, Label: "Name again"},
		)},
		{"missing label", carePlanType, fields(plans.Field{ID: "f1", Type: shortTextType, Label: ""})},
		{"select with no options", carePlanType, fields(plans.Field{ID: "f1", Type: "single_select", Label: "Pick one"})},
		{"select with blank option", carePlanType, fields(plans.Field{ID: "f1", Type: "multi_select", Label: "Pick some", Options: []string{"A", ""}})},
		{"non-select with options", carePlanType, fields(plans.Field{ID: "f1", Type: "checkbox", Label: "Agree", Options: []string{"yes"}})},
	}
	e := newEnv(t, signedInAs(fixture.Owner()))

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reqtest.ExpectStatus(t, http.MethodPut, e.templateURL(tc.planType), http.StatusBadRequest, e.SignedIn, tc.body)
		})
	}
}

// TestPutTemplateHandler_Success proves a full replace round-trips
// through GET, and that stored Order always reflects array position --
// not any Order value the client sent -- since the client here sends
// fields out of Order to prove the server recomputes it.
func TestPutTemplateHandler_Success(t *testing.T) {
	const secondFieldID = "second"
	const firstFieldID = "first"
	e := newEnv(t, signedInAs(fixture.Owner()))

	putOut := e.putTemplate(birthPlanType, []plans.Field{
		{ID: secondFieldID, Type: "checkbox", Label: "Consent", Order: 99},
		{ID: firstFieldID, Type: "single_select", Label: "Location", Options: []string{"Home", hospital}, Order: 1},
	}, http.StatusOK)

	if len(putOut.Fields) != 2 || putOut.Fields[0].ID != secondFieldID || putOut.Fields[0].Order != 0 || putOut.Fields[1].ID != firstFieldID || putOut.Fields[1].Order != 1 {
		t.Fatalf("PUT fields = %+v, want order recomputed from array position", putOut.Fields)
	}

	getOut := e.template(birthPlanType)
	if len(getOut.Fields) != 2 || getOut.Fields[0].ID != secondFieldID || getOut.Fields[1].ID != firstFieldID {
		t.Fatalf("GET fields after PUT = %+v, want the just-written fields", getOut.Fields)
	}
}

// TestPutTemplateHandler_ReplacesExistingRow proves PUT overwrites a row
// seeded by signup (or a prior PUT) rather than conflicting with it -- the
// ON CONFLICT DO UPDATE path.
func TestPutTemplateHandler_ReplacesExistingRow(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.seedTemplate(carePlanType, `[{"id":"old","type":"short_text","label":"Old field","order":0}]`)

	e.putTemplate(carePlanType, []plans.Field{{ID: "new", Type: shortTextType, Label: "New field"}}, http.StatusOK)

	out := e.template(carePlanType)
	if len(out.Fields) != 1 || out.Fields[0].ID != "new" {
		t.Fatalf("fields = %+v, want only the replacement field", out.Fields)
	}
}

// planTemplateAudit reads the plan_template activity rows for practiceID,
// oldest first: who wrote each, and its before/after diff.
func planTemplateAudit(t *testing.T, db *testdb.DB, practiceID string) []struct {
	actor string
	diff  map[string][]plans.Field
} {
	t.Helper()
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT actor_staff_id, diff FROM activity
		 WHERE practice_id = $1 AND subject_kind = 'plan_template' AND action = 'updated'
		 ORDER BY created_at, id`, practiceID)
	if err != nil {
		t.Fatalf("read audit rows: %v", err)
	}
	defer rows.Close()
	var out []struct {
		actor string
		diff  map[string][]plans.Field
	}
	for rows.Next() {
		var actor string
		var raw []byte
		if err := rows.Scan(&actor, &raw); err != nil {
			t.Fatalf("scan audit row: %v", err)
		}
		var diff map[string][]plans.Field
		if err := json.Unmarshal(raw, &diff); err != nil {
			t.Fatalf("decode audit diff: %v", err)
		}
		out = append(out, struct {
			actor string
			diff  map[string][]plans.Field
		}{actor, diff})
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read audit rows: %v", err)
	}
	return out
}

// TestPutTemplateHandler_WritesOneAuditRowPerRealChange proves a change to
// a Plan Template records who made it and the list before and after
// (#2193: each question page saves one question's change), and that
// re-saving an unchanged list records nothing.
func TestPutTemplateHandler_WritesOneAuditRowPerRealChange(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.seedTemplate(carePlanType, `[{"id":"old","type":"short_text","label":"Old field","order":0}]`)
	changed := []plans.Field{
		{ID: "old", Type: shortTextType, Label: "Old field"},
		{ID: "added", Type: shortTextType, Label: "Hospital transfer"},
	}

	for range 2 {
		e.putTemplate(carePlanType, changed, http.StatusOK)
	}

	audit := planTemplateAudit(t, e.DB, e.Practice.ID)
	if len(audit) != 1 {
		t.Fatalf("audit rows = %d, want 1 (the second PUT changed nothing)", len(audit))
	}
	if audit[0].actor != e.Staff.ID {
		t.Errorf("audit actor = %q, want %q", audit[0].actor, e.Staff.ID)
	}
	if before, after := audit[0].diff["before"], audit[0].diff["after"]; len(before) != 1 || len(after) != 2 || after[1].ID != "added" {
		t.Errorf("audit diff = %+v, want one field before and the added one after", audit[0].diff)
	}
}

// TestPutTemplateHandler_AuditsAFirstSaveAgainstAnEmptyList proves a
// Practice with no row for a plan type yet records its first save, with
// an empty list before it.
func TestPutTemplateHandler_AuditsAFirstSaveAgainstAnEmptyList(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))

	e.putTemplate(birthPlanType, []plans.Field{{ID: "first", Type: shortTextType, Label: "First"}}, http.StatusOK)

	audit := planTemplateAudit(t, e.DB, e.Practice.ID)
	if len(audit) != 1 || len(audit[0].diff["before"]) != 0 || len(audit[0].diff["after"]) != 1 {
		t.Fatalf("audit = %+v, want one row from an empty list to one field", audit)
	}
}
