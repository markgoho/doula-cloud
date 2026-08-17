package billing

import (
	"context"
	"fmt"
	"sync"
)

// FakeCheckoutSession records one CreateCheckoutSession call, for tests to
// assert against.
type FakeCheckoutSession struct {
	CustomerID string
	PracticeID string
	Quantity   int
}

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
	Sessions      []FakeCheckoutSession

	CreateCustomerErr        error
	CreateCheckoutSessionErr error
}

// NewFakeStripeClient returns a FakeStripeClient with no recorded calls.
func NewFakeStripeClient() *FakeStripeClient {
	return &FakeStripeClient{}
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
func (f *FakeStripeClient) CreateCheckoutSession(_ context.Context, customerID, practiceID string, quantity int) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.CreateCheckoutSessionErr != nil {
		return "", f.CreateCheckoutSessionErr
	}
	f.Sessions = append(f.Sessions, FakeCheckoutSession{CustomerID: customerID, PracticeID: practiceID, Quantity: quantity})
	f.nextID++
	return fmt.Sprintf("https://checkout.stripe.test/session/%d", f.nextID), nil
}

// Calls returns a copy of every CreateCheckoutSession call recorded so
// far.
func (f *FakeStripeClient) Calls() []FakeCheckoutSession {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]FakeCheckoutSession(nil), f.Sessions...)
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
