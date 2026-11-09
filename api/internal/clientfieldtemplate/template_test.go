package clientfieldtemplate_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/clientfieldtemplate"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
)

// TestGetHandler_EmptyByDefault proves a new Practice's Client Field
// Template GET returns an empty list, not a 404 -- the departure from
// plans.GetTemplateHandler that ADR-0017's "empty by default" state
// requires.
func TestGetHandler_EmptyByDefault(t *testing.T) {
	e := newEnv(t)

	if out := e.template(); len(out.Fields) != 0 {
		t.Fatalf("fields = %+v, want empty", out.Fields)
	}
}

// TestGetHandler_AnyStaffAllowed proves a Doula (non-Owner, non-Admin) can
// read the template -- only PUT is Owner/Admin-gated.
func TestGetHandler_AnyStaffAllowed(t *testing.T) {
	e := newEnv(t)
	e.seedIntakeNote()

	if out := e.template(); len(out.Fields) != 1 || out.Fields[0].ID != "f1" {
		t.Fatalf("fields = %+v, want one field with id f1", out.Fields)
	}
}

// TestPutHandler_RoleAndBodyBoundaries table-drives who may write the
// template and what the endpoint takes: AC1's refusal of a Doula at the
// endpoint seam, RequireOwnerOrAdmin's widened half (an Admin, not only
// an Owner), a body that is not JSON, and the exact-match shadow block
// -- "Emergency contact phone" must stay legal even though it contains
// "phone".
func TestPutHandler_RoleAndBodyBoundaries(t *testing.T) {
	someField := reqtest.JSON(clientfieldtemplate.TemplateResponse{Fields: []clientfieldtemplate.Field{shortText("f1", testFieldLabel)}})
	cases := []struct {
		name  string
		staff fixture.StaffOption
		body  reqtest.Option
		want  int
	}{
		{"a Doula is forbidden", fixture.Doula(), someField, http.StatusForbidden},
		{"an Admin is allowed", fixture.Admin(), someField, http.StatusOK},
		{"a body that is not JSON", fixture.Owner(), reqtest.RawJSON([]byte("not json")), http.StatusBadRequest},
		{"a label that only contains a structural field's name", fixture.Owner(),
			reqtest.JSON(clientfieldtemplate.TemplateResponse{Fields: []clientfieldtemplate.Field{shortText("f1", "Emergency contact phone")}}), http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			newEnv(t, tc.staff).putBody(tc.body, tc.want)
		})
	}
}

// TestPutHandler_ValidationRejections table-drives every normalizeFields
// error branch that doesn't depend on a previous field list: the
// ADR-0001 palette checks inherited from plans/template.go, plus the
// shadow-structural-field block ADR-0017 adds.
func TestPutHandler_ValidationRejections(t *testing.T) {
	cases := []struct {
		name   string
		fields []clientfieldtemplate.Field
	}{
		{"unknown field type", []clientfieldtemplate.Field{{ID: "f1", Type: "essay", Label: testFieldLabel}}},
		{"missing id", []clientfieldtemplate.Field{shortText("", testFieldLabel)}},
		{"duplicate id", []clientfieldtemplate.Field{shortText("f1", testFieldLabel), shortText("f1", "Again")}},
		{"missing label", []clientfieldtemplate.Field{shortText("f1", "")}},
		{"select with no options", []clientfieldtemplate.Field{{ID: "f1", Type: "single_select", Label: "Pick one"}}},
		{"select with blank option", []clientfieldtemplate.Field{{ID: "f1", Type: "multi_select", Label: "Pick some", Options: []string{"A", ""}}}},
		{"non-select with options", []clientfieldtemplate.Field{{ID: "f1", Type: "checkbox", Label: "Agree", Options: []string{"yes"}}}},
		{"shadows email", []clientfieldtemplate.Field{shortText("f1", "Email")}},
		{"shadows date of birth, case/whitespace-insensitive", []clientfieldtemplate.Field{shortText("f1", "  Date Of Birth  ")}},
		{"shadows phone", []clientfieldtemplate.Field{shortText("f1", "Phone Number")}},
	}

	e := newEnv(t, fixture.Owner())
	for _, tc := range cases {
		t.Run(tc.name, func(*testing.T) {
			e.putFields(http.StatusBadRequest, tc.fields...)
		})
	}
}

