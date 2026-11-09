package staffauth_test

import (
	"database/sql"
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/ianazone"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/plans"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/session"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

const (
	testStaffEmail = "s@example.com"
	jamieEmail     = "jamie@example.com"
	jamieName      = "Jamie"
	ownerLastName  = "Owner"
	// carePlanType and birthPlanType are the two seeded plan_templates.
	carePlanType  = "care_plan"
	birthPlanType = "birth_plan"
	// placeholderLastName fills the last name for a test whose subject is
	// something else.
	placeholderLastName = "Last"
	leaFirstName        = "Lena"
	maryAnne            = "Mary Anne"
	smith               = "Smith"
	jamieOwnerName      = "Jamie Owner"

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
	return newSignupServerWith(verifier, db)
}

// newSignupServerWith mounts other packages' routes beside this one's, for
// a test whose subject is how the two agree.
func newSignupServerWith(verifier authntest.Verifier, db *testdb.DB, others ...func(*staffauth.GatedRouter, *idempotency.Router, *sql.DB)) *httptest.Server {
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	staffauth.Mount(g, ir, db.App, verifier, authntest.NewFakeAccountManager(), tasknudge.NoOpEnqueuer{}, neverSuppressed)
	for _, mount := range others {
		mount(g, ir, db.App)
	}
	return httptest.NewServer(mux)
}

// signupZone is the zone every signup test that is not about zones sends
// (#1166) -- the value 00110_practice_timezone.sql carried as its column
// default before that ticket dropped it.
const signupZone = "America/New_York"

// postSignup posts a signup body. A staffauth.SignupRequest that names no
// zone is given one first (#1166): every one of this package's signup
// tests predates the field and is about something else, and the endpoint
// now refuses a body with no zone on it. A test that is about the zone
// itself passes a map instead of the struct, so it can send a zone the
// IANA database does not name, or none at all.
func postSignup(t *testing.T, srv *httptest.Server, token string, body any) *http.Response {
	t.Helper()
	if req, ok := body.(staffauth.SignupRequest); ok && req.Timezone == "" {
		req.Timezone = signupZone
		body = req
	}
	resp := reqtest.Post(t, srv.URL+"/api/staff/signup", reqtest.JSON(body), reqtest.HeaderIf(token != "", "Authorization", "Bearer "+token))
	return resp
}

// signupEnv is what a signup test starts from: a database and a server
// mounting this package's whole surface for a Verifier that vouches for
// one identity. Nobody is signed in and no Practice exists yet -- signup
// is what makes both -- so it takes the Bearer ID token every bootstrap
// endpoint still reads, not a Scene's session.
type signupEnv struct {
	T   *testing.T
	DB  *testdb.DB
	Srv *httptest.Server
}

func newSignupEnv(t *testing.T, verifier authntest.Verifier) *signupEnv {
	t.Helper()
	db := testdb.New(t)
	srv := newSignupServer(verifier, db)
	t.Cleanup(srv.Close)
	return &signupEnv{T: t, DB: db, Srv: srv}
}

// post signs up with token and leaves the response to the caller, for a
// test that reads its cookie or headers.
func (e *signupEnv) post(token string, body any) *http.Response {
	e.T.Helper()
	return postSignup(e.T, e.Srv, token, body)
}

// signUp founds a Practice and requires the 201.
func (e *signupEnv) signUp(body staffauth.SignupRequest) staffauth.SignupResponse {
	e.T.Helper()
	resp := e.post("tok", body)
	defer func() { _ = resp.Body.Close() }()
	reqtest.RequireStatus(e.T, resp, http.StatusCreated)
	return reqtest.Decode[staffauth.SignupResponse](e.T, resp)
}

// refused signs up and requires want, returning the refusal's envelope.
func (e *signupEnv) refused(body any, want int) apierr.APIError {
	e.T.Helper()
	resp := e.post("tok", body)
	defer func() { _ = resp.Body.Close() }()
	reqtest.RequireStatus(e.T, resp, want)
	return apierrtest.Decode(e.T, resp)
}

// founder is a signup body that is about something other than the person:
// the Practice's name is the only thing that varies.
func founder(practiceName string) staffauth.SignupRequest {
	return staffauth.SignupRequest{WorkState: "NY", PracticeName: practiceName, FirstName: jamieName, LastName: ownerLastName}
}

// TestSignupHandler_RefusesTheToken covers a request with no Bearer token
// and one whose token Identity Platform rejects: either way a 401 and no
// session cookie.
func TestSignupHandler_RefusesTheToken(t *testing.T) {
	cases := []struct {
		name     string
		verifier authntest.Verifier
		token    string
	}{
		{"no token", authntest.Verifier{}, ""},
		{"a token that does not verify", authntest.Verifier{Err: errBadToken}, "bad-token"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newSignupEnv(t, tc.verifier)

			resp := e.post(tc.token, staffauth.SignupRequest{WorkState: "NY", PracticeName: "P", FirstName: "S", LastName: placeholderLastName})
			defer resp.Body.Close()

			reqtest.RequireStatus(t, resp, http.StatusUnauthorized)
			if c := sessionCookie(resp); c != nil {
				t.Fatalf("cookie set on a refused token: %+v", c)
			}
		})
	}
}

