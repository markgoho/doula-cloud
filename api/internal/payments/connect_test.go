package payments_test

import (
	"errors"
	"net/http"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// errStripeFake is returned by FakeClient methods in tests that exercise a
// handler's Stripe-failure path.
var errStripeFake = errors.New("stripe: fake failure")

// The fixture Practice and the website she declared, named once because
// several tests seed the same pair and because a test reading "the same
// answer as last time" should be looking at the same literal.
const (
	fixturePracticeName = "Fixture Practice"
	fixtureOwnSiteURL   = "https://rochesterdoulas.com"
	// The Practice the fixture package creates, and so the descriptor its
	// name makes.
	seededPracticeName = "Test Practice"
	// ownerRole and doulaRole are named once so golangci-lint's goconst
	// check doesn't see repeated "owner"/"doula" literals across this
	// package's whole test surface.
	ownerRole = "owner"
	adminRole = "admin"
	doulaRole = "doula"

	connectPath = "/payments/connect"
)

// seedDeclaredWebsite records the website answer #440 collects, which
// #442 makes a precondition of starting Connect onboarding. Written
// straight to the table rather than through website.PutHandler: this
// package's tests are about the payments handler, and the declaration is
// a fixture for them, not the thing under test.
func seedDeclaredWebsite(t *testing.T, db *testdb.DB, practiceID, ownURL string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_websites (practice_id, mode, own_url) VALUES ($1, 'own', $2)`,
		practiceID, ownURL,
	); err != nil {
		t.Fatalf("seed declared website: %v", err)
	}
}

// seedPaymentsHostedPage records the other answer: a page published
// here, at the slug 00046 mints once, at an existing practiceID. Stays
// local under this name rather than testdb: sitebuild's own
// seedHostedPage creates its own fresh Practice too, a different shape
// this package's connect-status tests don't need.
func seedPaymentsHostedPage(t *testing.T, db *testdb.DB, practiceID, slug, description string) {
	t.Helper()
	seedHostedPageInState(t, db, practiceID, slug, description, "pending")
}

func seedHostedPageInState(t *testing.T, db *testdb.DB, practiceID, slug, description, pageState string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_websites (practice_id, mode, service_description, cancellation_policy, slug, page_state)
		 VALUES ($1, 'hosted', $2, 'Cancel any time up to two weeks before the due date.', $3, $4::practice_page_state)`,
		practiceID, description, slug, pageState,
	); err != nil {
		t.Fatalf("seed hosted page: %v", err)
	}
}

func stripeConnectAccountID(t *testing.T, db *testdb.DB, practiceID string) *string {
	t.Helper()
	id := testdb.Scalar[*string](t, db.Admin, `SELECT stripe_connect_account_id FROM practices WHERE id = $1`, practiceID)
	return id
}

// connectAccount gives the Practice a Stripe Connect account directly
// (bypassing HTTP, since the Staff member at the screen may not hold the
// owner role PostConnectHandler needs) and returns the account id, so
// RetrieveAccount tests can control what FakeClient.Statuses reports for
// it.
func (e *env) connectAccount() string {
	e.t.Helper()
	accountID, err := e.Stripe.CreateAccount(e.t.Context(), payments.AccountProfile{
		PracticeID:   e.Practice.ID,
		PracticeName: fixturePracticeName,
		BusinessURL:  fixtureOwnSiteURL,
	})
	if err != nil {
		e.t.Fatalf("CreateAccount fixture: %v", err)
	}
	if _, err := e.db.Admin.ExecContext(e.t.Context(),
		`UPDATE practices SET stripe_connect_account_id = $1 WHERE id = $2`, accountID, e.Practice.ID,
	); err != nil {
		e.t.Fatalf("seed stripe_connect_account_id: %v", err)
	}
	return accountID
}

// connectStatus reads the Practice's Connect status as the Staff member
// at the screen.
func (e *env) connectStatus() payments.ConnectStatusResponse {
	e.t.Helper()
	return reqtest.Expect[payments.ConnectStatusResponse](e.t, http.MethodGet, e.url(connectPath), http.StatusOK, e.signedIn)
}

