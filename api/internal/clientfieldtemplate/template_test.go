package clientfieldtemplate_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/clientfieldtemplate"
	"doula-cloud/api/internal/testdb"
)

// TestGetHandler_EmptyByDefault proves a new Practice's Client Field
// Template GET returns an empty list, not a 404 -- the departure from
// plans.GetTemplateHandler that ADR-0017's "empty by default" state
// requires.
func TestGetHandler_EmptyByDefault(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-empty-default"
	practiceID := seedDoula(t, db, uid)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := getTemplate(t, srv, session, practiceID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out clientfieldtemplate.TemplateResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.Fields) != 0 {
		t.Fatalf("fields = %+v, want empty", out.Fields)
	}
}

// TestGetHandler_AnyStaffAllowed proves a Doula (non-Owner, non-Admin) can
// read the template -- only PUT is Owner/Admin-gated.
func TestGetHandler_AnyStaffAllowed(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-any-staff"
	practiceID := seedDoula(t, db, uid)
	seedTemplate(t, db, practiceID, `[{"id":"f1","type":"short_text","label":"Intake note","order":0,"archived":false}]`)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := getTemplate(t, srv, session, practiceID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out clientfieldtemplate.TemplateResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.Fields) != 1 || out.Fields[0].ID != "f1" {
		t.Fatalf("fields = %+v, want one field with id f1", out.Fields)
	}
}

// TestPutHandler_DoulaForbidden proves AC1: a Doula cannot write the
// template, at the endpoint seam.
func TestPutHandler_DoulaForbidden(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-doula-forbidden"
	practiceID := seedDoula(t, db, uid)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := putTemplate(t, srv, session, practiceID, clientfieldtemplate.TemplateResponse{Fields: []clientfieldtemplate.Field{
		{ID: "f1", Type: shortTextType, Label: testFieldLabel},
	}})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestPutHandler_AdminAllowed proves RequireOwnerOrAdmin's widened half:
// an Admin, not only an Owner, may write the template.
func TestPutHandler_AdminAllowed(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-admin-allowed"
	practiceID := seedAdmin(t, db, uid)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := putTemplate(t, srv, session, practiceID, clientfieldtemplate.TemplateResponse{Fields: []clientfieldtemplate.Field{
		{ID: "f1", Type: shortTextType, Label: testFieldLabel},
	}})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestPutHandler_InvalidBody(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-invalid-body"
	practiceID := seedOwner(t, db, uid)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := putTemplateRaw(t, srv, session, practiceID, []byte("not json"))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
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
		{"missing id", []clientfieldtemplate.Field{{ID: "", Type: shortTextType, Label: testFieldLabel}}},
		{"duplicate id", []clientfieldtemplate.Field{
			{ID: "f1", Type: shortTextType, Label: testFieldLabel},
			{ID: "f1", Type: shortTextType, Label: "Again"},
		}},
		{"missing label", []clientfieldtemplate.Field{{ID: "f1", Type: shortTextType, Label: ""}}},
		{"select with no options", []clientfieldtemplate.Field{{ID: "f1", Type: "single_select", Label: "Pick one"}}},
		{"select with blank option", []clientfieldtemplate.Field{{ID: "f1", Type: "multi_select", Label: "Pick some", Options: []string{"A", ""}}}},
		{"non-select with options", []clientfieldtemplate.Field{{ID: "f1", Type: "checkbox", Label: "Agree", Options: []string{"yes"}}}},
		{"shadows email", []clientfieldtemplate.Field{{ID: "f1", Type: shortTextType, Label: "Email"}}},
		{"shadows date of birth, case/whitespace-insensitive", []clientfieldtemplate.Field{{ID: "f1", Type: shortTextType, Label: "  Date Of Birth  "}}},
		{"shadows phone", []clientfieldtemplate.Field{{ID: "f1", Type: shortTextType, Label: "Phone Number"}}},
	}

	db := testdb.New(t)
	const uid = "put-validation"
	practiceID := seedOwner(t, db, uid)
	srv, session := newServer(t, db, uid)
	defer srv.Close()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := putTemplate(t, srv, session, practiceID, clientfieldtemplate.TemplateResponse{Fields: tc.fields})
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
			}
		})
	}
}

