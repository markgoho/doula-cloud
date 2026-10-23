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
	pilotTestSecret   = "pilot-internal-test-secret"
	pilotAllowlistURL = "/api/internal/staffauth/pilot-allowlist"
	pilotGateOpenURL  = "/api/internal/staffauth/pilot-gate/open"
	pilotFounder      = "Mark"
	pilotPractice     = "Pilot Practice"
	anaAddress        = "ana@example.com"
	anaMixedCase      = "Ana@Example.com"
)

// newPilotServer mounts signup (on one verified identity) beside the two
// operator endpoints, so a test can refuse a signup, add the address, and
// retry on the same identity against one database.
func newPilotServer(t *testing.T, db *testdb.DB, uid, email string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	staffauth.Mount(g, ir, db.App, authntest.Verifier{UID: uid, Email: email}, authntest.NewFakeAccountManager(), tasknudge.NoOpEnqueuer{}, neverSuppressed)
	guard := internalauth.FromSecret(pilotTestSecret)
	mux.Handle("POST "+pilotAllowlistURL, staffauth.PilotAllowlistHandler(db.App, guard))
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

// addToPilot adds address through the operator endpoint and returns the
// row it answered with.
func addToPilot(t *testing.T, srv *httptest.Server, address, addedBy string) staffauth.PilotAllowlistEntry {
	t.Helper()
	body, err := json.Marshal(staffauth.PilotAllowlistRequest{Address: address, AddedBy: addedBy})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := postPilot(t, srv, pilotAllowlistURL, pilotTestSecret, string(body))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("add %q: status = %d, want 200", address, resp.StatusCode)
	}
	var entry staffauth.PilotAllowlistEntry
	if err := json.NewDecoder(resp.Body).Decode(&entry); err != nil {
		t.Fatalf("decode entry: %v", err)
	}
	return entry
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

	addToPilot(t, srv, anaAddress, pilotFounder)
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

			addToPilot(t, srv, tc.added, pilotFounder)
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

	addToPilot(t, srv, anaAddress, pilotFounder)
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

func TestPilotAllowlist_RecordsWhoAddedAndWhen(t *testing.T) {
	db := testdb.New(t)
	srv := newPilotServer(t, db, someUID, testStaffEmail)
	defer srv.Close()

	before := time.Now().Add(-time.Minute)
	entry := addToPilot(t, srv, "  "+anaMixedCase+" ", "  "+pilotFounder+" ")
	if entry.Address != anaAddress || entry.AddedBy != pilotFounder {
		t.Fatalf("entry = %+v, want %q added by %q", entry, anaAddress, pilotFounder)
	}
	if entry.AddedAt.Before(before) {
		t.Fatalf("addedAt = %v, want a time this test ran at", entry.AddedAt)
	}
}

// A second add of the same address, in any case, succeeds and answers
// the first record unchanged.
func TestPilotAllowlist_SecondAddKeepsTheFirstRecord(t *testing.T) {
	db := testdb.New(t)
	srv := newPilotServer(t, db, someUID, testStaffEmail)
	defer srv.Close()

	first := addToPilot(t, srv, anaAddress, pilotFounder)
	second := addToPilot(t, srv, anaMixedCase, "Someone Else")
	if !second.AddedAt.Equal(first.AddedAt) || second.AddedBy != first.AddedBy || second.Address != first.Address {
		t.Fatalf("second add = %+v, want the first record %+v", second, first)
	}
	if n := countRows(t, db, `SELECT count(*) FROM pilot_allowlist`); n != 1 {
		t.Fatalf("allowlist rows = %d, want 1", n)
	}
}

func TestPilotAllowlist_RefusesABadRequest(t *testing.T) {
	db := testdb.New(t)
	srv := newPilotServer(t, db, someUID, testStaffEmail)
	defer srv.Close()

	for name, body := range map[string]string{
		"truncated json":   `{"address": `,
		"empty address":    `{"address": "  ", "addedBy": "Mark"}`,
		"no at sign":       `{"address": "ana.example.com", "addedBy": "Mark"}`,
		"nothing after at": `{"address": "ana@", "addedBy": "Mark"}`,
		"no addedBy":       `{"address": "ana@example.com", "addedBy": " "}`,
	} {
		resp := postPilot(t, srv, pilotAllowlistURL, pilotTestSecret, body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, resp.StatusCode)
		}
	}
	if n := countRows(t, db, `SELECT count(*) FROM pilot_allowlist`); n != 0 {
		t.Fatalf("allowlist rows = %d, want 0", n)
	}
}

func TestPilotEndpoints_RefuseWithoutTheInternalCredential(t *testing.T) {
	db := testdb.New(t)
	srv := newPilotServer(t, db, someUID, testStaffEmail)
	defer srv.Close()

	for _, path := range []string{pilotAllowlistURL, pilotGateOpenURL} {
		for _, secret := range []string{"", "wrong-secret"} {
			resp := postPilot(t, srv, path, secret, `{"address": "ana@example.com", "addedBy": "Mark", "openedBy": "Mark"}`)
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("%s with secret %q: status = %d, want 401", path, secret, resp.StatusCode)
			}
		}
	}
	if n := countRows(t, db, `SELECT count(*) FROM pilot_allowlist`); n != 0 {
		t.Fatalf("allowlist rows = %d, want 0", n)
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
