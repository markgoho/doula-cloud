package payments_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// errStripeFake is returned by FakeClient methods in tests that exercise a
// handler's Stripe-failure path.
var errStripeFake = errors.New("stripe: fake failure")

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

// seedOwner seeds a Practice and a Staff member holding the owner role
// there -- PostConnectHandler is Owner-only, unlike GetConnectStatusHandler.
func seedOwner(t *testing.T, db *testdb.DB, identityUID string) (practiceID string) {
	t.Helper()
	practiceID = seedPractice(t, db, "Test Practice")
	staffID := seedStaff(t, db, identityUID)
	seedMembership(t, db, practiceID, staffID, "{owner}")
	return practiceID
}

// seedMember seeds a Practice and a Staff member holding a doula
// (non-Owner) role there.
func seedMember(t *testing.T, db *testdb.DB, identityUID string) (practiceID string) {
	t.Helper()
	practiceID = seedPractice(t, db, "Test Practice")
	staffID := seedStaff(t, db, identityUID)
	seedMembership(t, db, practiceID, staffID, "{doula}")
	return practiceID
}

func stripeConnectAccountID(t *testing.T, db *testdb.DB, practiceID string) *string {
	t.Helper()
	var id *string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT stripe_connect_account_id FROM practices WHERE id = $1`, practiceID).Scan(&id); err != nil {
		t.Fatalf("query stripe_connect_account_id: %v", err)
	}
	return id
}

func newConnectServer(t *testing.T, db *testdb.DB, uid string, client payments.Client) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("POST /practices/{practiceId}/payments/connect",
		staffauth.Middleware(db.App)(payments.PostConnectHandler(client)))
	mux.Handle("GET /practices/{practiceId}/payments/connect",
		staffauth.Middleware(db.App)(payments.GetConnectStatusHandler(client)))
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func postConnect(t *testing.T, srv *httptest.Server, session string, practiceID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		srv.URL+"/practices/"+practiceID+"/payments/connect", bytes.NewBufferString(``))
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

func getConnectStatus(t *testing.T, srv *httptest.Server, session string, practiceID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/practices/"+practiceID+"/payments/connect", nil)
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

// TestPostConnectHandler_OwnerCreatesAccountAndAccountLink proves an
// Owner's first connect attempt lazily creates a Stripe Connect account,
// persists its id on the Practice, and returns an onboarding URL.
func TestPostConnectHandler_OwnerCreatesAccountAndAccountLink(t *testing.T) {
	db := testdb.New(t)
	const uid = "connect-owner"
	practiceID := seedOwner(t, db, uid)
	client := payments.NewFakeClient()

	srv, session := newConnectServer(t, db, uid, client)
	defer srv.Close()

	resp := postConnect(t, srv, session, practiceID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out payments.ConnectResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.OnboardingURL == "" {
		t.Fatal("onboardingUrl is empty")
	}

	if got := client.AccountLinkCallCount(); got != 1 {
		t.Fatalf("CreateAccountLink calls = %d, want 1", got)
	}
	link := client.AccountLinkCalls[0]
	if link.PracticeID != practiceID {
		t.Fatalf("account link call practiceID = %q, want %q", link.PracticeID, practiceID)
	}

	id := stripeConnectAccountID(t, db, practiceID)
	if id == nil || *id == "" {
		t.Fatal("stripe_connect_account_id was not persisted")
	}
	if link.AccountID != *id {
		t.Fatalf("account link call account id = %q, want %q", link.AccountID, *id)
	}
}

// TestPostConnectHandler_SecondAttemptReusesExistingAccount proves a
// Practice's second connect attempt does not create a second Stripe
// Connect account.
func TestPostConnectHandler_SecondAttemptReusesExistingAccount(t *testing.T) {
	db := testdb.New(t)
	const uid = "connect-owner-repeat"
	practiceID := seedOwner(t, db, uid)
	client := payments.NewFakeClient()

	srv, session := newConnectServer(t, db, uid, client)
	defer srv.Close()

	first := postConnect(t, srv, session, practiceID)
	_ = first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first connect status = %d, want %d", first.StatusCode, http.StatusOK)
	}
	firstAccountID := stripeConnectAccountID(t, db, practiceID)

	second := postConnect(t, srv, session, practiceID)
	_ = second.Body.Close()
	if second.StatusCode != http.StatusOK {
		t.Fatalf("second connect status = %d, want %d", second.StatusCode, http.StatusOK)
	}

	if got := client.AccountCallCount(); got != 1 {
		t.Fatalf("CreateAccount call count = %d, want 1", got)
	}
	if got := stripeConnectAccountID(t, db, practiceID); got == nil || *got != *firstAccountID {
		t.Fatalf("stripe_connect_account_id changed across attempts: first %v, second %v", firstAccountID, got)
	}
	if got := client.AccountLinkCallCount(); got != 2 {
		t.Fatalf("CreateAccountLink calls = %d, want 2", got)
	}
}

// TestPostConnectHandler_NonOwnerForbidden proves a non-Owner Staff member
// cannot initiate a Stripe Connect connection.
func TestPostConnectHandler_NonOwnerForbidden(t *testing.T) {
	db := testdb.New(t)
	const uid = "connect-non-owner"
	practiceID := seedMember(t, db, uid) // doula role, not owner
	client := payments.NewFakeClient()

	srv, session := newConnectServer(t, db, uid, client)
	defer srv.Close()

	resp := postConnect(t, srv, session, practiceID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
	if got := client.AccountLinkCallCount(); got != 0 {
		t.Fatalf("CreateAccountLink calls = %d, want 0", got)
	}
}

// TestPostConnectHandler_CreateAccountFailureReturns500 proves a Stripe
// account-creation failure surfaces as an internal error and never
// persists an account id.
func TestPostConnectHandler_CreateAccountFailureReturns500(t *testing.T) {
	db := testdb.New(t)
	const uid = "connect-account-fail"
	practiceID := seedOwner(t, db, uid)
	client := payments.NewFakeClient()
	client.CreateAccountErr = errStripeFake

	srv, session := newConnectServer(t, db, uid, client)
	defer srv.Close()

	resp := postConnect(t, srv, session, practiceID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
	if got := stripeConnectAccountID(t, db, practiceID); got != nil {
		t.Fatalf("stripe_connect_account_id = %v, want nil (never persisted)", got)
	}
}

// TestPostConnectHandler_CreateAccountLinkFailureReturns500 proves an
// Account Link-creation failure surfaces as an internal error.
func TestPostConnectHandler_CreateAccountLinkFailureReturns500(t *testing.T) {
	db := testdb.New(t)
	const uid = "connect-link-fail"
	practiceID := seedOwner(t, db, uid)
	client := payments.NewFakeClient()
	client.CreateAccountLinkErr = errStripeFake

	srv, session := newConnectServer(t, db, uid, client)
	defer srv.Close()

	resp := postConnect(t, srv, session, practiceID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}

// TestGetConnectStatusHandler_NotConnected proves a Practice with no
// stored Stripe Connect account id reports not_connected without calling
// Stripe.
func TestGetConnectStatusHandler_NotConnected(t *testing.T) {
	db := testdb.New(t)
	const uid = "status-not-connected"
	practiceID := seedMember(t, db, uid)
	client := payments.NewFakeClient()

	srv, session := newConnectServer(t, db, uid, client)
	defer srv.Close()

	resp := getConnectStatus(t, srv, session, practiceID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out payments.ConnectStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Status != payments.StatusNotConnected {
		t.Fatalf("status = %q, want %q", out.Status, payments.StatusNotConnected)
	}
	if len(client.RetrieveCalls) != 0 {
		t.Fatalf("RetrieveAccount calls = %d, want 0", len(client.RetrieveCalls))
	}
}

// TestGetConnectStatusHandler_OnboardingIncomplete proves a connected
// account that still owes Stripe information reports
// onboarding_incomplete, carries the outstanding requirement paths, and
// that any Staff member (not just an Owner) can read it.
func TestGetConnectStatusHandler_OnboardingIncomplete(t *testing.T) {
	db := testdb.New(t)
	const uid = "status-incomplete"
	practiceID := seedMember(t, db, uid) // doula role, not owner
	client := payments.NewFakeClient()

	srv, session := newConnectServer(t, db, uid, client)
	defer srv.Close()

	connectResp := postConnectAsOwnerForStatusFixture(t, db, client, practiceID)
	client.Statuses[connectResp] = payments.AccountStatus{
		CardPayments:    payments.CapabilityRestricted,
		Payouts:         payments.CapabilityRestricted,
		RequirementsDue: []string{requirementMCC},
	}

	resp := getConnectStatus(t, srv, session, practiceID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out payments.ConnectStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Status != payments.StatusOnboardingIncomplete {
		t.Fatalf("status = %q, want %q", out.Status, payments.StatusOnboardingIncomplete)
	}
	if out.CardPaymentsStatus != payments.CapabilityRestricted || out.PayoutsStatus != payments.CapabilityRestricted {
		t.Fatalf("response = %+v, want both capabilities restricted", out)
	}
	if len(out.RequirementsDue) != 1 || out.RequirementsDue[0] != requirementMCC {
		t.Fatalf("requirementsDue = %v, want the one outstanding Stripe field path", out.RequirementsDue)
	}
}

// TestGetConnectStatusHandler_Pending proves the state v1's booleans
// could not express: Stripe is reviewing what the Owner already
// supplied, so nothing is outstanding and nothing works yet. It must not
// read as onboarding_incomplete -- there is nothing left to fill in.
func TestGetConnectStatusHandler_Pending(t *testing.T) {
	db := testdb.New(t)
	const uid = "status-pending"
	practiceID := seedOwner(t, db, uid)
	client := payments.NewFakeClient()

	accountID := postConnectAsOwnerForStatusFixture(t, db, client, practiceID)
	client.Statuses[accountID] = payments.AccountStatus{
		CardPayments: payments.CapabilityPending,
		Payouts:      payments.CapabilityPending,
	}

	srv, session := newConnectServer(t, db, uid, client)
	defer srv.Close()

	resp := getConnectStatus(t, srv, session, practiceID)
	defer resp.Body.Close()

	var out payments.ConnectStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Status != payments.StatusPending {
		t.Fatalf("status = %q, want %q", out.Status, payments.StatusPending)
	}
	if out.RequirementsDue == nil {
		t.Fatalf("requirementsDue = nil, want an empty list rather than a missing one")
	}
}

// TestGetConnectStatusHandler_FreshAccountReportsOnboardingIncomplete
// covers the state a just-created account is in: Stripe has granted
// nothing and reported no requirements yet. The Owner has everything
// still to do, so this must not read as `pending` -- the screen would
// hide the button and tell them to wait for a review that has not been
// asked for.
func TestGetConnectStatusHandler_FreshAccountReportsOnboardingIncomplete(t *testing.T) {
	db := testdb.New(t)
	const uid = "status-fresh"
	practiceID := seedOwner(t, db, uid)
	client := payments.NewFakeClient()

	accountID := postConnectAsOwnerForStatusFixture(t, db, client, practiceID)
	client.Statuses[accountID] = payments.AccountStatus{
		CardPayments: payments.CapabilityUnsupported,
		Payouts:      payments.CapabilityUnsupported,
	}

	srv, session := newConnectServer(t, db, uid, client)
	defer srv.Close()

	resp := getConnectStatus(t, srv, session, practiceID)
	defer resp.Body.Close()

	var out payments.ConnectStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Status != payments.StatusOnboardingIncomplete {
		t.Fatalf("status = %q, want %q", out.Status, payments.StatusOnboardingIncomplete)
	}
}

// TestGetConnectStatusHandler_MixedStateWithRequirementsIsNotPending
// pins the order in deriveStatus. The two capabilities move
// independently, so card_payments can be restricted while payouts is
// pending. Reading that as `pending` would hide the onboarding button
// while the screen still listed what Stripe was waiting on -- the Owner
// would see the ask and have no way to answer it.
func TestGetConnectStatusHandler_MixedStateWithRequirementsIsNotPending(t *testing.T) {
	db := testdb.New(t)
	const uid = "status-mixed"
	practiceID := seedOwner(t, db, uid)
	client := payments.NewFakeClient()

	accountID := postConnectAsOwnerForStatusFixture(t, db, client, practiceID)
	client.Statuses[accountID] = payments.AccountStatus{
		CardPayments:    payments.CapabilityRestricted,
		Payouts:         payments.CapabilityPending,
		RequirementsDue: []string{requirementMCC},
	}

	srv, session := newConnectServer(t, db, uid, client)
	defer srv.Close()

	resp := getConnectStatus(t, srv, session, practiceID)
	defer resp.Body.Close()

	var out payments.ConnectStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Status != payments.StatusOnboardingIncomplete {
		t.Fatalf("status = %q, want %q -- the Owner still has something to supply", out.Status, payments.StatusOnboardingIncomplete)
	}
}

// TestGetConnectStatusHandler_PayoutsRestricted proves the other state a
// single boolean pair collapsed: Clients can pay, but the money cannot
// reach the Practice's bank yet. Reporting this as onboarding_incomplete
// would read as if invoicing were broken, which it is not.
func TestGetConnectStatusHandler_PayoutsRestricted(t *testing.T) {
	db := testdb.New(t)
	const uid = "status-payouts-restricted"
	practiceID := seedOwner(t, db, uid)
	client := payments.NewFakeClient()

	accountID := postConnectAsOwnerForStatusFixture(t, db, client, practiceID)
	client.Statuses[accountID] = payments.AccountStatus{
		CardPayments:    payments.CapabilityActive,
		Payouts:         payments.CapabilityRestricted,
		RequirementsDue: []string{"configuration.merchant.bank_account"},
	}

	srv, session := newConnectServer(t, db, uid, client)
	defer srv.Close()

	resp := getConnectStatus(t, srv, session, practiceID)
	defer resp.Body.Close()

	var out payments.ConnectStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Status != payments.StatusPayoutsRestricted {
		t.Fatalf("status = %q, want %q", out.Status, payments.StatusPayoutsRestricted)
	}
}

// TestGetConnectStatusHandler_Active proves a fully-onboarded account
// reports active.
func TestGetConnectStatusHandler_Active(t *testing.T) {
	db := testdb.New(t)
	const uid = "status-active"
	practiceID := seedOwner(t, db, uid)
	client := payments.NewFakeClient()

	accountID := postConnectAsOwnerForStatusFixture(t, db, client, practiceID)
	client.Statuses[accountID] = payments.AccountStatus{
		CardPayments: payments.CapabilityActive,
		Payouts:      payments.CapabilityActive,
	}

	srv, session := newConnectServer(t, db, uid, client)
	defer srv.Close()

	resp := getConnectStatus(t, srv, session, practiceID)
	defer resp.Body.Close()

	var out payments.ConnectStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Status != payments.StatusActive {
		t.Fatalf("status = %q, want %q", out.Status, payments.StatusActive)
	}
}

// TestGetConnectStatusHandler_RetrieveFailureReturns500 proves a Stripe
// Account-retrieve failure surfaces as an internal error.
func TestGetConnectStatusHandler_RetrieveFailureReturns500(t *testing.T) {
	db := testdb.New(t)
	const uid = "status-retrieve-fail"
	practiceID := seedOwner(t, db, uid)
	client := payments.NewFakeClient()
	postConnectAsOwnerForStatusFixture(t, db, client, practiceID)
	client.RetrieveAccountErr = errStripeFake

	srv, session := newConnectServer(t, db, uid, client)
	defer srv.Close()

	resp := getConnectStatus(t, srv, session, practiceID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}

// postConnectAsOwnerForStatusFixture seeds a stripe_connect_account_id on
// practiceID directly (bypassing HTTP, since the caller for a status test
// may not hold the owner role needed to call PostConnectHandler) and
// returns the account id, so RetrieveAccount tests can control what
// FakeClient.Statuses reports for it.
func postConnectAsOwnerForStatusFixture(t *testing.T, db *testdb.DB, client *payments.FakeClient, practiceID string) string {
	t.Helper()
	accountID, err := client.CreateAccount(t.Context(), practiceID, "Fixture Practice")
	if err != nil {
		t.Fatalf("CreateAccount fixture: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET stripe_connect_account_id = $1 WHERE id = $2`, accountID, practiceID,
	); err != nil {
		t.Fatalf("seed stripe_connect_account_id: %v", err)
	}
	return accountID
}

// TestPostConnectHandler_PassesPracticeNameToStripe pins what #247's walk
// found the hard way: with no display_name on the v2 Account, Stripe
// falls back to the statement descriptor, and the Client's hosted invoice
// said it was "From DOULA.CLOU" rather than from the Practice they hired.
// The Practice's own name has to reach CreateAccount.
func TestPostConnectHandler_PassesPracticeNameToStripe(t *testing.T) {
	db := testdb.New(t)
	const uid = "connect-display-name"
	practiceID := seedOwner(t, db, uid)
	client := payments.NewFakeClient()

	srv, session := newConnectServer(t, db, uid, client)
	defer srv.Close()

	resp := postConnect(t, srv, session, practiceID)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if len(client.AccountNames) != 1 {
		t.Fatalf("AccountNames = %v, want exactly one CreateAccount call", client.AccountNames)
	}
	var want string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT name FROM practices WHERE id = $1`, practiceID).Scan(&want); err != nil {
		t.Fatalf("read practice name: %v", err)
	}
	if client.AccountNames[0] != want {
		t.Fatalf("display name sent to Stripe = %q, want the Practice's own name %q", client.AccountNames[0], want)
	}
}
