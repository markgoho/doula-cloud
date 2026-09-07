package billing

import (
	"context"
	"fmt"
	"sync"
)

// FakeStripeClient is an in-memory StripeClient double, injected into
// handler tests instead of a real Stripe account -- mirrors
// push.FakePusher. CreateCustomerErr and CreateCheckoutSessionErr, when
// set, are returned by the corresponding method (after still recording a
// CreateCustomer call, where applicable) so tests can exercise a handler's
// Stripe-failure handling.
type FakeStripeClient struct {
	mu            sync.Mutex
	nextID        int
	CustomerCalls []string
	Sessions      []CheckoutSessionRequest

	Refunds []RefundCall

	CreateCustomerErr        error
	CreateCheckoutSessionErr error
	RefundPaymentErr         error

	// CreditPriceUnitAmountCents and CreditPriceCurrency are what
	// CreditPrice returns by default -- the live $20.00 USD Price (#448) --
	// so a test that doesn't care about the exact figure still gets a
	// realistic one. CreditPriceErr, when set, makes CreditPrice fail
	// instead, the way it does with no Stripe credentials or an
	// unreachable Stripe.
	CreditPriceUnitAmountCents int64
	CreditPriceCurrency        string
	CreditPriceErr             error

	// ReplayedRefundID, when set, is returned by every RefundPayment
	// call -- what Stripe does when a retry carries an idempotency key
	// it has already seen.
	ReplayedRefundID string
}

// RefundCall is one recorded RefundPayment call: which payment was
// reversed, under which idempotency key, and by how much.
type RefundCall struct {
	PaymentIntentID string
	IdempotencyKey  string
	AmountCents     int64
}

// fakeCreditPriceUnitAmountCents is NewFakeStripeClient's default
// CreditPrice -- the live $20.00 USD credit Price (#448), mirrored in
// balance_test.go's own seedUnitPriceCents for the same reason.
const fakeCreditPriceUnitAmountCents = 2000

// NewFakeStripeClient returns a FakeStripeClient with no recorded calls,
// its CreditPrice defaulted to the live $20.00 USD credit Price (#448).
func NewFakeStripeClient() *FakeStripeClient {
	return &FakeStripeClient{CreditPriceUnitAmountCents: fakeCreditPriceUnitAmountCents, CreditPriceCurrency: "usd"}
}

// CreditPrice returns a CreditPrice built from CreditPriceUnitAmountCents
// and CreditPriceCurrency, or CreditPriceErr if a test set one.
func (f *FakeStripeClient) CreditPrice(_ context.Context) (CreditPrice, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.CreditPriceErr != nil {
		return CreditPrice{}, f.CreditPriceErr
	}
	return CreditPrice{UnitAmountCents: f.CreditPriceUnitAmountCents, Currency: f.CreditPriceCurrency}, nil
}

// CreateCustomer returns a deterministic fake Stripe Customer id, or
// CreateCustomerErr if a test set one.
func (f *FakeStripeClient) CreateCustomer(_ context.Context, practiceID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.CustomerCalls = append(f.CustomerCalls, practiceID)
	if f.CreateCustomerErr != nil {
		return "", f.CreateCustomerErr
	}
	f.nextID++
	return fmt.Sprintf("cus_fake_%d", f.nextID), nil
}

// CreateCheckoutSession records the call and returns a deterministic fake
// checkout URL, or CreateCheckoutSessionErr if a test set one.
func (f *FakeStripeClient) CreateCheckoutSession(_ context.Context, req CheckoutSessionRequest) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.CreateCheckoutSessionErr != nil {
		return "", f.CreateCheckoutSessionErr
	}
	f.Sessions = append(f.Sessions, req)
	f.nextID++
	return fmt.Sprintf("https://checkout.stripe.test/session/%d", f.nextID), nil
}

// Calls returns a copy of every CreateCheckoutSession call recorded so
// far.
func (f *FakeStripeClient) Calls() []CheckoutSessionRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]CheckoutSessionRequest(nil), f.Sessions...)
}

// CustomerCallCount returns how many times CreateCustomer has been called
// so far -- tests use this to prove a second purchase reuses the
// Practice's already-stored stripe_customer_id instead of creating a
// second Stripe Customer.
func (f *FakeStripeClient) CustomerCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.CustomerCalls)
}

// RefundPayment records the call and returns a deterministic fake Stripe
// Refund id, or RefundPaymentErr if a test set one.
func (f *FakeStripeClient) RefundPayment(_ context.Context, paymentIntentID, idempotencyKey string, amountCents int64) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.RefundPaymentErr != nil {
		return "", f.RefundPaymentErr
	}
	f.Refunds = append(f.Refunds, RefundCall{
		PaymentIntentID: paymentIntentID,
		IdempotencyKey:  idempotencyKey,
		AmountCents:     amountCents,
	})
	if f.ReplayedRefundID != "" {
		return f.ReplayedRefundID, nil
	}
	f.nextID++
	return fmt.Sprintf("re_fake_%d", f.nextID), nil
}

// RefundCalls returns a copy of every RefundPayment call recorded so far.
func (f *FakeStripeClient) RefundCalls() []RefundCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]RefundCall(nil), f.Refunds...)
}
