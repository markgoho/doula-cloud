package staffauth_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/plans"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/session"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

const (
	testStaffEmail = "s@example.com"
	jamieEmail     = "jamie@example.com"
	jamieName      = "Jamie"
	jamieOwnerName = "Jamie Owner"

	// addresslessPractice names the Practice #614's refusal must never
	// create. Read twice -- as the request's name, and as the row count
	// that has to stay zero.
	addresslessPractice = "Addressless Practice"

	// Shared across staffauth_test files: goconst flags repeated literals
	// package-wide, not just within one file.
	someUID           = "some-uid"
	roundtripOwnerUID = "roundtrip-owner"
	ownerRole         = "owner"
	doulaRole         = "doula"
	adminRole         = "admin"
	employeeType      = "employee"
	contractorType    = "contractor"
)

// errBadToken is what a fake Verifier returns for a token Identity
// Platform would reject. Shared by the bootstrap endpoints' tests -- signup
// and (in portalinvite) the Client portal's -- which since #151 are the
// only ones still reading a Bearer ID token.
var errBadToken = errors.New("invalid token")

// sessionCookie returns the __session cookie from resp, or nil if none
// was set. Shared across staffauth_test files.
func sessionCookie(resp *http.Response) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == session.CookieName {
			return c
		}
	}
	return nil
}

func newSignupServer(verifier authntest.Verifier, db *testdb.DB) *httptest.Server {
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	staffauth.Mount(g, ir, db.App, verifier, authntest.NewFakeAccountManager(), tasknudge.NoOpEnqueuer{})
	return httptest.NewServer(mux)
}

