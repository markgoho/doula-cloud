package billing_test

import (
	"errors"
	"net/http"
	"testing"

	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

// errStripeFake is returned by FakeStripeClient methods in tests that
// exercise a handler's Stripe-failure path.
var errStripeFake = errors.New("stripe: fake failure")

func stripeCustomerID(t *testing.T, db *testdb.DB, practiceID string) *string {
	t.Helper()
	id := testdb.Scalar[*string](t, db.Admin, `SELECT stripe_customer_id FROM practices WHERE id = $1`, practiceID)
	return id
}

// TestPostPurchaseHandler_OwnerCreatesCustomerAndCheckoutSession proves an
// Owner's first purchase lazily creates a Stripe Customer, persists its id
// on the Practice, and returns a Checkout Session URL tagged with the
// right metadata.
func TestPostPurchaseHandler_OwnerCreatesCustomerAndCheckoutSession(t *testing.T) {
	e := newEnv(t)

	out := e.purchase("5")

	if out.CheckoutURL == "" {
		t.Fatal("checkoutUrl is empty")
	}
	calls := e.Stripe.Calls()
	if len(calls) != 1 {
		t.Fatalf("CreateCheckoutSession calls = %d, want 1", len(calls))
	}
	if calls[0].PracticeID != e.Practice.ID || calls[0].Quantity != 5 {
		t.Fatalf("checkout session call = %+v, want practiceID %q, quantity 5", calls[0], e.Practice.ID)
	}
	// The sole Staff member works in New York, so the sale is wholly
	// taxable and the Checkout page keeps its ordinary single line item.
	if calls[0].NewYorkStaff != 1 || calls[0].TotalStaff != 1 {
		t.Fatalf("apportionment = %d of %d, want 1 of 1", calls[0].NewYorkStaff, calls[0].TotalStaff)
	}
	id := stripeCustomerID(t, e.DB, e.Practice.ID)
	if id == nil || *id == "" {
		t.Fatal("stripe_customer_id was not persisted")
	}
	if calls[0].CustomerID != *id {
		t.Fatalf("checkout session customer id = %q, want %q", calls[0].CustomerID, *id)
	}
}

// TestPostPurchaseHandler_AdminCanPurchase proves ADR-0017's correction:
// an Admin, not only an Owner, may buy Credits.
func TestPostPurchaseHandler_AdminCanPurchase(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Admin()))

	if out := e.purchase("3"); out.CheckoutURL == "" {
		t.Fatal("checkoutUrl is empty")
	}
}

// TestPostPurchaseHandler_SecondPurchaseReusesExistingCustomer proves a
// Practice's second purchase does not create a second Stripe Customer.
func TestPostPurchaseHandler_SecondPurchaseReusesExistingCustomer(t *testing.T) {
	e := newEnv(t)

	e.purchase("3")
	firstCustomerID := stripeCustomerID(t, e.DB, e.Practice.ID)
	e.purchase("10")

	if got := e.Stripe.CustomerCallCount(); got != 1 {
		t.Fatalf("CreateCustomer call count = %d, want 1", got)
	}
	if got := stripeCustomerID(t, e.DB, e.Practice.ID); got == nil || *got != *firstCustomerID {
		t.Fatalf("stripe_customer_id changed across purchases: first %v, second %v", firstCustomerID, got)
	}
	if got := len(e.Stripe.Calls()); got != 2 {
		t.Fatalf("CreateCheckoutSession calls = %d, want 2", got)
	}
}

