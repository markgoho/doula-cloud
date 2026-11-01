package staffauth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/internalauth"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

const (
	pilotTestSecret  = "pilot-internal-test-secret"
	pilotGateOpenURL = "/api/internal/staffauth/pilot-gate/open"
	pilotFounder     = "Mark"
	pilotPractice    = "Pilot Practice"
	anaAddress       = "ana@example.com"
	anaMixedCase     = "Ana@Example.com"
)

// newPilotServer mounts signup (on one verified identity) beside the gate
// endpoint, so a test can refuse a signup, let the address in, and retry
// on the same identity against one database.
func newPilotServer(t *testing.T, db *testdb.DB, uid, email string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	staffauth.Mount(g, ir, db.App, authntest.Verifier{UID: uid, Email: email}, authntest.NewFakeAccountManager(), tasknudge.NoOpEnqueuer{}, neverSuppressed)
	guard := internalauth.FromSecret(pilotTestSecret)
	mux.Handle("POST "+pilotGateOpenURL, staffauth.PilotGateOpenHandler(db.App, guard))
	return httptest.NewServer(mux)
}

func postPilot(t *testing.T, srv *httptest.Server, path, secret, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		req.Header.Set("X-Internal-Secret", secret)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func openPilotGate(t *testing.T, srv *httptest.Server, openedBy string) staffauth.PilotGate {
	t.Helper()
	body, err := json.Marshal(staffauth.PilotGateOpenRequest{OpenedBy: openedBy})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := postPilot(t, srv, pilotGateOpenURL, pilotTestSecret, string(body))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("open gate: status = %d, want 200", resp.StatusCode)
	}
	var gate staffauth.PilotGate
	if err := json.NewDecoder(resp.Body).Decode(&gate); err != nil {
		t.Fatalf("decode gate: %v", err)
	}
	return gate
}

func pilotSignup(t *testing.T, srv *httptest.Server) *http.Response {
	t.Helper()
	return postSignup(t, srv, "tok", staffauth.SignupRequest{
		WorkState: "NY", PracticeName: pilotPractice, FirstName: "Ana", LastName: placeholderLastName,
	})
}

func assertPilotRefusal(t *testing.T, db *testdb.DB, resp *http.Response) {
	t.Helper()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	out := apierrtest.Decode(t, resp)
	if out.Code != apierr.CodePilotNotOpen {
		t.Fatalf("code = %q, want %q", out.Code, apierr.CodePilotNotOpen)
	}
	if out.Message != staffauth.MsgPilotNotOpen {
		t.Fatalf("message = %q, want %q", out.Message, staffauth.MsgPilotNotOpen)
	}
	if c := sessionCookie(resp); c != nil {
		t.Fatalf("cookie set on refused signup: %+v", c)
	}
	for _, query := range []string{
		`SELECT count(*) FROM practices`,
		`SELECT count(*) FROM staff`,
		`SELECT count(*) FROM practice_memberships`,
		`SELECT count(*) FROM credit_ledger`,
	} {
		if n := countRows(t, db, query); n != 0 {
			t.Fatalf("%s = %d, want 0 -- a refused signup writes nothing", query, n)
		}
	}
}

func assertSignedUpWithWelcomeCredits(t *testing.T, db *testdb.DB, resp *http.Response) {
	t.Helper()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	var out staffauth.SignupResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	credits := countRows(t, db,
		`SELECT COALESCE(sum(quantity), 0) FROM credit_ledger WHERE practice_id = $1 AND origin = 'signup_bonus'`, out.PracticeID)
	if credits != 3 {
		t.Fatalf("signup bonus = %d, want 3", credits)
	}
	if n := countRows(t, db, `SELECT count(*) FROM practice_memberships WHERE practice_id = $1`, out.PracticeID); n != 1 {
		t.Fatalf("memberships = %d, want 1", n)
	}
}

func TestPilotSignup_RefusesAnAddressNotOnTheAllowlist(t *testing.T) {
	db := testdb.New(t)
	testdb.ClosePilotGate(t, db)
	srv := newPilotServer(t, db, "stranger-uid", "stranger@example.com")
	defer srv.Close()

	resp := pilotSignup(t, srv)
	defer resp.Body.Close()
	assertPilotRefusal(t, db, resp)
}

// The empty-address refusal still comes first: an identity with no
// address has nothing the allowlist could hold.
func TestPilotSignup_NoAddressIsRefusedBeforeTheAllowlist(t *testing.T) {
	db := testdb.New(t)
	testdb.ClosePilotGate(t, db)
	srv := newPilotServer(t, db, "addressless-pilot-uid", "")
	defer srv.Close()

	resp := pilotSignup(t, srv)
	defer resp.Body.Close()
	out := apierrtest.Decode(t, resp)
	if out.Message != staffauth.MsgNoAddressToCreateAPractice {
		t.Fatalf("message = %q, want %q", out.Message, staffauth.MsgNoAddressToCreateAPractice)
	}
}

func TestPilotSignup_AllowsAnAddressOnTheAllowlist(t *testing.T) {
	db := testdb.New(t)
	testdb.ClosePilotGate(t, db)
	srv := newPilotServer(t, db, "ana-uid", anaAddress)
	defer srv.Close()

	testdb.SeedPilotAddress(t, db, anaAddress)
	resp := pilotSignup(t, srv)
	defer resp.Body.Close()
	assertSignedUpWithWelcomeCredits(t, db, resp)
}