// newOwnerUID is the identity the signup-validation tests sign in as --
// one literal, since none of them gets far enough to write a row.
const newOwnerUID = "new-owner"

func TestSignupHandler_InvalidBody(t *testing.T) {
	e := newSignupEnv(t, authntest.Verifier{UID: "bad-body-uid"})

	reqtest.ExpectStatus(t, http.MethodPost, e.Srv.URL+"/api/staff/signup", http.StatusBadRequest, reqtest.RawJSON([]byte("not json")), reqtest.Header("Authorization", "Bearer tok"))
}

// TestSignupHandler_RefusesAnIncompleteForm covers the fields the signup
// screen asks for. #488: the refusal names the field at fault, keyed by
// the DTO's own json tag, and names only the ones at fault -- the two
// names are asked for on one screen and can be missing together, so
// answering only the first would send her back twice. An empty Practice
// name is no refusal (#1536): it takes her name. The work state's Details
// sentence differs from the Message beside it: the message names the JSON
// field and the format for a caller reading the API, the detail is for
// the person looking at the control.
func TestSignupHandler_RefusesAnIncompleteForm(t *testing.T) {
	cases := []struct {
		name    string
		body    staffauth.SignupRequest
		present map[string]string
		absent  []string
	}{
		{"no first name",
			staffauth.SignupRequest{WorkState: "NY", PracticeName: "P", FirstName: "", LastName: placeholderLastName},
			map[string]string{firstNameKey: staffauth.MsgFirstNameNeeded}, []string{lastNameKey}},
		{"neither name",
			staffauth.SignupRequest{WorkState: "NY"},
			map[string]string{firstNameKey: staffauth.MsgFirstNameNeeded, lastNameKey: staffauth.MsgLastNameNeeded}, []string{"practiceName"}},
		{"a work state that is not one",
			staffauth.SignupRequest{PracticeName: "P", FirstName: "S", LastName: placeholderLastName, WorkState: "ZZ"},
			map[string]string{workStateKey: staffauth.MsgWorkStateNeeded}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newSignupEnv(t, authntest.Verifier{UID: newOwnerUID})

			details := e.refused(tc.body, http.StatusBadRequest).Details
			for field, want := range tc.present {
				if details[field] != want {
					t.Fatalf("details = %v, want %s entry %q", details, field, want)
				}
			}
			for _, field := range tc.absent {
				if _, ok := details[field]; ok {
					t.Fatalf("details = %v, want no %s entry", details, field)
				}
			}
		})
	}
}