// TestPostPurchaseHandler_Refusals covers what the handler turns away. Each
// refusal before Stripe leaves it untouched; a Stripe failure surfaces as
// an internal error, and a failed Customer creation never persists an id.
func TestPostPurchaseHandler_Refusals(t *testing.T) {
	cases := []struct {
		name string
		// staff is who is at the screen; nil leaves the Owner.
		staff []fixture.StaffOption
		body  string
		// stripeFails shapes the fake client.
		stripeFails func(*billing.FakeStripeClient)
		want        int
		// reachesCheckout is set for the one refusal that happens after
		// the Customer exists.
		reachesCheckout bool
	}{
		{name: "a non-Owner", staff: []fixture.StaffOption{fixture.Doula()}, body: `{"quantity": 5}`, want: http.StatusForbidden},
		{name: "a zero quantity", body: `{"quantity": 0}`, want: http.StatusBadRequest},
		{name: "a body that is not JSON", body: `not json`, want: http.StatusBadRequest},
		{
			name: "a Customer Stripe will not create", body: `{"quantity": 5}`, want: http.StatusInternalServerError,
			stripeFails: func(c *billing.FakeStripeClient) { c.CreateCustomerErr = errStripeFake },
		},
		{
			name: "a Checkout Session Stripe will not create", body: `{"quantity": 5}`, want: http.StatusInternalServerError,
			stripeFails:     func(c *billing.FakeStripeClient) { c.CreateCheckoutSessionErr = errStripeFake },
			reachesCheckout: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stripe := billing.NewFakeStripeClient()
			if c.stripeFails != nil {
				c.stripeFails(stripe)
			}
			opts := []envOption{withStripe(stripe)}
			if c.staff != nil {
				opts = append(opts, signedInAs(c.staff...))
			}
			e := newEnv(t, opts...)

			e.startPurchase(c.body, c.want)

			if got := len(stripe.Calls()); got != 0 {
				t.Errorf("CreateCheckoutSession calls = %d, want 0", got)
			}
			if got := stripeCustomerID(t, e.DB, e.Practice.ID); got != nil && !c.reachesCheckout {
				t.Errorf("stripe_customer_id = %v, want nil (never persisted)", *got)
			}
		})
	}
}

// attachMembership inserts a practice_memberships row directly for a
// Staff row already seeded with its own custom attributes (a specific
// work_state or email testdb.SeedStaffAtPractice can't parameterize).
// Stays local: this package's own apportion-by-work-state tests are the
// only ones that need a Staff row with an explicit non-NY work_state.
func attachMembership(t *testing.T, db *testdb.DB, practiceID, staffID, roles string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type) VALUES ($1, $2, $3::practice_role[], 'employee')`,
		practiceID, staffID, roles,
	); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
}

// seedStaffInState seeds a Staff member who works in workState and gives
// her a Membership at practiceID.
func seedStaffInState(t *testing.T, db *testdb.DB, practiceID, identityUID, workState string) {
	t.Helper()
	staffID := testdb.Scalar[string](t, db.Admin, `INSERT INTO staff (identity_uid, first_name, last_name, email, work_state) VALUES ($1, $1, '', $1 || '@example.com', $2) RETURNING id`,
		identityUID, workState)
	attachMembership(t, db, practiceID, staffID, "{doula}")
}

// TestPostPurchaseHandler_ApportionsByWhereStaffWork proves the headcount
// New York's sales tax is computed on (#389) is counted from the
// Practice's own roster, and that pair -- not the quantity, and not the
// caseload -- is what reaches Stripe. A Practice with nobody in New York
// reaches Stripe with a zero numerator, which is what makes her purchase
// carry no New York tax.
func TestPostPurchaseHandler_ApportionsByWhereStaffWork(t *testing.T) {
	cases := []struct {
		name string
		// ownerState is where the Owner at the screen works.
		ownerState string
		// colleagues are the work states of the Practice's other Staff.
		colleagues             []string
		wantNewYork, wantTotal int
	}{
		{name: "two of four work in New York", ownerState: "NY", colleagues: []string{"NY", "NJ", "CA"}, wantNewYork: 2, wantTotal: 4},
		{name: "wholly out of state", ownerState: "NJ", colleagues: []string{"PA"}, wantNewYork: 0, wantTotal: 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			e.Exec(`UPDATE staff SET work_state = $1 WHERE id = $2`, c.ownerState, e.Staff.ID)
			for i, state := range c.colleagues {
				seedStaffInState(t, e.DB, e.Practice.ID, "apportion-"+string(rune('a'+i)), state)
			}
			// A second Practice's out-of-state doula must not dilute the ratio.
			seedStaffInState(t, e.DB, testdb.SeedPractice(t, e.DB, "Other Practice"), "apportion-other", "TX")

			e.purchase("20")

			calls := e.Stripe.Calls()
			if len(calls) != 1 {
				t.Fatalf("CreateCheckoutSession calls = %d, want 1", len(calls))
			}
			if calls[0].NewYorkStaff != c.wantNewYork || calls[0].TotalStaff != c.wantTotal {
				t.Fatalf("apportionment = %d of %d, want %d of %d", calls[0].NewYorkStaff, calls[0].TotalStaff, c.wantNewYork, c.wantTotal)
			}
		})
	}
}