// The match is case-insensitive in both directions: the stored address
// and the token's address each go through NormalizeAddress.
func TestPilotSignup_MatchIgnoresCase(t *testing.T) {
	for name, tc := range map[string]struct{ added, signsUp string }{
		"added mixed, signs up lower": {anaMixedCase, anaAddress},
		"added lower, signs up mixed": {anaAddress, anaMixedCase},
	} {
		t.Run(name, func(t *testing.T) {
			db := testdb.New(t)
			testdb.ClosePilotGate(t, db)
			srv := newPilotServer(t, db, "ana-case-uid", tc.signsUp)
			defer srv.Close()

			testdb.SeedPilotAddress(t, db, tc.added)
			resp := pilotSignup(t, srv)
			defer resp.Body.Close()
			assertSignedUpWithWelcomeCredits(t, db, resp)
		})
	}
}

// A refused person keeps the Identity Platform account the screen made
// before calling the BFF. Once the founder adds the address, the same
// identity signing up again goes through.
func TestPilotSignup_RefusedIdentityRetriesAfterTheAddressIsAdded(t *testing.T) {
	db := testdb.New(t)
	testdb.ClosePilotGate(t, db)
	srv := newPilotServer(t, db, "ana-retry-uid", anaAddress)
	defer srv.Close()

	refused := pilotSignup(t, srv)
	defer refused.Body.Close()
	assertPilotRefusal(t, db, refused)

	testdb.SeedPilotAddress(t, db, anaAddress)
	retried := pilotSignup(t, srv)
	defer retried.Body.Close()
	assertSignedUpWithWelcomeCredits(t, db, retried)
}

// An Owner already chose the invitee, so acceptance never asks the
// allowlist.
func TestPilotSignup_InvitationAcceptanceIsNotGated(t *testing.T) {
	db := testdb.New(t)
	ownerID, practiceID := seedOwnerMembership(t, db, "pilot-owner")
	testdb.ClosePilotGate(t, db)
	_, token := seedInvitationWithToken(t, db, practiceID, ownerID, "invitee@example.com", "{doula}", contractorType, time.Now().Add(time.Hour))

	srv := newAcceptServer(t, db, "invitee-uid", "invitee@example.com")
	defer srv.Close()

	resp := postAccept(t, srv, staffauth.AcceptInviteRequest{WorkState: "NY", InviteToken: token, FirstName: leaFirstName, LastName: placeholderLastName})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestPilotGate_RefusesWithoutTheInternalCredential(t *testing.T) {
	db := testdb.New(t)
	testdb.ClosePilotGate(t, db)
	srv := newPilotServer(t, db, someUID, testStaffEmail)
	defer srv.Close()

	for _, secret := range []string{"", "wrong-secret"} {
		resp := postPilot(t, srv, pilotGateOpenURL, secret, `{"openedBy": "Mark"}`)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("secret %q: status = %d, want 401", secret, resp.StatusCode)
		}
	}
	if n := countRows(t, db, `SELECT count(*) FROM pilot_gate WHERE opened_at IS NULL`); n != 1 {
		t.Fatalf("gate is open after a refused request")
	}
}

// Opening the gate records who and when, and from then on signup does
// not ask the allowlist. Opening it again keeps the first record.
func TestPilotGate_OpeningLetsEveryoneSignUpAndKeepsTheFirstRecord(t *testing.T) {
	db := testdb.New(t)
	testdb.ClosePilotGate(t, db)
	srv := newPilotServer(t, db, "january-uid", "january@example.com")
	defer srv.Close()

	before := time.Now().Add(-time.Minute)
	first := openPilotGate(t, srv, "  "+pilotFounder+" ")
	if first.OpenedBy != pilotFounder || first.OpenedAt.Before(before) {
		t.Fatalf("gate = %+v, want opened by %q now", first, pilotFounder)
	}
	second := openPilotGate(t, srv, "Someone Else")
	if second.OpenedBy != first.OpenedBy || !second.OpenedAt.Equal(first.OpenedAt) {
		t.Fatalf("second open = %+v, want the first record %+v", second, first)
	}

	resp := pilotSignup(t, srv)
	defer resp.Body.Close()
	assertSignedUpWithWelcomeCredits(t, db, resp)
}

func TestPilotGate_RefusesABadRequest(t *testing.T) {
	db := testdb.New(t)
	testdb.ClosePilotGate(t, db)
	srv := newPilotServer(t, db, someUID, testStaffEmail)
	defer srv.Close()

	for name, body := range map[string]string{
		"truncated json": `{"openedBy": `,
		"no openedBy":    `{"openedBy": " "}`,
	} {
		resp := postPilot(t, srv, pilotGateOpenURL, pilotTestSecret, body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, resp.StatusCode)
		}
	}
	if n := countRows(t, db, `SELECT count(*) FROM pilot_gate WHERE opened_at IS NULL`); n != 1 {
		t.Fatalf("gate is open after a refused request")
	}
}

func TestLooksLikeAddress_IsAnAtWithSomethingEitherSide(t *testing.T) {
	for address, want := range map[string]bool{
		anaAddress:        true,
		"a@b":             true,
		"":                false,
		"ana.example.com": false,
		"ana@":            false,
		"@example.com":    false,
	} {
		if got := staffauth.LooksLikeAddress(address); got != want {
			t.Errorf("LooksLikeAddress(%q) = %v, want %v", address, got, want)
		}
	}
}