// TestSignupHandler_EmptyPracticeNameTakesHerName is #1536's fallback,
// held in the BFF so a caller that sends no Practice name gets what the
// form gets: the Practice is called by her own name, first name then last
// name as she typed them, and never by nothing (practices.name is NOT
// NULL, and a Contract, the portal and a card statement each need one).
// A name of white space is no name, and a body with no practiceName key
// at all is the same request as an empty one.
func TestSignupHandler_EmptyPracticeNameTakesHerName(t *testing.T) {
	bodies := map[string]any{
		"empty":       staffauth.SignupRequest{WorkState: "NY", PracticeName: "", FirstName: " " + maryAnne + " ", LastName: smith},
		"white space": staffauth.SignupRequest{WorkState: "NY", PracticeName: "   ", FirstName: maryAnne, LastName: smith},
		"no key": map[string]string{
			firstNameKey: maryAnne, lastNameKey: smith, workStateKey: "NY", "timezone": signupZone,
		},
	}
	for label, body := range bodies {
		t.Run(label, func(t *testing.T) {
			e := newSignupEnv(t, authntest.Verifier{UID: newOwnerUID, Email: jamieEmail})

			resp := e.post("tok", body)
			defer resp.Body.Close()

			reqtest.RequireStatus(t, resp, http.StatusCreated)
			out := reqtest.Decode[staffauth.SignupResponse](t, resp)
			if got, want := storedPracticeName(t, e.DB, out.PracticeID), maryAnne+" "+smith; got != want {
				t.Fatalf("practices.name = %q, want her own name %q", got, want)
			}
		})
	}
}

// storedPracticeName reads a Practice's stored name. Shared with
// signupresume_test.go.
func storedPracticeName(t *testing.T, db *testdb.DB, practiceID string) string {
	t.Helper()
	stored := testdb.Scalar[string](t, db.Admin, `SELECT name FROM practices WHERE id = $1`, practiceID)
	return stored
}

// TestSignupHandler_ZoneIsStatedNotInherited is #1166's own criterion at
// the signup end: a Practice's zone comes off the body the founder sent,
// not off a column default, and the row proves it by holding a zone
// nobody would have picked for her.
func TestSignupHandler_ZoneIsStatedNotInherited(t *testing.T) {
	e := newSignupEnv(t, authntest.Verifier{UID: "signup-states-a-zone", Email: "denver@example.com"})

	out := e.signUp(staffauth.SignupRequest{
		PracticeName: "Mile High Doulas", FirstName: "Robin", LastName: placeholderLastName, WorkState: "CO",
		Timezone: "America/Denver",
	})

	stored := testdb.Scalar[string](t, e.DB.Admin, `SELECT timezone FROM practices WHERE id = $1`, out.PracticeID)
	if stored != "America/Denver" {
		t.Fatalf("stored zone = %q, want the one she stated", stored)
	}
}

// TestSignupHandler_RefusesAZoneTheDatabaseDoesNotName proves the zone is
// held to the IANA database at signup too, not only on the settings
// write -- and that an omitted field is a refusal rather than a silent
// UTC. The body is a map rather than a SignupRequest so postSignup's own
// default does not fill the field in.
func TestSignupHandler_RefusesAZoneTheDatabaseDoesNotName(t *testing.T) {
	for _, zone := range []string{"Nowhere/Atlantis", ""} {
		t.Run("zone "+zone, func(t *testing.T) {
			e := newSignupEnv(t, authntest.Verifier{UID: newOwnerUID, Email: jamieEmail})

			details := e.refused(map[string]string{
				"practiceName": "P", firstNameKey: "S", lastNameKey: "Last", workStateKey: "NY", "timezone": zone,
			}, http.StatusBadRequest).Details

			// Two sentences for two different mistakes: nothing chosen
			// reads "choose one", and a name the database does not carry
			// says what is wrong with the name -- "choose one" beside an
			// already-filled control would tell her off for the wrong
			// thing.
			want := staffauth.MsgTimezoneNeeded
			if zone != "" {
				want = ianazone.MsgNotRecognized
			}
			if details["timezone"] != want {
				t.Fatalf("details = %v, want %q for zone %q", details, want, zone)
			}
		})
	}
}

// decodeDetails reads APIError.Details off a refusal, so a test can
// assert which field the BFF said was at fault (#488).
func decodeDetails(t *testing.T, resp *http.Response) map[string]string {
	t.Helper()
	return apierrtest.Decode(t, resp).Details
}