// TestPostConnectHandler_OwnerCreatesAccountAndAccountLink proves an
// Owner's first connect attempt lazily creates a Stripe Connect account,
// persists its id on the Practice, and returns an onboarding URL.
func TestPostConnectHandler_OwnerCreatesAccountAndAccountLink(t *testing.T) {
	e := newEnv(t, withDeclaredWebsite(fixtureOwnSiteURL))

	out := reqtest.Expect[payments.ConnectResponse](t, http.MethodPost, e.url(connectPath), http.StatusOK, e.signedIn)
	if out.OnboardingURL == "" {
		t.Fatal("onboardingUrl is empty")
	}

	if got := e.Stripe.AccountLinkCallCount(); got != 1 {
		t.Fatalf("CreateAccountLink calls = %d, want 1", got)
	}
	link := e.Stripe.AccountLinkCalls[0]
	if link.PracticeID != e.Practice.ID {
		t.Fatalf("account link call practiceID = %q, want %q", link.PracticeID, e.Practice.ID)
	}

	id := stripeConnectAccountID(t, e.db, e.Practice.ID)
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
	e := newEnv(t, withDeclaredWebsite(fixtureOwnSiteURL))

	e.expect(http.MethodPost, connectPath, http.StatusOK)
	firstAccountID := stripeConnectAccountID(t, e.db, e.Practice.ID)
	e.expect(http.MethodPost, connectPath, http.StatusOK)

	if got := e.Stripe.AccountCallCount(); got != 1 {
		t.Fatalf("CreateAccount call count = %d, want 1", got)
	}
	if got := stripeConnectAccountID(t, e.db, e.Practice.ID); got == nil || *got != *firstAccountID {
		t.Fatalf("stripe_connect_account_id changed across attempts: first %v, second %v", firstAccountID, got)
	}
	if got := e.Stripe.AccountLinkCallCount(); got != 2 {
		t.Fatalf("CreateAccountLink calls = %d, want 2", got)
	}
}

// TestPostConnectHandler_OnlyTheOwnerMayConnect proves the write gate: an
// Admin may read Connect state but may not start or resume hosted
// onboarding, which stays the Owner's alone (#267), and a Doula may not
// either.
func TestPostConnectHandler_OnlyTheOwnerMayConnect(t *testing.T) {
	for _, tc := range []struct {
		name  string
		staff fixture.StaffOption
	}{
		{doulaRole, fixture.Doula()},
		{adminRole, fixture.Admin()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.staff))

			e.expect(http.MethodPost, connectPath, http.StatusForbidden)

			if got := e.Stripe.AccountLinkCallCount(); got != 0 {
				t.Fatalf("CreateAccountLink calls = %d, want 0", got)
			}
		})
	}
}

// TestPostConnectHandler_StripeFailureReturns500 proves a Stripe failure
// surfaces as an internal error, and that an account-creation failure
// never persists an account id.
func TestPostConnectHandler_StripeFailureReturns500(t *testing.T) {
	for _, tc := range []struct {
		name    string
		breakIt func(*payments.FakeClient)
	}{
		{"CreateAccount", func(c *payments.FakeClient) { c.CreateAccountErr = errStripeFake }},
		{"CreateAccountLink", func(c *payments.FakeClient) { c.CreateAccountLinkErr = errStripeFake }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stripe := payments.NewFakeClient()
			tc.breakIt(stripe)
			e := newEnv(t, withStripe(stripe), withDeclaredWebsite(fixtureOwnSiteURL))

			e.expect(http.MethodPost, connectPath, http.StatusInternalServerError)

			if tc.name == "CreateAccount" {
				if got := stripeConnectAccountID(t, e.db, e.Practice.ID); got != nil {
					t.Fatalf("stripe_connect_account_id = %v, want nil (never persisted)", got)
				}
			}
		})
	}
}

// TestGetConnectStatusHandler_NotConnected proves a Practice with no
// stored Stripe Connect account id reports not_connected without calling
// Stripe.
func TestGetConnectStatusHandler_NotConnected(t *testing.T) {
	e := newEnv(t)

	out := e.connectStatus()

	if out.Status != payments.StatusNotConnected {
		t.Fatalf("status = %q, want %q", out.Status, payments.StatusNotConnected)
	}
	if len(e.Stripe.RetrieveCalls) != 0 {
		t.Fatalf("RetrieveAccount calls = %d, want 0", len(e.Stripe.RetrieveCalls))
	}
}

