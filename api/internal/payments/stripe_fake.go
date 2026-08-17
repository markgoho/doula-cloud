package payments

import (
	"context"
	"fmt"
	"sync"
)

// FakeAccountLinkCall records one CreateAccountLink call, for tests to
// assert against.
type FakeAccountLinkCall struct {
	AccountID  string
	PracticeID string
}

// FakeClient is an in-memory Client double, injected into handler tests
// instead of a real Stripe account -- mirrors billing.FakeStripeClient.
// The *Err fields, when set, are returned by the corresponding method
// (after still recording the call, where applicable) so tests can
// exercise a handler's Stripe-failure handling.
type FakeClient struct {
	mu     sync.Mutex
	nextID int

	AccountCalls     []string
	AccountLinkCalls []FakeAccountLinkCall
	RetrieveCalls    []string

	// Statuses, keyed by account id, is what RetrieveAccount returns --
	// tests set this to control the "not connected / onboarding
	// incomplete / active" status GetConnectStatusHandler reports.
	Statuses map[string]AccountStatus

	CreateAccountErr     error
	CreateAccountLinkErr error
	RetrieveAccountErr   error
}

// NewFakeClient returns a FakeClient with no recorded calls.
func NewFakeClient() *FakeClient {
	return &FakeClient{Statuses: map[string]AccountStatus{}}
}

// CreateAccount returns a deterministic fake Stripe Connect account id, or
// CreateAccountErr if a test set one.
func (f *FakeClient) CreateAccount(_ context.Context, practiceID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.AccountCalls = append(f.AccountCalls, practiceID)
	if f.CreateAccountErr != nil {
		return "", f.CreateAccountErr
	}
	f.nextID++
	return fmt.Sprintf("acct_fake_%d", f.nextID), nil
}

// CreateAccountLink records the call and returns a deterministic fake
// onboarding URL, or CreateAccountLinkErr if a test set one.
func (f *FakeClient) CreateAccountLink(_ context.Context, accountID, practiceID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.CreateAccountLinkErr != nil {
		return "", f.CreateAccountLinkErr
	}
	f.AccountLinkCalls = append(f.AccountLinkCalls, FakeAccountLinkCall{AccountID: accountID, PracticeID: practiceID})
	f.nextID++
	return fmt.Sprintf("https://connect.stripe.test/setup/%d", f.nextID), nil
}

// RetrieveAccount returns the AccountStatus a test set for accountID in
// Statuses (the zero value if unset), or RetrieveAccountErr if a test set
// one.
func (f *FakeClient) RetrieveAccount(_ context.Context, accountID string) (AccountStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RetrieveCalls = append(f.RetrieveCalls, accountID)
	if f.RetrieveAccountErr != nil {
		return AccountStatus{}, f.RetrieveAccountErr
	}
	return f.Statuses[accountID], nil
}

// CreateInvoice is not exercised by any handler yet (#81/#82 build it) --
// it always returns a deterministic fake invoice id.
func (f *FakeClient) CreateInvoice(_ context.Context, _, _, _ string, _ int64) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	return fmt.Sprintf("in_fake_%d", f.nextID), nil
}

// FinalizeInvoice is not exercised by any handler yet (#81/#82 build it) --
// it always returns a deterministic fake hosted invoice URL.
func (f *FakeClient) FinalizeInvoice(_ context.Context, _, invoiceID string) (string, error) {
	return "https://invoice.stripe.test/" + invoiceID, nil
}

// VerifyWebhookSignature is not exercised through FakeClient by any
// handler -- #80's PostConnectWebhookHandler test suite injects the real
// StripeAPIClient instead, per its ticket body ("stripe-go's real
// signature-construction helper... needs no faking"), so this always
// returns a zero-value WebhookEvent and no error.
func (f *FakeClient) VerifyWebhookSignature(_ []byte, _, _ string) (WebhookEvent, error) {
	return WebhookEvent{}, nil
}

// AccountCallCount returns how many times CreateAccount has been called so
// far -- tests use this to prove a second connection attempt reuses the
// Practice's already-stored stripe_connect_account_id instead of creating
// a second Stripe account.
func (f *FakeClient) AccountCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.AccountCalls)
}

// AccountLinkCallCount returns how many times CreateAccountLink has been
// called so far.
func (f *FakeClient) AccountLinkCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.AccountLinkCalls)
}