func TestSignupHandler_Success(t *testing.T) {
	e := newSignupEnv(t, authntest.Verifier{UID: "new-owner", Email: jamieEmail})

	resp := e.post("tok", founder("Solo Doula Co"))
	defer resp.Body.Close()

	reqtest.RequireStatus(t, resp, http.StatusCreated)

	out := reqtest.Decode[staffauth.SignupResponse](t, resp)
	if out.StaffID == "" || out.PracticeID == "" {
		t.Fatalf("expected non-empty ids, got %+v", out)
	}

	roleCount := testdb.Scalar[int](t, e.DB.Admin, `SELECT array_length(roles, 1) FROM practice_memberships WHERE practice_id = $1 AND staff_id = $2`, out.PracticeID, out.StaffID)
	if roleCount != 3 {
		t.Fatalf("expected 3 roles (owner, admin, doula), got %d", roleCount)
	}

	// #145: signup sets the session cookie on its own response, same
	// name as the create-session endpoint's (#144) -- deliberately not
	// asserting on the cookie's value. Its attributes and lifetime are
	// authn.MintSession's own contract, proved once in authn's test suite
	// (#837's one session-mint seam) rather than re-proved at every caller.
	c := sessionCookie(resp)
	if c == nil {
		t.Fatal("no __session cookie set on successful signup")
	}
	if c.Value == "" {
		t.Fatal("cookie value is empty")
	}
}

// TestSignupHandler_SessionStoreFailure proves a signup that cannot
// create its session commits nothing: no cookie, and no Practice/Staff
// rows, so retrying the signup is safe instead of hitting a 409 for a
// Practice the caller never saw created (#145). Creating the session is
// the last thing the handler does before committing, which is what
// makes that ordering hold.
func TestSignupHandler_SessionStoreFailure(t *testing.T) {
	const practiceName = "Mint Fail Practice"
	e := newSignupEnv(t, authntest.Verifier{UID: "mint-fail-owner", Email: jamieEmail})
	if _, err := e.DB.Admin.ExecContext(t.Context(), `DROP TABLE sessions`); err != nil {
		t.Fatalf("drop sessions: %v", err)
	}

	resp := e.post("tok", founder(practiceName))
	defer resp.Body.Close()

	reqtest.RequireStatus(t, resp, http.StatusInternalServerError)
	if c := sessionCookie(resp); c != nil {
		t.Fatalf("cookie set on session store failure: %+v", c)
	}

	count := testdb.Scalar[int](t, e.DB.Admin, `SELECT count(*) FROM practices WHERE name = $1`, practiceName)
	if count != 0 {
		t.Fatalf("expected no Practice committed after mint failure, found %d", count)
	}
}

func TestSignupHandler_DuplicateSignup(t *testing.T) {
	e := newSignupEnv(t, authntest.Verifier{UID: "repeat-owner", Email: jamieEmail})

	e.signUp(founder("First Practice"))

	e.refused(founder("Second Practice"), http.StatusConflict)
}

// TestSignupHandler_GrantsSignupBonus proves signup inserts exactly one
// +3 signup_bonus credit_ledger row for the new Practice, in the same
// transaction as the Practice/Staff/membership rows.
func TestSignupHandler_GrantsSignupBonus(t *testing.T) {
	e := newSignupEnv(t, authntest.Verifier{UID: "bonus-owner", Email: jamieEmail})
	out := e.signUp(founder("Bonus Practice"))

	var origin string
	var quantity, rowCount int
	if err := e.DB.Admin.QueryRowContext(t.Context(),
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
	e := newSignupEnv(t, authntest.Verifier{UID: "seed-owner", Email: jamieEmail})
	out := e.signUp(founder("Seeded Practice"))

	planTypes := testdb.Column[string](t, e.DB.Admin, `SELECT plan_type::text FROM plan_templates WHERE practice_id = $1 ORDER BY plan_type`, out.PracticeID)
	if len(planTypes) != 2 || planTypes[0] != birthPlanType || planTypes[1] != carePlanType {
		t.Fatalf("seeded plan_types = %v, want [birth_plan care_plan]", planTypes)
	}
}

// seededPlanFields reads the field list signup seeded for planType.
func seededPlanFields[T any](t *testing.T, e *signupEnv, practiceID, planType string) []T {
	t.Helper()
	raw := testdb.Scalar[[]byte](t, e.DB.Admin, `SELECT fields FROM plan_templates WHERE practice_id = $1 AND plan_type = $2`, practiceID, planType)
	var fields []T
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("decode %s fields: %v", planType, err)
	}
	return fields
}