// TestGetConnectStatusHandler_AdminReadsStatus proves an Admin who holds
// no Owner role reads the Practice's Stripe Connect status through the
// real GatedRouter mount: ADR-0008's Stripe Connect state row is Owner
// yes, Admin yes (#267), the same pair the Invoice-history row already
// carries.
func TestGetConnectStatusHandler_AdminReadsStatus(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Admin()))

	if out := e.connectStatus(); out.Status != payments.StatusNotConnected {
		t.Fatalf("status = %q, want %q", out.Status, payments.StatusNotConnected)
	}
}

// TestGetConnectStatusHandler_DoulaForbidden proves a Doula cannot read
// the Practice's Stripe Connect status through the real GatedRouter
// mount: ADR-0008's Stripe Connect state row is a no for a Doula of
// either employment type (#267). Both employment types, because the row
// says both: a contractor is confined to what she is attached to, and an
// employee Doula has no stake in the payment rail either.
func TestGetConnectStatusHandler_DoulaForbidden(t *testing.T) {
	for _, tc := range []struct {
		name  string
		staff []fixture.StaffOption
	}{
		{"employee", []fixture.StaffOption{fixture.Doula()}},
		{"contractor", []fixture.StaffOption{fixture.Doula(), fixture.Contractor()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.staff...))

			e.expect(http.MethodGet, connectPath, http.StatusForbidden)
		})
	}
}

// TestGetConnectStatusHandler_OnboardingIncomplete proves a connected
// account that still owes Stripe information reports
// onboarding_incomplete and carries the outstanding requirement paths.
func TestGetConnectStatusHandler_OnboardingIncomplete(t *testing.T) {
	e := newEnv(t)
	e.Stripe.Statuses[e.connectAccount()] = payments.AccountStatus{
		CardPayments:    payments.CapabilityRestricted,
		Payouts:         payments.CapabilityRestricted,
		RequirementsDue: []string{requirementMCC},
	}

	out := e.connectStatus()

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
	e := newEnv(t)
	e.Stripe.Statuses[e.connectAccount()] = payments.AccountStatus{
		CardPayments: payments.CapabilityPending,
		Payouts:      payments.CapabilityPending,
	}

	out := e.connectStatus()

	if out.Status != payments.StatusPending {
		t.Fatalf("status = %q, want %q", out.Status, payments.StatusPending)
	}
	if out.RequirementsDue == nil {
		t.Fatalf("requirementsDue = nil, want an empty list rather than a missing one")
	}
}

// TestGetConnectStatusHandler_DerivesTheStatusFromTheCapabilities pins the
// order in deriveStatus. The two capabilities move independently, so each
// row is a combination Stripe can report and the one status the screen
// must show for it.
func TestGetConnectStatusHandler_DerivesTheStatusFromTheCapabilities(t *testing.T) {
	for _, tc := range []struct {
		name   string
		stripe payments.AccountStatus
		want   payments.ConnectStatus
	}{
		{
			// A just-created account: Stripe has granted nothing and reported
			// no requirements yet. The Owner has everything still to do, so
			// this must not read as pending -- the screen would hide the
			// button and tell her to wait for a review that has not been
			// asked for.
			name:   "a fresh account reports onboarding_incomplete",
			stripe: payments.AccountStatus{CardPayments: payments.CapabilityUnsupported, Payouts: payments.CapabilityUnsupported},
			want:   payments.StatusOnboardingIncomplete,
		},
		{
			// card_payments restricted while payouts is pending. Reading that
			// as pending would hide the onboarding button while the screen
			// still listed what Stripe was waiting on -- the Owner would see
			// the ask and have no way to answer it.
			name: "a mixed state with requirements is not pending",
			stripe: payments.AccountStatus{
				CardPayments:    payments.CapabilityRestricted,
				Payouts:         payments.CapabilityPending,
				RequirementsDue: []string{requirementMCC},
			},
			want: payments.StatusOnboardingIncomplete,
		},
		{
			// Clients can pay, but the money cannot reach the Practice's bank
			// yet. onboarding_incomplete would read as if invoicing were
			// broken, which it is not.
			name: "restricted payouts with card payments active",
			stripe: payments.AccountStatus{
				CardPayments:    payments.CapabilityActive,
				Payouts:         payments.CapabilityRestricted,
				RequirementsDue: []string{"configuration.merchant.bank_account"},
			},
			want: payments.StatusPayoutsRestricted,
		},
		{
			name:   "a fully onboarded account is active",
			stripe: payments.AccountStatus{CardPayments: payments.CapabilityActive, Payouts: payments.CapabilityActive},
			want:   payments.StatusActive,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			e.Stripe.Statuses[e.connectAccount()] = tc.stripe

			if out := e.connectStatus(); out.Status != tc.want {
				t.Fatalf("status = %q, want %q", out.Status, tc.want)
			}
		})
	}
}

