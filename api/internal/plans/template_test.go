package plans_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/plans"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// fakeVerifier is a test double for authn.Verifier -- real Identity
// Platform tokens can't be minted without a live GCP project. Mirrors
// staffauth_test's own fakeVerifier; kept package-local since Go test
// doubles aren't exported across packages.
type fakeVerifier struct{ uid string }

// testFieldLabel is a placeholder Label used across many test cases that
// don't care about its value, just that it's non-empty.
const testFieldLabel = "Name"

const shortTextType = "short_text"

const carePlanType = "care_plan"
const birthPlanType = "birth_plan"

func (f fakeVerifier) VerifyIDToken(_ context.Context, _ string) (*authn.VerifiedToken, error) {
	return &authn.VerifiedToken{UID: f.uid}, nil
}

func seedPractice(t *testing.T, db *testdb.DB, name string) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(), `INSERT INTO practices (name) VALUES ($1) RETURNING id`, name).Scan(&id); err != nil {
		t.Fatalf("seed practice %q: %v", name, err)
	}
	return id
}

func seedStaff(t *testing.T, db *testdb.DB, identityUID string) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email) VALUES ($1, 'Test Staff', 'staff@example.com') RETURNING id`,
		identityUID,
	).Scan(&id); err != nil {
		t.Fatalf("seed staff %q: %v", identityUID, err)
	}
	return id
}

func seedMembership(t *testing.T, db *testdb.DB, practiceID, staffID string, roles string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles) VALUES ($1, $2, $3::practice_role[])`,
		practiceID, staffID, roles,
	); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
}

// seedMember seeds a Practice and a Staff member holding a doula (non-Owner)
// role there.
func seedMember(t *testing.T, db *testdb.DB, identityUID string) (practiceID string) {
	t.Helper()
	practiceID = seedPractice(t, db, "Test Practice")
	staffID := seedStaff(t, db, identityUID)
	seedMembership(t, db, practiceID, staffID, "{doula}")
	return practiceID
}

// seedOwner seeds a Practice and a Staff member holding the 'owner' role
// there -- the only role PutTemplateHandler accepts.
func seedOwner(t *testing.T, db *testdb.DB, identityUID string) (practiceID string) {
	t.Helper()
	practiceID = seedPractice(t, db, "Test Practice")
	staffID := seedStaff(t, db, identityUID)
	seedMembership(t, db, practiceID, staffID, "{owner}")
	return practiceID
}

// seedTemplate seeds a Plan Template row directly (bypassing the handlers
// under test).
func seedTemplate(t *testing.T, db *testdb.DB, practiceID, planType, fieldsJSON string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO plan_templates (practice_id, plan_type, fields) VALUES ($1, $2, $3)`,
		practiceID, planType, fieldsJSON,
	); err != nil {
		t.Fatalf("seed template: %v", err)
	}
}

// seedEngagement inserts a Client and an Engagement linking them to
// practiceID, using the superuser Admin connection -- mirrors
// visit/helpers_test.go's seedEngagement.
func seedEngagement(t *testing.T, db *testdb.DB, practiceID string) (engagementID string) {
	t.Helper()
	var clientID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (name, email) VALUES ('Test Client', 'client@example.com') RETURNING id`,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id) VALUES ($1, $2) RETURNING id`,
		clientID, practiceID,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}
	return engagementID
}

// seedInstance seeds a Plan Instance row directly (bypassing the handlers
// under test).
func seedInstance(t *testing.T, db *testdb.DB, engagementID, planType, fieldsJSON, answersJSON string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO plan_instances (engagement_id, plan_type, fields, answers) VALUES ($1, $2, $3, $4)`,
		engagementID, planType, fieldsJSON, answersJSON,
	); err != nil {
		t.Fatalf("seed instance: %v", err)
	}
}

func newPlanServer(verifier fakeVerifier, db *testdb.DB) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("GET /practices/{practiceId}/plan-templates/{planType}",
		staffauth.Middleware(verifier, db.App)(plans.GetTemplateHandler()))
	mux.Handle("PUT /practices/{practiceId}/plan-templates/{planType}",
		staffauth.Middleware(verifier, db.App)(plans.PutTemplateHandler()))
	mux.Handle("POST /practices/{practiceId}/engagements/{engagementId}/plans/{planType}",
		staffauth.Middleware(verifier, db.App)(plans.PostInstanceHandler()))
	mux.Handle("GET /practices/{practiceId}/engagements/{engagementId}/plans/{planType}",
		staffauth.Middleware(verifier, db.App)(plans.GetInstanceHandler()))
	mux.Handle("PUT /practices/{practiceId}/engagements/{engagementId}/plans/{planType}",
		staffauth.Middleware(verifier, db.App)(plans.PutInstanceHandler()))
	return httptest.NewServer(mux)
}

