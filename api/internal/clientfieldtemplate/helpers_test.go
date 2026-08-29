package clientfieldtemplate_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clientfieldtemplate"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

const (
	testFieldLabel   = "Some Field"
	shortTextType    = "short_text"
	firstFieldLabel  = "First"
	secondFieldLabel = "Second"
)

func seedPractice(t *testing.T, db *testdb.DB) (practiceID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(), `INSERT INTO practices (name) VALUES ('Test Practice') RETURNING id`).Scan(&practiceID); err != nil {
		t.Fatalf("seed practice: %v", err)
	}
	return practiceID
}

func seedStaff(t *testing.T, db *testdb.DB, identityUID string) (staffID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email, work_state) VALUES ($1, 'Test Staff', $1 || '@example.com', 'NY') RETURNING id`,
		identityUID,
	).Scan(&staffID); err != nil {
		t.Fatalf("seed staff %q: %v", identityUID, err)
	}
	return staffID
}

func seedMembership(t *testing.T, db *testdb.DB, practiceID, staffID, roles string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type) VALUES ($1, $2, $3::practice_role[], 'employee')`,
		practiceID, staffID, roles,
	); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
}

// seedDoula seeds a Practice and a Staff member holding only the doula
// role there -- the role PutHandler must refuse.
func seedDoula(t *testing.T, db *testdb.DB, identityUID string) (practiceID string) {
	t.Helper()
	practiceID = seedPractice(t, db)
	staffID := seedStaff(t, db, identityUID)
	seedMembership(t, db, practiceID, staffID, "{doula}")
	return practiceID
}

// seedOwner seeds a Practice and a Staff member holding the owner role.
func seedOwner(t *testing.T, db *testdb.DB, identityUID string) (practiceID string) {
	t.Helper()
	practiceID = seedPractice(t, db)
	staffID := seedStaff(t, db, identityUID)
	seedMembership(t, db, practiceID, staffID, "{owner}")
	return practiceID
}

// seedAdmin seeds a Practice and a Staff member holding the admin role --
// the widened half of RequireOwnerOrAdmin plans/template.go's Owner-only
// PutTemplateHandler doesn't need to prove.
func seedAdmin(t *testing.T, db *testdb.DB, identityUID string) (practiceID string) {
	t.Helper()
	practiceID = seedPractice(t, db)
	staffID := seedStaff(t, db, identityUID)
	seedMembership(t, db, practiceID, staffID, "{admin}")
	return practiceID
}

// seedTemplate seeds a client_field_templates row directly (bypassing the
// handlers under test).
func seedTemplate(t *testing.T, db *testdb.DB, practiceID, fieldsJSON string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO client_field_templates (practice_id, fields) VALUES ($1, $2)`,
		practiceID, fieldsJSON,
	); err != nil {
		t.Fatalf("seed template: %v", err)
	}
}

// auditEventCount counts activity rows (subject_kind
// 'client_field_template', ADR-0022) for practiceID.
func auditEventCount(t *testing.T, db *testdb.DB, practiceID string) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM activity WHERE practice_id = $1 AND subject_kind = 'client_field_template'`, practiceID,
	).Scan(&count); err != nil {
		t.Fatalf("count audit events: %v", err)
	}
	return count
}

// lastAuditActor reads the actor_staff_id of the most recent activity
// row (subject_kind 'client_field_template') for practiceID.
func lastAuditActor(t *testing.T, db *testdb.DB, practiceID string) string {
	t.Helper()
	var actorID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT actor_staff_id FROM activity
		 WHERE practice_id = $1 AND subject_kind = 'client_field_template' ORDER BY created_at DESC LIMIT 1`,
		practiceID,
	).Scan(&actorID); err != nil {
		t.Fatalf("read last audit actor: %v", err)
	}
	return actorID
}

func newServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /practices/{practiceId}/client-field-template",
		staffauth.Middleware(db.App)(clientfieldtemplate.GetHandler()))
	mux.Handle("PUT /practices/{practiceId}/client-field-template",
		staffauth.Middleware(db.App)(clientfieldtemplate.PutHandler()))
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func templatePath(practiceID string) string {
	return "/practices/" + practiceID + "/client-field-template"
}

func getTemplate(t *testing.T, srv *httptest.Server, session, practiceID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+templatePath(practiceID), nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func putTemplateRaw(t *testing.T, srv *httptest.Server, session, practiceID string, body []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, srv.URL+templatePath(practiceID), bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func putTemplate(t *testing.T, srv *httptest.Server, session, practiceID string, body clientfieldtemplate.TemplateResponse) *http.Response {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return putTemplateRaw(t, srv, session, practiceID, payload)
}