func postSignup(t *testing.T, srv *httptest.Server, token string, body any) *http.Response {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/staff/signup", bytes.NewReader(payload))
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
	srv := newSignupServer(authntest.Verifier{}, db)
	defer srv.Close()

	resp := postSignup(t, srv, "", staffauth.SignupRequest{WorkState: "NY", PracticeName: "P", StaffName: "S"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	if c := sessionCookie(resp); c != nil {
		t.Fatalf("cookie set on missing token: %+v", c)
	}
}

func TestSignupHandler_TokenVerificationFailure(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(authntest.Verifier{Err: errBadToken}, db)
	defer srv.Close()

	resp := postSignup(t, srv, "bad-token", staffauth.SignupRequest{WorkState: "NY", PracticeName: "P", StaffName: "S"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	if c := sessionCookie(resp); c != nil {
		t.Fatalf("cookie set on invalid token: %+v", c)
	}
}

func TestSignupHandler_InvalidBody(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(authntest.Verifier{UID: "bad-body-uid"}, db)
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/staff/signup", bytes.NewReader([]byte("not json")))
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
	srv := newSignupServer(authntest.Verifier{UID: "new-owner"}, db)
	defer srv.Close()

	resp := postSignup(t, srv, "tok", staffauth.SignupRequest{WorkState: "NY", PracticeName: "", StaffName: "S"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestSignupHandler_Success(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(authntest.Verifier{UID: "new-owner", Email: jamieEmail}, db)
	defer srv.Close()

	resp := postSignup(t, srv, "tok", staffauth.SignupRequest{WorkState: "NY",
		PracticeName: "Solo Doula Co",
		StaffName:    jamieOwnerName,
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
		t.Fatalf("expected 3 roles (owner, admin, doula), got %d", roleCount)
	}

	// #145: signup sets the session cookie on its own response, same
	// name/attributes/lifetime as the create-session endpoint's (#144) --
	// deliberately not asserting on the cookie's value.
	c := sessionCookie(resp)
	if c == nil {
		t.Fatal("no __session cookie set on successful signup")
	}
	if c.Value == "" {
		t.Fatal("cookie value is empty")
	}
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
		t.Errorf("cookie attributes = %+v, want HttpOnly, Secure, SameSite=Lax, Path=/", c)
	}
	if wantMaxAge := int(session.Lifetime.Seconds()); c.MaxAge != wantMaxAge {
		t.Errorf("MaxAge = %d, want %d", c.MaxAge, wantMaxAge)
	}
}

// TestSignupHandler_SessionStoreFailure proves a signup that cannot
// create its session commits nothing: no cookie, and no Practice/Staff
// rows, so retrying the signup is safe instead of hitting a 409 for a
// Practice the caller never saw created (#145). Creating the session is
// the last thing the handler does before committing, which is what
// makes that ordering hold.
func TestSignupHandler_SessionStoreFailure(t *testing.T) {
	db := testdb.New(t)
	const practiceName = "Mint Fail Practice"
	srv := newSignupServer(authntest.Verifier{UID: "mint-fail-owner", Email: jamieEmail}, db)
	defer srv.Close()
	if _, err := db.Admin.ExecContext(t.Context(), `DROP TABLE sessions`); err != nil {
		t.Fatalf("drop sessions: %v", err)
	}

	resp := postSignup(t, srv, "tok", staffauth.SignupRequest{WorkState: "NY",
		PracticeName: practiceName,
		StaffName:    jamieOwnerName,
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
	if c := sessionCookie(resp); c != nil {
		t.Fatalf("cookie set on session store failure: %+v", c)
	}

	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM practices WHERE name = $1`, practiceName,
	).Scan(&count); err != nil {
		t.Fatalf("query practices: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no Practice committed after mint failure, found %d", count)
	}
}

func TestSignupHandler_DuplicateSignup(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(authntest.Verifier{UID: "repeat-owner", Email: jamieEmail}, db)
	defer srv.Close()

	body := staffauth.SignupRequest{WorkState: "NY", PracticeName: "First Practice", StaffName: jamieName}
	first := postSignup(t, srv, "tok", body)
	_ = first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first signup status = %d, want %d", first.StatusCode, http.StatusCreated)
	}

	second := postSignup(t, srv, "tok", staffauth.SignupRequest{WorkState: "NY", PracticeName: "Second Practice", StaffName: jamieName})
	defer second.Body.Close()
	if second.StatusCode != http.StatusConflict {
		t.Fatalf("second signup status = %d, want %d", second.StatusCode, http.StatusConflict)
	}
}

// TestSignupHandler_GrantsSignupBonus proves signup inserts exactly one
// +3 signup_bonus credit_ledger row for the new Practice, in the same
// transaction as the Practice/Staff/membership rows.
func TestSignupHandler_GrantsSignupBonus(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(authntest.Verifier{UID: "bonus-owner", Email: jamieEmail}, db)
	defer srv.Close()

	resp := postSignup(t, srv, "tok", staffauth.SignupRequest{WorkState: "NY",
		PracticeName: "Bonus Practice",
		StaffName:    jamieOwnerName,
	})
	defer resp.Body.Close()

	var out staffauth.SignupResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	var origin string
	var quantity, rowCount int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT origin::text, quantity, count(*) OVER () FROM credit_ledger WHERE practice_id = $1`,
		out.PracticeID,
	).Scan(&origin, &quantity, &rowCount); err != nil {
		t.Fatalf("query credit_ledger: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("expected exactly 1 credit_ledger row, got %d", rowCount)
	}
	if origin != "signup_bonus" || quantity != 3 {
		t.Fatalf("credit_ledger row = {origin: %q, quantity: %d}, want {signup_bonus, 3}", origin, quantity)
	}
}

// TestSignupHandler_SeedsDefaultPlanTemplates proves signup inserts one
// plan_templates row per plan type (care_plan, birth_plan) in the same
// transaction as the Practice/Staff/membership rows.
func TestSignupHandler_SeedsDefaultPlanTemplates(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(authntest.Verifier{UID: "seed-owner", Email: jamieEmail}, db)
	defer srv.Close()

	resp := postSignup(t, srv, "tok", staffauth.SignupRequest{WorkState: "NY",
		PracticeName: "Seeded Practice",
		StaffName:    jamieOwnerName,
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
	verifier := authntest.Verifier{UID: roundtripOwnerUID, Email: jamieEmail}

	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	staffauth.Mount(g, ir, db.App, verifier, authntest.NewFakeAccountManager(), tasknudge.NoOpEnqueuer{})
	plans.Mount(g, ir, db.App)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Signup itself still takes a Bearer ID token (it runs before a
	// session exists); everything behind the middleware takes the cookie.
	session := authntest.SeedSession(t, db.App, roundtripOwnerUID)

	signupResp := postSignup(t, srv, "tok", staffauth.SignupRequest{WorkState: "NY",
		PracticeName: "Roundtrip Practice", StaffName: jamieName,
	})
	defer signupResp.Body.Close()
	var signedUp staffauth.SignupResponse
	if err := json.NewDecoder(signupResp.Body).Decode(&signedUp); err != nil {
		t.Fatalf("decode signup response: %v", err)
	}

	for _, planType := range []string{"care_plan", "birth_plan"} {
		getReq, err := http.NewRequestWithContext(t.Context(), http.MethodGet,
			srv.URL+"/api/practices/"+signedUp.PracticeID+"/plan-templates/"+planType, nil)
		if err != nil {
			t.Fatalf("build GET request: %v", err)
		}
		authntest.AddSessionCookie(getReq, session)
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
			srv.URL+"/api/practices/"+signedUp.PracticeID+"/plan-templates/"+planType, bytes.NewReader(putBody))
		if err != nil {
			t.Fatalf("build PUT request: %v", err)
		}
		authntest.AddSessionCookie(putReq, session)
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

// TestSignupHandler_SeedsDefaultContractTemplate proves signup inserts a
// contract_templates row in the same transaction as the
// Practice/Staff/membership rows.
func TestSignupHandler_SeedsDefaultContractTemplate(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(authntest.Verifier{UID: "seed-contract-owner", Email: jamieEmail}, db)
	defer srv.Close()

	resp := postSignup(t, srv, "tok", staffauth.SignupRequest{WorkState: "NY",
		PracticeName: "Seeded Contract Practice",
		StaffName:    jamieOwnerName,
	})
	defer resp.Body.Close()

	var out staffauth.SignupResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	var prose string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT prose FROM contract_templates WHERE practice_id = $1`, out.PracticeID,
	).Scan(&prose); err != nil {
		t.Fatalf("query contract_templates: %v", err)
	}
	if prose == "" {
		t.Fatal("expected a non-empty seeded contract template prose")
	}
}

// TestSignupHandler_SeededContractTemplateRoundTripsThroughContractsAPI
// proves the hardcoded default prose in signup.go passes the contracts
// package's own validation: GET the seeded template, then PUT that exact
// payload back. If signup's literal ever normalizes to blank, this test
// goes red instead of shipping a seeded template the API would reject.
func TestSignupHandler_SeededContractTemplateRoundTripsThroughContractsAPI(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "roundtrip-contract-owner"
	verifier := authntest.Verifier{UID: ownerUID, Email: jamieEmail}

	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	staffauth.Mount(g, ir, db.App, verifier, authntest.NewFakeAccountManager(), tasknudge.NoOpEnqueuer{})
	contracts.Mount(g, ir, db.App, objectstore.NewMemoryStore(), push.NewFakePusher())
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Signup itself still takes a Bearer ID token (it runs before a
	// session exists); everything behind the middleware takes the cookie.
	session := authntest.SeedSession(t, db.App, ownerUID)

	signupResp := postSignup(t, srv, "tok", staffauth.SignupRequest{WorkState: "NY",
		PracticeName: "Roundtrip Contract Practice", StaffName: jamieName,
	})
	defer signupResp.Body.Close()
	var signedUp staffauth.SignupResponse
	if err := json.NewDecoder(signupResp.Body).Decode(&signedUp); err != nil {
		t.Fatalf("decode signup response: %v", err)
	}

	getReq, err := http.NewRequestWithContext(t.Context(), http.MethodGet,
		srv.URL+"/api/practices/"+signedUp.PracticeID+"/contract-template", nil)
	if err != nil {
		t.Fatalf("build GET request: %v", err)
	}
	authntest.AddSessionCookie(getReq, session)
	getResp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		t.Fatalf("GET request: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d, want %d", getResp.StatusCode, http.StatusOK)
	}
	var seeded contracts.TemplateResponse
	if err := json.NewDecoder(getResp.Body).Decode(&seeded); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	if seeded.Prose == "" {
		t.Fatal("expected seeded prose to be non-empty")
	}

	putBody, err := json.Marshal(seeded)
	if err != nil {
		t.Fatalf("marshal seeded prose: %v", err)
	}
	putReq, err := http.NewRequestWithContext(t.Context(), http.MethodPut,
		srv.URL+"/api/practices/"+signedUp.PracticeID+"/contract-template", bytes.NewReader(putBody))
	if err != nil {
		t.Fatalf("build PUT request: %v", err)
	}
	authntest.AddSessionCookie(putReq, session)
	putReq.Header.Set("Content-Type", "application/json")
	putResp, err := http.DefaultClient.Do(putReq)
	if err != nil {
		t.Fatalf("PUT request: %v", err)
	}
	defer putResp.Body.Close()
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status = %d, want %d", putResp.StatusCode, http.StatusOK)
	}
}

// TestSignupHandler_RefusesTokenWithNoAddress covers #614: staff.email is
// the address on the verified ID token, so an identity whose provider
// reports no address has nothing to write there. The refusal is
// acceptInvite's, for acceptInvite's reason -- the address is what a
// later invitation to this Practice is matched against, so it may not be
// self-asserted -- and it leaves nothing behind: no Practice, no session.
func TestSignupHandler_RefusesTokenWithNoAddress(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(authntest.Verifier{UID: "addressless-owner"}, db)
	defer srv.Close()

	resp := postSignup(t, srv, "tok", staffauth.SignupRequest{
		WorkState: "NY", PracticeName: addresslessPractice, StaffName: jamieName,
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
	var out apierr.APIError
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if out.Message != staffauth.MsgNoAddressToCreateAPractice {
		t.Fatalf("message = %q, want %q", out.Message, staffauth.MsgNoAddressToCreateAPractice)
	}
	if c := sessionCookie(resp); c != nil {
		t.Fatalf("cookie set on refused signup: %+v", c)
	}

	var practices int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM practices WHERE name = $1`, addresslessPractice,
	).Scan(&practices); err != nil {
		t.Fatalf("count practices: %v", err)
	}
	if practices != 0 {
		t.Fatalf("practices = %d, want 0 -- the refusal must roll back", practices)
	}
}

// TestSignupHandler_TakesEmailFromVerifiedToken covers #614's other half:
// the address comes off the token, normalized the way resolveStaff
// normalizes an invited address, and the request body has no say in it.
// The token's address is deliberately mixed-case and padded, so a column
// that matched by accident could not.
func TestSignupHandler_TakesEmailFromVerifiedToken(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(authntest.Verifier{UID: "token-address-owner", Email: "  Jamie@Example.COM "}, db)
	defer srv.Close()

	resp := postSignup(t, srv, "tok", staffauth.SignupRequest{
		WorkState: "NY", PracticeName: "Token Address Practice", StaffName: jamieName,
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var out staffauth.SignupResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	var email string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT email FROM staff WHERE id = $1`, out.StaffID,
	).Scan(&email); err != nil {
		t.Fatalf("query staff email: %v", err)
	}
	if email != jamieEmail {
		t.Fatalf("staff.email = %q, want %q", email, jamieEmail)
	}
}