// TestSignupHandler_SeededPlansShowNoGroupOverSeven proves each seeded
// plan is chunked so that no headed group asks more than seven decisions
// at once -- stage 4's Decisions budget on the solo birth doula's map
// (MO-G14, #2195). It counts the strict way the walk did: a choose-several
// question one decision per box, and the form's Save and Cancel with the
// last group, which they sit directly under.
func TestSignupHandler_SeededPlansShowNoGroupOverSeven(t *testing.T) {
	e := newSignupEnv(t, authntest.Verifier{UID: "chunk-owner", Email: jamieEmail})
	out := e.signUp(founder("Chunked Practice"))

	const budget = 7
	const formButtons = 2 // Save and Cancel
	type planField struct {
		Type    string   `json:"type"`
		Label   string   `json:"label"`
		Options []string `json:"options"`
	}
	for _, planType := range []string{carePlanType, birthPlanType} {
		var groups []int
		var headings []string
		for _, field := range seededPlanFields[planField](t, e, out.PracticeID, planType) {
			if field.Type == "section_header" {
				groups = append(groups, 0)
				headings = append(headings, field.Label)
				continue
			}
			if len(groups) == 0 {
				groups = append(groups, 0)
				headings = append(headings, "(before any heading)")
			}
			decisions := 1
			if field.Type == "multi_select" {
				decisions = len(field.Options)
			}
			groups[len(groups)-1] += decisions
		}
		groups[len(groups)-1] += formButtons
		for i, count := range groups {
			if count > budget {
				t.Errorf("%s group %q shows %d decisions at once, want at most %d", planType, headings[i], count, budget)
			}
		}
	}
}

// TestSignupHandler_PainReliefIsAskedOnTheBirthPlanOnly proves the
// pain relief question sits on the Birth Plan, which the Client reads and
// the hospital is handed, and not also on the staff-only Care Plan (#2580).
func TestSignupHandler_PainReliefIsAskedOnTheBirthPlanOnly(t *testing.T) {
	e := newSignupEnv(t, authntest.Verifier{UID: "pain-owner", Email: jamieEmail})
	out := e.signUp(founder("Pain Practice"))

	type planField struct {
		Label string `json:"label"`
	}
	for planType, wantAsked := range map[string]bool{birthPlanType: true, carePlanType: false} {
		asked := false
		for _, field := range seededPlanFields[planField](t, e, out.PracticeID, planType) {
			if field.Label == "Preferred pain relief" {
				asked = true
			}
		}
		if asked != wantAsked {
			t.Errorf("%s asks \"Preferred pain relief\" = %v, want %v", planType, asked, wantAsked)
		}
	}
}

// signupThenReadBack signs up with a Verifier for uid, with another
// package's routes mounted beside this one's, and returns the new Practice
// and a session for the Owner it made -- signup itself still takes a
// Bearer ID token (it runs before a session exists); everything behind the
// middleware takes the cookie.
func signupThenReadBack(t *testing.T, uid string, practiceName string, mount func(*staffauth.GatedRouter, *idempotency.Router, *sql.DB)) (srv *httptest.Server, practiceID string, session reqtest.Option) {
	t.Helper()
	db := testdb.New(t)
	srv = newSignupServerWith(authntest.Verifier{UID: uid, Email: jamieEmail}, db, mount)
	t.Cleanup(srv.Close)

	sessionToken := authntest.SeedSession(t, db.App, uid)
	e := &signupEnv{T: t, DB: db, Srv: srv}
	return srv, e.signUp(founder(practiceName)).PracticeID, reqtest.Session(sessionToken)
}

// TestSignupHandler_SeededTemplatesRoundTripThroughPlansAPI proves the
// hardcoded default field JSON in signup.go matches the palette the plans
// package validates: GET the seeded template, then PUT that exact payload
// back. If signup's literal drifts from plans' validation rules, this test
// goes red instead of shipping a seeded template the API would reject.
func TestSignupHandler_SeededTemplatesRoundTripThroughPlansAPI(t *testing.T) {
	srv, practiceID, session := signupThenReadBack(t, roundtripOwnerUID, "Roundtrip Practice", plans.Mount)

	for _, planType := range []string{carePlanType, birthPlanType} {
		url := srv.URL + "/api/practices/" + practiceID + "/plan-templates/" + planType
		seeded := reqtest.Expect[plans.TemplateResponse](t, http.MethodGet, url, http.StatusOK, session)
		if len(seeded.Fields) == 0 {
			t.Fatalf("expected seeded %s fields to be non-empty", planType)
		}

		reqtest.ExpectStatus(t, http.MethodPut, url, http.StatusOK, reqtest.JSON(seeded), session)
	}
}

