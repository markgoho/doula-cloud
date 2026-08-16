package contracts_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// fakeVerifier is a test double for authn.Verifier -- real Identity
// Platform tokens can't be minted without a live GCP project. Mirrors
// staffauth_test's and plans_test's own fakeVerifier; kept package-local
// since Go test doubles aren't exported across packages.
type fakeVerifier struct{ uid string }

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

// seedTemplate seeds a Contract Template row directly (bypassing the
// handlers under test).
func seedTemplate(t *testing.T, db *testdb.DB, practiceID, prose string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO contract_templates (practice_id, prose) VALUES ($1, $2)`,
		practiceID, prose,
	); err != nil {
		t.Fatalf("seed template: %v", err)
	}
}

// seedEngagement inserts a Client and an Engagement linking them to
// practiceID, using the superuser Admin connection -- mirrors
// plans/template_test.go's seedEngagement.
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

// seedContract seeds a Contract row directly (bypassing the handlers
// under test), with an explicit status so tests can exercise a
// non-'draft' Contract that PutContractHandler must reject -- a state no
// endpoint in this ticket can reach on its own. merge_field_values is
// left at its NOT NULL DEFAULT '{}'::jsonb, matching every caller.
func seedContract(t *testing.T, db *testdb.DB, engagementID, status, prose string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO contracts (engagement_id, status, prose) VALUES ($1, $2::contract_status, $3)`,
		engagementID, status, prose,
	); err != nil {
		t.Fatalf("seed contract: %v", err)
	}
}

func newContractServer(verifier fakeVerifier, db *testdb.DB) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("GET /practices/{practiceId}/contract-template",
		staffauth.Middleware(verifier, db.App)(contracts.GetTemplateHandler()))
	mux.Handle("PUT /practices/{practiceId}/contract-template",
		staffauth.Middleware(verifier, db.App)(contracts.PutTemplateHandler()))
	mux.Handle("POST /practices/{practiceId}/engagements/{engagementId}/contract",
		staffauth.Middleware(verifier, db.App)(contracts.PostContractHandler()))
	mux.Handle("GET /practices/{practiceId}/engagements/{engagementId}/contract",
		staffauth.Middleware(verifier, db.App)(contracts.GetContractHandler()))
	mux.Handle("PUT /practices/{practiceId}/engagements/{engagementId}/contract",
		staffauth.Middleware(verifier, db.App)(contracts.PutContractHandler()))
	return httptest.NewServer(mux)
}

func getTemplate(t *testing.T, srv *httptest.Server, practiceID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/practices/"+practiceID+"/contract-template", nil)
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

func putTemplateRaw(t *testing.T, srv *httptest.Server, practiceID string, body []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, srv.URL+"/practices/"+practiceID+"/contract-template", bytes.NewReader(body))
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

func putTemplate(t *testing.T, srv *httptest.Server, practiceID string, body contracts.TemplateResponse) *http.Response {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return putTemplateRaw(t, srv, practiceID, payload)
}

func TestGetTemplateHandler_NotFound(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-not-found"
	practiceID := seedMember(t, db, uid)

	srv := newContractServer(fakeVerifier{uid: uid}, db)
	defer srv.Close()

	resp := getTemplate(t, srv, practiceID)
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
	seedTemplate(t, db, practiceID, "Some prose with {{client_name}}")

	srv := newContractServer(fakeVerifier{uid: uid}, db)
	defer srv.Close()

	resp := getTemplate(t, srv, practiceID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var out contracts.TemplateResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Prose != "Some prose with {{client_name}}" {
		t.Fatalf("prose = %q, want the seeded prose", out.Prose)
	}
}

func TestPutTemplateHandler_NonOwnerForbidden(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-non-owner"
	practiceID := seedMember(t, db, uid)

	srv := newContractServer(fakeVerifier{uid: uid}, db)
	defer srv.Close()

	resp := putTemplate(t, srv, practiceID, contracts.TemplateResponse{Prose: "Some prose"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestPutTemplateHandler_InvalidBody(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-invalid-body"
	practiceID := seedOwner(t, db, uid)

	srv := newContractServer(fakeVerifier{uid: uid}, db)
	defer srv.Close()

	resp := putTemplateRaw(t, srv, practiceID, []byte("not json"))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPutTemplateHandler_BlankProseRejected table-drives an empty and a
// whitespace-only prose, both of which normalize to "" after trimming.
func TestPutTemplateHandler_BlankProseRejected(t *testing.T) {
	cases := []struct {
		name  string
		prose string
	}{
		{"empty", ""},
		{"whitespace only", "   \n\t  "},
	}

	db := testdb.New(t)
	const uid = "put-blank-prose"
	practiceID := seedOwner(t, db, uid)
	srv := newContractServer(fakeVerifier{uid: uid}, db)
	defer srv.Close()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := putTemplate(t, srv, practiceID, contracts.TemplateResponse{Prose: tc.prose})
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
			}
		})
	}
}

// TestPutTemplateHandler_Success proves a full replace round-trips through
// GET, and that surrounding whitespace is trimmed.
func TestPutTemplateHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-success"
	practiceID := seedOwner(t, db, uid)

	srv := newContractServer(fakeVerifier{uid: uid}, db)
	defer srv.Close()

	putResp := putTemplate(t, srv, practiceID, contracts.TemplateResponse{Prose: "  Agreement with {{client_name}}  "})
	defer putResp.Body.Close()

	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status = %d, want %d", putResp.StatusCode, http.StatusOK)
	}

	var putOut contracts.TemplateResponse
	if err := json.NewDecoder(putResp.Body).Decode(&putOut); err != nil {
		t.Fatalf("decode PUT response: %v", err)
	}
	if putOut.Prose != "Agreement with {{client_name}}" {
		t.Fatalf("PUT prose = %q, want trimmed", putOut.Prose)
	}

	getResp := getTemplate(t, srv, practiceID)
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d, want %d", getResp.StatusCode, http.StatusOK)
	}
	var getOut contracts.TemplateResponse
	if err := json.NewDecoder(getResp.Body).Decode(&getOut); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	if getOut.Prose != "Agreement with {{client_name}}" {
		t.Fatalf("GET prose after PUT = %q, want the just-written prose", getOut.Prose)
	}
}

// TestPutTemplateHandler_ReplacesExistingRow proves PUT overwrites a row
// seeded by signup (or a prior PUT) rather than conflicting with it -- the
// ON CONFLICT DO UPDATE path.
func TestPutTemplateHandler_ReplacesExistingRow(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-replace"
	practiceID := seedOwner(t, db, uid)
	seedTemplate(t, db, practiceID, "Old prose")

	srv := newContractServer(fakeVerifier{uid: uid}, db)
	defer srv.Close()

	resp := putTemplate(t, srv, practiceID, contracts.TemplateResponse{Prose: "New prose"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	getResp := getTemplate(t, srv, practiceID)
	defer getResp.Body.Close()
	var out contracts.TemplateResponse
	if err := json.NewDecoder(getResp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Prose != "New prose" {
		t.Fatalf("prose = %q, want only the replacement prose", out.Prose)
	}
}