// TestGetConnectStatusHandler_RetrieveFailureReturns500 proves a Stripe
// Account-retrieve failure surfaces as an internal error.
func TestGetConnectStatusHandler_RetrieveFailureReturns500(t *testing.T) {
	e := newEnv(t)
	e.connectAccount()
	e.Stripe.RetrieveAccountErr = errStripeFake

	e.expect(http.MethodGet, connectPath, http.StatusInternalServerError)
}

// TestPostConnectHandler_PassesPracticeNameToStripe pins what #247's walk
// found the hard way: with no display_name on the v2 Account, Stripe
// falls back to the statement descriptor, and the Client's hosted invoice
// said it was "From DOULA.CLOU" rather than from the Practice they hired.
// The Practice's own name has to reach CreateAccount.
func TestPostConnectHandler_PassesPracticeNameToStripe(t *testing.T) {
	e := newEnv(t, withDeclaredWebsite(fixtureOwnSiteURL))

	e.expect(http.MethodPost, connectPath, http.StatusOK)

	if len(e.Stripe.AccountProfiles) != 1 {
		t.Fatalf("AccountProfiles = %v, want exactly one CreateAccount call", e.Stripe.AccountProfiles)
	}
	want := testdb.Scalar[string](t, e.db.Admin, `SELECT name FROM practices WHERE id = $1`, e.Practice.ID)
	if e.Stripe.AccountProfiles[0].PracticeName != want {
		t.Fatalf("display name sent to Stripe = %q, want the Practice's own name %q", e.Stripe.AccountProfiles[0].PracticeName, want)
	}
}

// TestPostConnectHandler_RefusesWithoutDeclaredWebsite is the boundary
// half of #442's block-over-warn rule. The screen hides the button, but
// the screen is not what enforces this: #421 walked what Stripe's hosted
// form does with an empty website field -- it accepts it, lets her finish
// every remaining step, and returns her "done" with card_payments
// restricted and nothing saying why. So no account is created and no
// Account Link is minted for a Practice who has not answered.
func TestPostConnectHandler_RefusesWithoutDeclaredWebsite(t *testing.T) {
	e := newEnv(t)

	out := e.postRefused(connectPath, http.StatusConflict)

	if out.Code != apierr.CodeFailedPrecondition {
		t.Fatalf("code = %q, want %q", out.Code, apierr.CodeFailedPrecondition)
	}
	if out.Message != payments.MsgWebsiteRequired {
		t.Fatalf("message = %q, want %q", out.Message, payments.MsgWebsiteRequired)
	}
	// The refusal has to land before the account is created, not merely
	// before the link is minted: a Stripe account made without a website
	// is one whose Owner is asked for it by hand, which is the failure
	// this ticket exists to remove.
	if got := e.Stripe.AccountCallCount(); got != 0 {
		t.Fatalf("CreateAccount calls = %d, want 0", got)
	}
	if got := e.Stripe.AccountLinkCallCount(); got != 0 {
		t.Fatalf("CreateAccountLink calls = %d, want 0", got)
	}
	if got := stripeConnectAccountID(t, e.db, e.Practice.ID); got != nil {
		t.Fatalf("stripe_connect_account_id = %v, want nil", got)
	}
}

// TestPostConnectHandler_RefusesAnAlreadyConnectedPracticeWithoutAWebsite
// proves the gate is on minting a link and not only on creating an
// account. A Practice connected before #440 existed has an account id and
// no declaration, and re-opening the hosted flow for her would walk her
// into the same empty-website trap.
func TestPostConnectHandler_RefusesAnAlreadyConnectedPracticeWithoutAWebsite(t *testing.T) {
	e := newEnv(t)
	e.connectAccount()

	e.expect(http.MethodPost, connectPath, http.StatusConflict)

	if got := e.Stripe.AccountLinkCallCount(); got != 0 {
		t.Fatalf("CreateAccountLink calls = %d, want 0", got)
	}
}