// TestSignupHandler_SeedsDefaultContractTemplate proves signup inserts a
// contract_templates row in the same transaction as the
// Practice/Staff/membership rows.
func TestSignupHandler_SeedsDefaultContractTemplate(t *testing.T) {
	e := newSignupEnv(t, authntest.Verifier{UID: "seed-contract-owner", Email: jamieEmail})
	out := e.signUp(founder("Seeded Contract Practice"))

	prose := testdb.Scalar[string](t, e.DB.Admin, `SELECT prose FROM contract_templates WHERE practice_id = $1`, out.PracticeID)
	if prose == "" {
		t.Fatal("expected a non-empty seeded contract template prose")
	}
	// #2222: the Client signs this, so its words are hers ("my care",
	// GLOSSARY.md), never the team's "Engagement".
	if want := "Dates of care: {{engagement_start_date}} through {{engagement_end_date}}"; !strings.Contains(prose, want) {
		t.Errorf("seeded prose does not contain %q:\n%s", want, prose)
	}
	if strings.Contains(prose, "Engagement") {
		t.Errorf("seeded prose names the Engagement to the Client:\n%s", prose)
	}
}

// TestSignupHandler_SeededContractTemplateRoundTripsThroughContractsAPI
// proves the hardcoded default prose in signup.go passes the contracts
// package's own validation: GET the seeded template, then PUT that exact
// payload back. If signup's literal ever normalizes to blank, this test
// goes red instead of shipping a seeded template the API would reject.
func TestSignupHandler_SeededContractTemplateRoundTripsThroughContractsAPI(t *testing.T) {
	mount := func(g *staffauth.GatedRouter, ir *idempotency.Router, app *sql.DB) {
		contracts.Mount(g, ir, app, objectstore.NewMemoryStore(), push.NewFakePusher())
	}
	srv, practiceID, session := signupThenReadBack(t, "roundtrip-contract-owner", "Roundtrip Contract Practice", mount)

	url := srv.URL + "/api/practices/" + practiceID + "/contract-template"
	seeded := reqtest.Expect[contracts.TemplateResponse](t, http.MethodGet, url, http.StatusOK, session)
	if seeded.Prose == "" {
		t.Fatal("expected seeded prose to be non-empty")
	}

	reqtest.ExpectStatus(t, http.MethodPut, url, http.StatusOK, reqtest.JSON(seeded), session)
}

// TestSignupHandler_RefusesTokenWithNoAddress covers #614: staff.email is
// the address on the verified ID token, so an identity whose provider
// reports no address has nothing to write there. The refusal is
// acceptInvite's, for acceptInvite's reason -- the address is what a
// later invitation to this Practice is matched against, so it may not be
// self-asserted -- and it leaves nothing behind: no Practice, no session.
func TestSignupHandler_RefusesTokenWithNoAddress(t *testing.T) {
	e := newSignupEnv(t, authntest.Verifier{UID: "addressless-owner"})

	resp := e.post("tok", founder(addresslessPractice))
	defer resp.Body.Close()

	reqtest.RequireStatus(t, resp, http.StatusForbidden)
	out := apierrtest.Decode(t, resp)
	if out.Message != staffauth.MsgNoAddressToCreateAPractice {
		t.Fatalf("message = %q, want %q", out.Message, staffauth.MsgNoAddressToCreateAPractice)
	}
	if c := sessionCookie(resp); c != nil {
		t.Fatalf("cookie set on refused signup: %+v", c)
	}

	practices := testdb.Scalar[int](t, e.DB.Admin, `SELECT count(*) FROM practices WHERE name = $1`, addresslessPractice)
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
	e := newSignupEnv(t, authntest.Verifier{UID: "token-address-owner", Email: "  Jamie@Example.COM "})

	out := e.signUp(founder("Token Address Practice"))

	email := testdb.Scalar[string](t, e.DB.Admin, `SELECT email FROM staff WHERE id = $1`, out.StaffID)
	if email != jamieEmail {
		t.Fatalf("staff.email = %q, want %q", email, jamieEmail)
	}
}
