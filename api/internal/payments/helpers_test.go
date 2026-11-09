package payments_test

import (
	"database/sql"
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

const (
	notAUUID           = "not-a-uuid"
	unknownID          = "00000000-0000-0000-0000-000000000000"
	statusDeadLettered = "dead_lettered"
	reasonGiven        = "reason"

	caseBodyNotJSON      = "a body that is not JSON"
	caseInvoiceIDNotUUID = "an Invoice id that is not a uuid"
	caseDoula            = "a Doula"
)

// env is what a test in this package starts from: a Practice with this
// package's whole surface mounted on a server through payments.Mount, one
// Staff member at the screen and the fake Stripe client the handlers talk
// to. With no option she is the Practice's Owner and the Stripe client is
// a fresh FakeClient; a test says only what it changes, with signedInAs,
// withStripe, withEnqueuer, atInstant and withDeclaredWebsite.
type env struct {
	*fixture.Scene

	Stripe *payments.FakeClient

	// AccountID is the Stripe Connect account connected() gave the Practice.
	AccountID string
}

type envSpec struct {
	staff   []fixture.StaffOption
	scene   []fixture.SceneOption
	stripe  *payments.FakeClient
	enq     tasknudge.Enqueuer
	website string

	billingMode payments.BillingMode
	connect     *connectSpec
}

type connectSpec struct {
	cardStatus   string
	billingUnset bool
}

type envOption func(*envSpec)

// connected gives the Practice a Stripe Connect account that can take a
// Client's payment: an account the fake Stripe client made, with
// card_payments active and the billing mode answered "stripe".
func connected() envOption { return connectedWithCardStatus("active") }

// connectedWithCardStatus is connected with an explicit card_payments
// status, for an account left restricted by an abandoned onboarding.
func connectedWithCardStatus(status string) envOption {
	return func(s *envSpec) { s.connect = &connectSpec{cardStatus: status} }
}

// connectAccountOnly gives the Practice a Stripe Connect account id and
// nothing else, the way an onboarding that has not yet been confirmed by a
// webhook leaves it.
func connectAccountOnly() envOption {
	return func(s *envSpec) { s.connect = &connectSpec{} }
}

// connectedBillingUnset is connected with the billing-mode question still
// unanswered, for the first Invoice that answers it.
func connectedBillingUnset() envOption {
	return func(s *envSpec) { s.connect = &connectSpec{cardStatus: "active", billingUnset: true} }
}

func viaScene(opts ...fixture.SceneOption) envOption {
	return func(s *envSpec) { s.scene = append(s.scene, opts...) }
}

// signedInAs replaces the Owner at the screen with a Staff member of
// these options.
func signedInAs(opts ...fixture.StaffOption) envOption {
	return func(s *envSpec) { s.staff = opts }
}

// withStripe hands the server a FakeClient the test has already shaped.
func withStripe(c *payments.FakeClient) envOption {
	return func(s *envSpec) { s.stripe = c }
}

// withEnqueuer replaces the no-op Enqueuer, for a test that asserts on the
// nudge the handler enqueues.
func withEnqueuer(enq tasknudge.Enqueuer) envOption {
	return func(s *envSpec) { s.enq = enq }
}

// atInstant fixes the server's clock at now, the way main.go seeds the
// request clock through clock.Middleware. The session is minted at the
// same instant, so the fixed clock does not read it as expired.
func atInstant(now time.Time) envOption { return viaScene(fixture.At(now)) }

// withDeclaredWebsite records the website answer #440 collects, which #442
// makes a precondition of starting Connect onboarding.
func withDeclaredWebsite(ownURL string) envOption {
	return func(s *envSpec) { s.website = ownURL }
}

// withBillingMode records the Practice's answer to the billing-mode
// question.
func withBillingMode(mode payments.BillingMode) envOption {
	return func(s *envSpec) { s.billingMode = mode }
}

func newEnv(t *testing.T, opts ...envOption) *env {
	t.Helper()
	spec := envSpec{enq: tasknudge.NoOpEnqueuer{}, staff: []fixture.StaffOption{fixture.Owner()}}
	for _, opt := range opts {
		opt(&spec)
	}
	if spec.stripe == nil {
		spec.stripe = payments.NewFakeClient()
	}
	mount := func(g *staffauth.GatedRouter, ir *idempotency.Router, app *sql.DB) {
		payments.Mount(g, ir, spec.stripe, spec.enq, app, testPublishableKey)
	}
	sceneOpts := append([]fixture.SceneOption{fixture.SignedInAs(spec.staff...)}, spec.scene...)
	e := &env{Scene: fixture.NewScene(t, mount, sceneOpts...), Stripe: spec.stripe}
	if spec.website != "" {
		seedDeclaredWebsite(t, e.DB, e.Practice.ID, spec.website)
	}
	if c := spec.connect; c != nil {
		e.connect(c)
	}
	if spec.billingMode != "" {
		testdb.SeedBillingMode(t, e.DB, e.Practice.ID, string(spec.billingMode))
	}
	return e
}

func (e *env) connect(c *connectSpec) {
	e.T.Helper()
	accountID, err := e.Stripe.CreateAccount(e.T.Context(), payments.AccountProfile{
		PracticeID:   e.Practice.ID,
		PracticeName: fixturePracticeName,
		BusinessURL:  fixtureOwnSiteURL,
	})
	if err != nil {
		e.T.Fatalf("fixture CreateAccount: %v", err)
	}
	e.AccountID = accountID
	if c.cardStatus == "" {
		if _, err := e.DB.Admin.ExecContext(e.T.Context(), `UPDATE practices SET stripe_connect_account_id = $1 WHERE id = $2`, accountID, e.Practice.ID); err != nil {
			e.T.Fatalf("seed connect account id: %v", err)
		}
		return
	}
	seedConnectAccountWithCardStatus(e.T, e.DB, e.Practice.ID, accountID, c.cardStatus)
	if c.billingUnset {
		if _, err := e.DB.Admin.ExecContext(e.T.Context(), `UPDATE practices SET billing_mode = NULL WHERE id = $1`, e.Practice.ID); err != nil {
			e.T.Fatalf("unset billing mode: %v", err)
		}
	}
}

// postRefused POSTs one request as the Staff member at the screen, requires
// the status want, and returns the error envelope of docs/api-design.md
// section 7 it carries.
func (e *env) postRefused(path string, want int, opts ...reqtest.Option) apierr.APIError {
	e.T.Helper()
	return reqtest.Expect[apierr.APIError](e.T, http.MethodPost, e.PracticeURL(path), want, append([]reqtest.Option{e.SignedIn}, opts...)...)
}

// expect sends one request as the Staff member at the screen and requires
// the status, for a write or a refusal whose body the test does not read.
func (e *env) expect(method, path string, want int, opts ...reqtest.Option) {
	e.T.Helper()
	reqtest.ExpectStatus(e.T, method, e.PracticeURL(path), want, append([]reqtest.Option{e.SignedIn}, opts...)...)
}
