package billing_test

import (
	"database/sql"
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/legal"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
)

// env is what a handler test in this package starts from: a Practice with
// this package's whole surface mounted through billing.Mount, the call
// main.go makes on the real GatedRouter and idempotency.Router, one Staff
// member at the screen and the fake Stripe client the handlers talk to.
// With no option she is the Practice's Owner; a test says only what it
// changes.
type env struct {
	*fixture.Scene

	Stripe *billing.FakeStripeClient
}

type envSpec struct {
	staff  []fixture.StaffOption
	stripe *billing.FakeStripeClient
	terms  legal.Document
	at     time.Time
}

type envOption func(*envSpec)

// withTerms serves a version history of the Terms other than the real
// one.
func withTerms(terms legal.Document) envOption {
	return func(s *envSpec) { s.terms = terms }
}

// atInstant fixes the request clock at at; the Scene's MoveClockTo moves
// it from there.
func atInstant(at time.Time) envOption {
	return func(s *envSpec) { s.at = at }
}

// signedInAs replaces the Owner at the screen with a Staff member of
// these options.
func signedInAs(opts ...fixture.StaffOption) envOption {
	return func(s *envSpec) { s.staff = opts }
}

// withStripe hands the server a fake Stripe client the test has already
// shaped.
func withStripe(c *billing.FakeStripeClient) envOption {
	return func(s *envSpec) { s.stripe = c }
}

func newEnv(t *testing.T, opts ...envOption) *env {
	t.Helper()
	spec := envSpec{staff: []fixture.StaffOption{fixture.Owner()}, terms: legal.Terms}
	for _, opt := range opts {
		opt(&spec)
	}
	if spec.stripe == nil {
		spec.stripe = billing.NewFakeStripeClient()
	}
	mount := func(g *staffauth.GatedRouter, ir *idempotency.Router, _ *sql.DB) {
		billing.Mount(g, ir, spec.stripe, spec.terms)
	}
	return &env{Scene: fixture.NewScene(t, mount, fixture.SignedInAs(spec.staff...), fixture.At(spec.at)), Stripe: spec.stripe}
}

// seedLedger seeds one lot of origin at the Practice.
func (e *env) seedLedger(origin string, quantity int) {
	e.T.Helper()
	seedLedgerRow(e.T, e.DB, e.Practice.ID, origin, quantity)
}

// startPurchase posts a purchase body as the Staff member at the screen
// and requires the status want.
func (e *env) startPurchase(body string, want int) {
	e.T.Helper()
	reqtest.ExpectStatus(e.T, http.MethodPost, e.PracticeURL("/billing/purchases"), want, e.SignedIn, reqtest.RawJSON([]byte(body)))
}

// purchase starts a purchase that succeeds and returns the Checkout
// Session the response points at.
func (e *env) purchase(quantity string) billing.PurchaseResponse {
	e.T.Helper()
	return reqtest.Expect[billing.PurchaseResponse](e.T, http.MethodPost, e.PracticeURL("/billing/purchases"), http.StatusOK,
		e.SignedIn, reqtest.RawJSON([]byte(`{"quantity": `+quantity+`}`)))
}

// balance reads the Billing screen as the Staff member at the screen.
func (e *env) balance() billing.BalanceResponse {
	e.T.Helper()
	return reqtest.Expect[billing.BalanceResponse](e.T, http.MethodGet, e.PracticeURL("/billing"), http.StatusOK, e.SignedIn)
}

// balanceAt is balance with a query string, for a read that is refused.
func (e *env) balanceAt(query string, want int) {
	e.T.Helper()
	reqtest.ExpectStatus(e.T, http.MethodGet, e.PracticeURL("/billing")+query, want, e.SignedIn)
}