// TestPutHandler_ShadowStructuralAllowsANonMatchingSynonym proves the
// block is an exact match on the trimmed/lowercased label, not a
// substring match -- "Emergency contact phone" must stay legal even
// though it contains "phone".
func TestPutHandler_ShadowStructuralAllowsANonMatchingSynonym(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-shadow-non-match"
	practiceID := seedOwner(t, db, uid)
	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := putTemplate(t, srv, session, practiceID, clientfieldtemplate.TemplateResponse{Fields: []clientfieldtemplate.Field{
		{ID: "f1", Type: shortTextType, Label: "Emergency contact phone"},
	}})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// TestPutHandler_ExistingFieldCannotBeRemoved proves ADR-0017's
// archive-never-delete rule: a PUT whose field list drops an id the
// Practice already has is refused outright, rather than silently
// deleting the field and orphaning any Client value stored under it.
func TestPutHandler_ExistingFieldCannotBeRemoved(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-cannot-remove"
	practiceID := seedOwner(t, db, uid)
	seedTemplate(t, db, practiceID, `[{"id":"f1","type":"short_text","label":"Intake note","order":0,"archived":false}]`)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := putTemplate(t, srv, session, practiceID, clientfieldtemplate.TemplateResponse{Fields: []clientfieldtemplate.Field{
		{ID: "f2", Type: shortTextType, Label: "Something else"},
	}})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPutHandler_ExistingFieldTypeCannotChange proves live-read values
// can never be reinterpreted under a Practice's feet: once a field id
// exists, its Type is locked, even though its Label and Options stay
// editable.
func TestPutHandler_ExistingFieldTypeCannotChange(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-type-locked"
	practiceID := seedOwner(t, db, uid)
	seedTemplate(t, db, practiceID, `[{"id":"f1","type":"short_text","label":"Intake note","order":0,"archived":false}]`)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := putTemplate(t, srv, session, practiceID, clientfieldtemplate.TemplateResponse{Fields: []clientfieldtemplate.Field{
		{ID: "f1", Type: "long_text", Label: "Intake note"},
	}})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPutHandler_ArchiveThenUnarchiveRoundTrips proves AC2: archiving a
// field preserves it (never deletes it), and un-archiving restores it to
// the form -- and AC1's "reorder" half, since the round trip also sends
// fields out of array order and checks the server recomputes Order from
// position rather than trusting a client-sent value.
func TestPutHandler_ArchiveThenUnarchiveRoundTrips(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-archive-round-trip"
	practiceID := seedOwner(t, db, uid)
	srv, session := newServer(t, db, uid)
	defer srv.Close()

	create := putTemplate(t, srv, session, practiceID, clientfieldtemplate.TemplateResponse{Fields: []clientfieldtemplate.Field{
		{ID: "f1", Type: shortTextType, Label: firstFieldLabel, Order: 9},
		{ID: "f2", Type: shortTextType, Label: secondFieldLabel, Order: 1},
	}})
	_ = create.Body.Close()
	if create.StatusCode != http.StatusOK {
		t.Fatalf("create status = %d, want %d", create.StatusCode, http.StatusOK)
	}

	archive := putTemplate(t, srv, session, practiceID, clientfieldtemplate.TemplateResponse{Fields: []clientfieldtemplate.Field{
		{ID: "f2", Type: shortTextType, Label: secondFieldLabel},
		{ID: "f1", Type: shortTextType, Label: firstFieldLabel, Archived: true},
	}})
	defer archive.Body.Close()
	if archive.StatusCode != http.StatusOK {
		t.Fatalf("archive status = %d, want %d", archive.StatusCode, http.StatusOK)
	}
	var archived clientfieldtemplate.TemplateResponse
	if err := json.NewDecoder(archive.Body).Decode(&archived); err != nil {
		t.Fatalf("decode archive response: %v", err)
	}
	if len(archived.Fields) != 2 || archived.Fields[0].ID != "f2" || archived.Fields[0].Order != 0 {
		t.Fatalf("fields after archive = %+v, want f2 first at order 0", archived.Fields)
	}
	if archived.Fields[1].ID != "f1" || !archived.Fields[1].Archived || archived.Fields[1].Order != 1 {
		t.Fatalf("fields after archive = %+v, want f1 archived at order 1", archived.Fields)
	}

	unarchive := putTemplate(t, srv, session, practiceID, clientfieldtemplate.TemplateResponse{Fields: []clientfieldtemplate.Field{
		{ID: "f1", Type: shortTextType, Label: firstFieldLabel, Archived: false},
		{ID: "f2", Type: shortTextType, Label: secondFieldLabel},
	}})
	defer unarchive.Body.Close()
	var restored clientfieldtemplate.TemplateResponse
	if err := json.NewDecoder(unarchive.Body).Decode(&restored); err != nil {
		t.Fatalf("decode unarchive response: %v", err)
	}
	if restored.Fields[0].ID != "f1" || restored.Fields[0].Archived {
		t.Fatalf("fields after unarchive = %+v, want f1 restored, not archived", restored.Fields)
	}
}

// TestPutHandler_WritesOneAuditRowPerRealChange proves AC4: every change
// to the field list writes an audit row naming the actor, and a no-op
// PUT (re-saving an unchanged list) writes none.
func TestPutHandler_WritesOneAuditRowPerRealChange(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-audit-trail"
	practiceID := seedOwner(t, db, uid)
	var staffID string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT id FROM staff WHERE identity_uid = $1`, uid).Scan(&staffID); err != nil {
		t.Fatalf("read seeded staff id: %v", err)
	}

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	first := putTemplate(t, srv, session, practiceID, clientfieldtemplate.TemplateResponse{Fields: []clientfieldtemplate.Field{
		{ID: "f1", Type: shortTextType, Label: testFieldLabel},
	}})
	_ = first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first put status = %d, want %d", first.StatusCode, http.StatusOK)
	}
	if got := auditEventCount(t, db, practiceID); got != 1 {
		t.Fatalf("audit events after first put = %d, want 1", got)
	}
	if got := lastAuditActor(t, db, practiceID); got != staffID {
		t.Fatalf("audit actor = %q, want %q", got, staffID)
	}

	noop := putTemplate(t, srv, session, practiceID, clientfieldtemplate.TemplateResponse{Fields: []clientfieldtemplate.Field{
		{ID: "f1", Type: shortTextType, Label: testFieldLabel},
	}})
	_ = noop.Body.Close()
	if got := auditEventCount(t, db, practiceID); got != 1 {
		t.Fatalf("audit events after no-op put = %d, want still 1", got)
	}

	second := putTemplate(t, srv, session, practiceID, clientfieldtemplate.TemplateResponse{Fields: []clientfieldtemplate.Field{
		{ID: "f1", Type: shortTextType, Label: "Renamed", Archived: true},
	}})
	_ = second.Body.Close()
	if got := auditEventCount(t, db, practiceID); got != 2 {
		t.Fatalf("audit events after second real change = %d, want 2", got)
	}
}
