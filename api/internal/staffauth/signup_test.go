package staffauth_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/plans"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

const (
	testStaffEmail = "s@example.com"
	jamieEmail     = "jamie@example.com"
	jamieName      = "Jamie"

	// Shared across staffauth_test files: goconst flags repeated literals
	// package-wide, not just within one file.
	someUID            = "some-uid"
	inviteeIdentityUID = "invitee-identity"
	inviteeName        = "Invitee"
	ownerRole          = "owner"
	doulaRole          = "doula"
)

func newSignupServer(verifier fakeVerifier, db *testdb.DB) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /staff/signup", staffauth.SignupHandler(verifier, db.App))
	return httptest.NewServer(mux)
}

func postSignup(t *testing.T, srv *httptest.Server, token string, body any) *http.Response {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/staff/signup", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func TestSignupHandler_MissingToken(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(fakeVerifier{}, db)
	defer srv.Close()

	resp := postSignup(t, srv, "", staffauth.SignupRequest{PracticeName: "P", StaffName: "S", StaffEmail: testStaffEmail})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestSignupHandler_TokenVerificationFailure(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(fakeVerifier{err: errBadToken}, db)
	defer srv.Close()

	resp := postSignup(t, srv, "bad-token", staffauth.SignupRequest{PracticeName: "P", StaffName: "S", StaffEmail: testStaffEmail})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestSignupHandler_InvalidBody(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(fakeVerifier{uid: "bad-body-uid"}, db)
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/staff/signup", bytes.NewReader([]byte("not json")))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestSignupHandler_MissingFields(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(fakeVerifier{uid: "new-owner"}, db)
	defer srv.Close()

	resp := postSignup(t, srv, "tok", staffauth.SignupRequest{PracticeName: "", StaffName: "S", StaffEmail: testStaffEmail})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestSignupHandler_Success(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(fakeVerifier{uid: "new-owner"}, db)
	defer srv.Close()

	resp := postSignup(t, srv, "tok", staffauth.SignupRequest{
		PracticeName: "Solo Doula Co",
		StaffName:    "Jamie Owner",
		StaffEmail:   jamieEmail,
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var out staffauth.SignupResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.StaffID == "" || out.PracticeID == "" {
		t.Fatalf("expected non-empty ids, got %+v", out)
	}

	var roleCount int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT array_length(roles, 1) FROM practice_memberships WHERE practice_id = $1 AND staff_id = $2`,
		out.PracticeID, out.StaffID,
	).Scan(&roleCount); err != nil {
		t.Fatalf("query membership roles: %v", err)
	}
	if roleCount != 3 {
		t.Fatalf("expected 3 roles (owner, office_manager, doula), got %d", roleCount)
	}
}

func TestSignupHandler_DuplicateSignup(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(fakeVerifier{uid: "repeat-owner"}, db)
	defer srv.Close()

	body := staffauth.SignupRequest{PracticeName: "First Practice", StaffName: jamieName, StaffEmail: jamieEmail}
	first := postSignup(t, srv, "tok", body)
	_ = first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first signup status = %d, want %d", first.StatusCode, http.StatusCreated)
	}

	second := postSignup(t, srv, "tok", staffauth.SignupRequest{PracticeName: "Second Practice", StaffName: jamieName, StaffEmail: jamieEmail})
	defer second.Body.Close()
	if second.StatusCode != http.StatusConflict {
		t.Fatalf("second signup status = %d, want %d", second.StatusCode, http.StatusConflict)
	}
}

// TestSignupHandler_SeedsDefaultPlanTemplates proves signup inserts one
// plan_templates row per plan type (care_plan, birth_plan) in the same
// transaction as the Practice/Staff/membership rows.
func TestSignupHandler_SeedsDefaultPlanTemplates(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(fakeVerifier{uid: "seed-owner"}, db)
	defer srv.Close()

	resp := postSignup(t, srv, "tok", staffauth.SignupRequest{
		PracticeName: "Seeded Practice",
		StaffName:    "Jamie Owner",
		StaffEmail:   jamieEmail,
	})
	defer resp.Body.Close()

	var out staffauth.SignupResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	var planTypes []string
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT plan_type::text FROM plan_templates WHERE practice_id = $1 ORDER BY plan_type`, out.PracticeID,
	)
	if err != nil {
		t.Fatalf("query plan_templates: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var pt string
		if err := rows.Scan(&pt); err != nil {
			t.Fatalf("scan plan_type: %v", err)
		}
		planTypes = append(planTypes, pt)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate plan_templates rows: %v", err)
	}
	if len(planTypes) != 2 || planTypes[0] != "birth_plan" || planTypes[1] != "care_plan" {
		t.Fatalf("seeded plan_types = %v, want [birth_plan care_plan]", planTypes)
	}
}

// TestSignupHandler_SeededTemplatesRoundTripThroughPlansAPI proves the
// hardcoded default field JSON in signup.go matches the palette the plans
// package validates: GET the seeded template, then PUT that exact payload
// back. If signup's literal drifts from plans' validation rules, this test
// goes red instead of shipping a seeded template the API would reject.
func TestSignupHandler_SeededTemplatesRoundTripThroughPlansAPI(t *testing.T) {
	db := testdb.New(t)
	verifier := fakeVerifier{uid: "roundtrip-owner"}

	mux := http.NewServeMux()
	mux.Handle("POST /staff/signup", staffauth.SignupHandler(verifier, db.App))
	mux.Handle("GET /practices/{practiceId}/plan-templates/{planType}",
		staffauth.Middleware(verifier, db.App)(plans.GetTemplateHandler()))
	mux.Handle("PUT /practices/{practiceId}/plan-templates/{planType}",
		staffauth.Middleware(verifier, db.App)(plans.PutTemplateHandler()))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	signupResp := postSignup(t, srv, "tok", staffauth.SignupRequest{
		PracticeName: "Roundtrip Practice", StaffName: jamieName, StaffEmail: jamieEmail,
	})
	defer signupResp.Body.Close()
	var signedUp staffauth.SignupResponse
	if err := json.NewDecoder(signupResp.Body).Decode(&signedUp); err != nil {
		t.Fatalf("decode signup response: %v", err)
	}

	for _, planType := range []string{"care_plan", "birth_plan"} {
		getReq, err := http.NewRequestWithContext(t.Context(), http.MethodGet,
			srv.URL+"/practices/"+signedUp.PracticeID+"/plan-templates/"+planType, nil)
		if err != nil {
			t.Fatalf("build GET request: %v", err)
		}
		getReq.Header.Set("Authorization", "Bearer tok")
		getResp, err := http.DefaultClient.Do(getReq)
		if err != nil {
			t.Fatalf("GET request: %v", err)
		}
		defer getResp.Body.Close()
		if getResp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s status = %d, want %d", planType, getResp.StatusCode, http.StatusOK)
		}
		var seeded plans.TemplateResponse
		if err := json.NewDecoder(getResp.Body).Decode(&seeded); err != nil {
			t.Fatalf("decode GET %s response: %v", planType, err)
		}
		if len(seeded.Fields) == 0 {
			t.Fatalf("expected seeded %s fields to be non-empty", planType)
		}

		putBody, err := json.Marshal(seeded)
		if err != nil {
			t.Fatalf("marshal seeded %s fields: %v", planType, err)
		}
		putReq, err := http.NewRequestWithContext(t.Context(), http.MethodPut,
			srv.URL+"/practices/"+signedUp.PracticeID+"/plan-templates/"+planType, bytes.NewReader(putBody))
		if err != nil {
			t.Fatalf("build PUT request: %v", err)
		}
		putReq.Header.Set("Authorization", "Bearer tok")
		putReq.Header.Set("Content-Type", "application/json")
		putResp, err := http.DefaultClient.Do(putReq)
		if err != nil {
			t.Fatalf("PUT request: %v", err)
		}
		defer putResp.Body.Close()
		if putResp.StatusCode != http.StatusOK {
			t.Fatalf("PUT %s status = %d, want %d", planType, putResp.StatusCode, http.StatusOK)
		}
	}
}