// TestPutHandler_ExistingFieldsAreLocked proves ADR-0017's
// archive-never-delete rule and the lock on a field's Type: a PUT whose
// field list drops an id the Practice already has is refused outright,
// rather than silently deleting the field and orphaning any Client value
// stored under it, and once a field id exists its Type can never change
// (so live-read values are never reinterpreted under a Practice's feet),
// even though its Label and Options stay editable.
func TestPutHandler_ExistingFieldsAreLocked(t *testing.T) {
	cases := []struct {
		name   string
		fields []clientfieldtemplate.Field
	}{
		{"an existing field cannot be removed", []clientfieldtemplate.Field{shortText("f2", "Something else")}},
		{"an existing field's type cannot change", []clientfieldtemplate.Field{{ID: "f1", Type: "long_text", Label: "Intake note"}}},
	}

	e := newEnv(t, fixture.Owner())
	e.seedIntakeNote()
	for _, tc := range cases {
		t.Run(tc.name, func(*testing.T) {
			e.putFields(http.StatusBadRequest, tc.fields...)
		})
	}
}

// TestPutHandler_ArchiveThenUnarchiveRoundTrips proves AC2: archiving a
// field preserves it (never deletes it), and un-archiving restores it to
// the form -- and AC1's "reorder" half, since the round trip also sends
// fields out of array order and checks the server recomputes Order from
// position rather than trusting a client-sent value.
func TestPutHandler_ArchiveThenUnarchiveRoundTrips(t *testing.T) {
	e := newEnv(t, fixture.Owner())

	e.putFieldsOK(
		clientfieldtemplate.Field{ID: "f1", Type: shortTextType, Label: firstFieldLabel, Order: 9},
		clientfieldtemplate.Field{ID: "f2", Type: shortTextType, Label: secondFieldLabel, Order: 1},
	)

	archived := e.putFieldsOK(
		shortText("f2", secondFieldLabel),
		clientfieldtemplate.Field{ID: "f1", Type: shortTextType, Label: firstFieldLabel, Archived: true},
	)
	if len(archived.Fields) != 2 || archived.Fields[0].ID != "f2" || archived.Fields[0].Order != 0 {
		t.Fatalf("fields after archive = %+v, want f2 first at order 0", archived.Fields)
	}
	if archived.Fields[1].ID != "f1" || !archived.Fields[1].Archived || archived.Fields[1].Order != 1 {
		t.Fatalf("fields after archive = %+v, want f1 archived at order 1", archived.Fields)
	}

	restored := e.putFieldsOK(shortText("f1", firstFieldLabel), shortText("f2", secondFieldLabel))
	if restored.Fields[0].ID != "f1" || restored.Fields[0].Archived {
		t.Fatalf("fields after unarchive = %+v, want f1 restored, not archived", restored.Fields)
	}
}

// TestPutHandler_WritesOneAuditRowPerRealChange proves AC4: every change
// to the field list writes an audit row naming the actor, and a no-op
// PUT (re-saving an unchanged list) writes none.
func TestPutHandler_WritesOneAuditRowPerRealChange(t *testing.T) {
	e := newEnv(t, fixture.Owner())

	e.putFields(http.StatusOK, shortText("f1", testFieldLabel))
	if got := e.auditEventCount(); got != 1 {
		t.Fatalf("audit events after first put = %d, want 1", got)
	}
	if got := e.lastAuditActor(); got != e.Staff.ID {
		t.Fatalf("audit actor = %q, want %q", got, e.Staff.ID)
	}

	e.putFields(http.StatusOK, shortText("f1", testFieldLabel))
	if got := e.auditEventCount(); got != 1 {
		t.Fatalf("audit events after no-op put = %d, want still 1", got)
	}

	e.putFields(http.StatusOK, clientfieldtemplate.Field{ID: "f1", Type: shortTextType, Label: "Renamed", Archived: true})
	if got := e.auditEventCount(); got != 2 {
		t.Fatalf("audit events after second real change = %d, want 2", got)
	}
}