// TestPostConnectHandler_SendsHerOwnWebsiteToStripe proves the URL she
// declared is what the account is created with, so Stripe's hosted form
// never asks her for it -- the four requirements #442's Sandbox walk
// showed disappear when the account carries them at create.
func TestPostConnectHandler_SendsHerOwnWebsiteToStripe(t *testing.T) {
	e := newEnv(t, withDeclaredWebsite("https://facebook.com/rochester-doulas"))

	e.expect(http.MethodPost, connectPath, http.StatusOK)

	if len(e.Stripe.AccountProfiles) != 1 {
		t.Fatalf("AccountProfiles = %v, want exactly one CreateAccount call", e.Stripe.AccountProfiles)
	}
	got := e.Stripe.AccountProfiles[0]
	if got.BusinessURL != "https://facebook.com/rochester-doulas" {
		t.Fatalf("BusinessURL = %q, want the URL she declared", got.BusinessURL)
	}
	// #440 asks a Practice declaring her own site for nothing but the
	// address, so there is no product description to send and inventing
	// one would be putting words in her mouth on a form Stripe
	// underwrites her against.
	if got.ProductDescription != "" {
		t.Fatalf("ProductDescription = %q, want empty", got.ProductDescription)
	}
	// The descriptor comes from the Practice's name and not from the URL.
	// Deriving it from the URL is exactly what Stripe does when it is not
	// told one, and #421 watched that put FACEBOOK.COM/ROCHESTER onto a
	// walked account's Clients' card statements.
	if got.StatementDescriptor != seededPracticeName {
		t.Fatalf("StatementDescriptor = %q, want the Practice's name", got.StatementDescriptor)
	}
}

// TestPostConnectHandler_SendsHerHostedPageToStripe proves the other
// answer resolves to the address the page is actually published at, and
// carries the words she wrote as Stripe's product description.
func TestPostConnectHandler_SendsHerHostedPageToStripe(t *testing.T) {
	e := newEnv(t)
	seedPaymentsHostedPage(t, e.db, e.Practice.ID, "rochester-doulas", "Birth and postpartum doula support across Monroe County.")

	e.expect(http.MethodPost, connectPath, http.StatusOK)

	got := e.Stripe.AccountProfiles[0]
	if got.BusinessURL != "https://doula.cloud/p/rochester-doulas" {
		t.Fatalf("BusinessURL = %q, want her published page's address", got.BusinessURL)
	}
	if got.ProductDescription != "Birth and postpartum doula support across Monroe County." {
		t.Fatalf("ProductDescription = %q, want what she wrote on her page", got.ProductDescription)
	}
}

// TestPostConnectHandler_RefusesWhenHerPublishedPageDoesNotLoad is
// #443's half of the same block-over-warn rule. She answered, and the
// page we publish for her is not there -- so the URL Stripe would be
// handed 404s, and #382 established the review of that URL is ongoing
// with no published SLA, which makes the rejection arrive weeks later
// with no visible cause.
func TestPostConnectHandler_RefusesWhenHerPublishedPageDoesNotLoad(t *testing.T) {
	e := newEnv(t)
	seedHostedPageInState(t, e.db, e.Practice.ID, "test-practice", "Birth support.", "failed")

	out := e.postRefused(connectPath, http.StatusConflict)

	if out.Message != payments.MsgPageNotLive {
		t.Fatalf("message = %q, want %q", out.Message, payments.MsgPageNotLive)
	}
	if got := e.Stripe.AccountCallCount(); got != 0 {
		t.Fatalf("CreateAccount calls = %d, want 0", got)
	}
}

// A page still waiting for its deploy must not block her. Every
// Practice passes through "pending" on the way to "live", and blocking
// there would block the happy path.
func TestPostConnectHandler_AllowsAPageStillWaitingForItsDeploy(t *testing.T) {
	e := newEnv(t)
	seedHostedPageInState(t, e.db, e.Practice.ID, "test-practice", "Birth support.", "pending")

	e.expect(http.MethodPost, connectPath, http.StatusOK)
}