func getTemplate(t *testing.T, srv *httptest.Server, practiceID, planType string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/practices/"+practiceID+"/plan-templates/"+planType, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer tok")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func putTemplateRaw(t *testing.T, srv *httptest.Server, practiceID, planType string, body []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, srv.URL+"/practices/"+practiceID+"/plan-templates/"+planType, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func putTemplate(t *testing.T, srv *httptest.Server, practiceID, planType string, body plans.TemplateResponse) *http.Response {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return putTemplateRaw(t, srv, practiceID, planType, payload)
}

func instancePath(practiceID, engagementID, planType string) string {
	return "/practices/" + practiceID + "/engagements/" + engagementID + "/plans/" + planType
}

func postInstance(t *testing.T, srv *httptest.Server, practiceID, engagementID, planType string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+instancePath(practiceID, engagementID, planType), nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer tok")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func getInstance(t *testing.T, srv *httptest.Server, practiceID, engagementID, planType string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+instancePath(practiceID, engagementID, planType), nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer tok")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func putInstanceRaw(t *testing.T, srv *httptest.Server, practiceID, engagementID, planType string, body []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, srv.URL+instancePath(practiceID, engagementID, planType), bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func putInstance(t *testing.T, srv *httptest.Server, practiceID, engagementID, planType string, body plans.PutInstanceRequest) *http.Response {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return putInstanceRaw(t, srv, practiceID, engagementID, planType, payload)
}

func TestGetTemplateHandler_UnknownPlanType(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-unknown-plan-type"
	practiceID := seedMember(t, db, uid)

	srv := newPlanServer(fakeVerifier{uid: uid}, db)
	defer srv.Close()

	resp := getTemplate(t, srv, practiceID, "not_a_plan_type")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestGetTemplateHandler_NotFound(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-not-found"
	practiceID := seedMember(t, db, uid)

	srv := newPlanServer(fakeVerifier{uid: uid}, db)
	defer srv.Close()

	resp := getTemplate(t, srv, practiceID, "care_plan")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestGetTemplateHandler_AnyMemberAllowed proves a non-Owner member (a
// doula) can read the template -- only PUT is Owner-gated.
func TestGetTemplateHandler_AnyMemberAllowed(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-any-member"
	practiceID := seedMember(t, db, uid)
	seedTemplate(t, db, practiceID, carePlanType, `[{"id":"f1","type":"short_text","label":"Name","order":0}]`)

	srv := newPlanServer(fakeVerifier{uid: uid}, db)
	defer srv.Close()

	resp := getTemplate(t, srv, practiceID, "care_plan")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var out plans.TemplateResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.PlanType != "care_plan" {
		t.Fatalf("planType = %q, want care_plan", out.PlanType)
	}
	if len(out.Fields) != 1 || out.Fields[0].ID != "f1" {
		t.Fatalf("fields = %+v, want one field with id f1", out.Fields)
	}
}

func TestPutTemplateHandler_NonOwnerForbidden(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-non-owner"
	practiceID := seedMember(t, db, uid)

	srv := newPlanServer(fakeVerifier{uid: uid}, db)
	defer srv.Close()

	resp := putTemplate(t, srv, practiceID, "care_plan", plans.TemplateResponse{Fields: []plans.Field{
		{ID: "f1", Type: shortTextType, Label: testFieldLabel},
	}})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestPutTemplateHandler_UnknownPlanType(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-unknown-plan-type"
	practiceID := seedOwner(t, db, uid)

	srv := newPlanServer(fakeVerifier{uid: uid}, db)
	defer srv.Close()

	resp := putTemplate(t, srv, practiceID, "not_a_plan_type", plans.TemplateResponse{Fields: []plans.Field{
		{ID: "f1", Type: shortTextType, Label: testFieldLabel},
	}})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestPutTemplateHandler_InvalidBody(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-invalid-body"
	practiceID := seedOwner(t, db, uid)

	srv := newPlanServer(fakeVerifier{uid: uid}, db)
	defer srv.Close()

	resp := putTemplateRaw(t, srv, practiceID, "care_plan", []byte("not json"))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPutTemplateHandler_ValidationRejections table-drives every
// normalizeFields error branch: unknown field type, missing id, duplicate
// id, missing label, a select field with no options, a select field with a
// blank option, and a non-select field carrying options.
func TestPutTemplateHandler_ValidationRejections(t *testing.T) {
	cases := []struct {
		name   string
		fields []plans.Field
	}{
		{"unknown field type", []plans.Field{{ID: "f1", Type: "essay", Label: testFieldLabel}}},
		{"missing id", []plans.Field{{ID: "", Type: shortTextType, Label: testFieldLabel}}},
		{"duplicate id", []plans.Field{
			{ID: "f1", Type: shortTextType, Label: testFieldLabel},
			{ID: "f1", Type: shortTextType, Label: "Name again"},
		}},
		{"missing label", []plans.Field{{ID: "f1", Type: shortTextType, Label: ""}}},
		{"select with no options", []plans.Field{{ID: "f1", Type: "single_select", Label: "Pick one"}}},
		{"select with blank option", []plans.Field{{ID: "f1", Type: "multi_select", Label: "Pick some", Options: []string{"A", ""}}}},
		{"non-select with options", []plans.Field{{ID: "f1", Type: "checkbox", Label: "Agree", Options: []string{"yes"}}}},
	}

	db := testdb.New(t)
	const uid = "put-validation"
	practiceID := seedOwner(t, db, uid)
	srv := newPlanServer(fakeVerifier{uid: uid}, db)
	defer srv.Close()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := putTemplate(t, srv, practiceID, "care_plan", plans.TemplateResponse{Fields: tc.fields})
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
			}
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

	db := testdb.New(t)
	const uid = "put-success"
	practiceID := seedOwner(t, db, uid)

	srv := newPlanServer(fakeVerifier{uid: uid}, db)
	defer srv.Close()

	putResp := putTemplate(t, srv, practiceID, birthPlanType, plans.TemplateResponse{Fields: []plans.Field{
		{ID: secondFieldID, Type: "checkbox", Label: "Consent", Order: 99},
		{ID: firstFieldID, Type: "single_select", Label: "Location", Options: []string{"Home", "Hospital"}, Order: 1},
	}})
	defer putResp.Body.Close()

	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status = %d, want %d", putResp.StatusCode, http.StatusOK)
	}

	var putOut plans.TemplateResponse
	if err := json.NewDecoder(putResp.Body).Decode(&putOut); err != nil {
		t.Fatalf("decode PUT response: %v", err)
	}
	if len(putOut.Fields) != 2 || putOut.Fields[0].ID != secondFieldID || putOut.Fields[0].Order != 0 || putOut.Fields[1].ID != firstFieldID || putOut.Fields[1].Order != 1 {
		t.Fatalf("PUT fields = %+v, want order recomputed from array position", putOut.Fields)
	}

	getResp := getTemplate(t, srv, practiceID, birthPlanType)
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d, want %d", getResp.StatusCode, http.StatusOK)
	}
	var getOut plans.TemplateResponse
	if err := json.NewDecoder(getResp.Body).Decode(&getOut); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	if len(getOut.Fields) != 2 || getOut.Fields[0].ID != secondFieldID || getOut.Fields[1].ID != firstFieldID {
		t.Fatalf("GET fields after PUT = %+v, want the just-written fields", getOut.Fields)
	}
}

// TestPutTemplateHandler_ReplacesExistingRow proves PUT overwrites a row
// seeded by signup (or a prior PUT) rather than conflicting with it -- the
// ON CONFLICT DO UPDATE path.
func TestPutTemplateHandler_ReplacesExistingRow(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-replace"
	practiceID := seedOwner(t, db, uid)
	seedTemplate(t, db, practiceID, carePlanType, `[{"id":"old","type":"short_text","label":"Old field","order":0}]`)

	srv := newPlanServer(fakeVerifier{uid: uid}, db)
	defer srv.Close()

	resp := putTemplate(t, srv, practiceID, "care_plan", plans.TemplateResponse{Fields: []plans.Field{
		{ID: "new", Type: shortTextType, Label: "New field"},
	}})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	getResp := getTemplate(t, srv, practiceID, "care_plan")
	defer getResp.Body.Close()
	var out plans.TemplateResponse
	if err := json.NewDecoder(getResp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.Fields) != 1 || out.Fields[0].ID != "new" {
		t.Fatalf("fields = %+v, want only the replacement field", out.Fields)
	}
}
