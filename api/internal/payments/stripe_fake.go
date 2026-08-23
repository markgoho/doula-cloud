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

// FakeCreateInvoiceCall records one CreateInvoice call -- every argument
// the handler passed the port, so a test can assert the connected account
// id, the Client's name/email (and nothing else, i.e. no clinical field),
// and that Description was InvoiceLineItemDescription and nothing
// request-supplied.
type FakeCreateInvoiceCall struct {
	AccountID     string
	CustomerEmail string
	CustomerName  string
	Description   string
	AmountCents   int64
}

// FakeClient is an in-memory Client double, injected into handler tests
// instead of a real Stripe account -- mirrors billing.FakeStripeClient.
// The *Err fields, when set, are returned by the corresponding method
// (after still recording the call, where applicable) so tests can
// exercise a handler's Stripe-failure handling.
type FakeClient struct {
	mu     sync.Mutex
	nextID int

	AccountCalls []string
	// AccountNames records the display name passed alongside each
	// CreateAccount call, so a test can prove the Practice's own name is
	// what reaches Stripe (#247).
	AccountNames       []string
	AccountLinkCalls   []FakeAccountLinkCall
	RetrieveCalls      []string
	CreateInvoiceCalls []FakeCreateInvoiceCall
	FinalizeInvoiceIDs []string

	// Statuses, keyed by account id, is what RetrieveAccount returns --
	// tests set this to control the "not connected / onboarding
	// incomplete / active" status GetConnectStatusHandler reports.
	Statuses map[string]AccountStatus

	// PaymentReferences, keyed by Stripe invoice id, is what
	// RetrieveInvoicePaymentReference returns.
	PaymentReferences map[string]string

	CreateAccountErr     error
	CreateAccountLinkErr error
	RetrieveAccountErr   error
	CreateInvoiceErr     error
	FinalizeInvoiceErr   error
	PaymentReferenceErr  error
}

// NewFakeClient returns a FakeClient with no recorded calls.
func NewFakeClient() *FakeClient {
	return &FakeClient{Statuses: map[string]AccountStatus{}, PaymentReferences: map[string]string{}}
}

// CreateAccount returns a deterministic fake Stripe Connect account id, or
// CreateAccountErr if a test set one.
func (f *FakeClient) CreateAccount(_ context.Context, practiceID, practiceName string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.AccountCalls = append(f.AccountCalls, practiceID)
	f.AccountNames = append(f.AccountNames, practiceName)
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

// CreateInvoice records the call -- accountID, customerEmail,
// customerName, description, amountCents, exactly as PostInvoiceHandler
// passed them -- and returns a deterministic fake invoice id, or
// CreateInvoiceErr if a test set one.
func (f *FakeClient) CreateInvoice(_ context.Context, accountID, customerEmail, customerName, description string, amountCents int64) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.CreateInvoiceErr != nil {
		return "", f.CreateInvoiceErr
	}
	f.CreateInvoiceCalls = append(f.CreateInvoiceCalls, FakeCreateInvoiceCall{
		AccountID:     accountID,
		CustomerEmail: customerEmail,
		CustomerName:  customerName,
		Description:   description,
		AmountCents:   amountCents,
	})
	f.nextID++
	return fmt.Sprintf("in_fake_%d", f.nextID), nil
}

// FinalizeInvoice records the call and returns a deterministic fake
// hosted invoice URL, or FinalizeInvoiceErr if a test set one.
func (f *FakeClient) FinalizeInvoice(_ context.Context, _, invoiceID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.FinalizeInvoiceErr != nil {
		return "", f.FinalizeInvoiceErr
	}
	f.FinalizeInvoiceIDs = append(f.FinalizeInvoiceIDs, invoiceID)
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

// ParseAccountEvent is likewise not faked for signature verification --
// PostAccountWebhookHandler's tests inject the real StripeAPIClient so a
// genuinely signed thin event is what reaches the handler. It returns a
// zero-value AccountEvent and no error.
func (f *FakeClient) ParseAccountEvent(_ []byte, _, _ string) (AccountEvent, error) {
	return AccountEvent{}, nil
}

// RetrieveInvoicePaymentReference returns the reference a test set in
// PaymentReferences for the invoice id (empty if unset), or
// PaymentReferenceErr if a test set one.
// coverage:ignore reason: PostConnectWebhookHandler's tests inject referenceClient (real signature verification over a stubbed lookup) rather than FakeClient, so this double's implementation is never called
func (f *FakeClient) RetrieveInvoicePaymentReference(_ context.Context, _, invoiceID string) (string, error) {
	// coverage:ignore reason: PostConnectWebhookHandler's tests inject referenceClient rather than FakeClient, so this double's implementation is never called
	f.mu.Lock()
	// coverage:ignore reason: PostConnectWebhookHandler's tests inject referenceClient rather than FakeClient, so this double's implementation is never called
	defer f.mu.Unlock()
	// coverage:ignore reason: PostConnectWebhookHandler's tests inject referenceClient rather than FakeClient, so this double's implementation is never called
	if f.PaymentReferenceErr != nil {
		return "", f.PaymentReferenceErr
	}
	// coverage:ignore reason: PostConnectWebhookHandler's tests inject referenceClient rather than FakeClient, so this double's implementation is never called
	return f.PaymentReferences[invoiceID], nil
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
