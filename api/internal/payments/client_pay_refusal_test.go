package payments_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/testdb"
)

// textNotWaiting is the refusal a paid, void or written-off Invoice reads.
const textNotWaiting = "not waiting for a payment"

// payWorld is a Client at a Practice that can take a card, with the fake
// Stripe the mounted route talks to and a session for her.
type payWorld struct {
	db      *testdb.DB
	me      clientMoney
	stripe  *payments.FakeClient
	srv     *httptest.Server
	session string
}

func newPayWorld(t *testing.T, uid string) payWorld {
	t.Helper()
	db := testdb.New(t)
	me := seedClientMoney(t, db, uid, "Jordan Client", "jordan@example.com")
	seedConnectAccount(t, db, me.practiceID, payAccountID)
	fake := payments.NewFakeClient()
	return payWorld{db, me, fake, newClientMoneyServerWith(t, db, fake), portalSession(t, db, uid)}
}

// get asks for the pay secret of invoiceID under this Client's Engagement.
func (w payWorld) get(t *testing.T, invoiceID string) (int, string) {
	t.Helper()
	return getPortal(t, w.srv, w.session, clientPaymentPath(w.me.engagementID, invoiceID))
}

// TestClientGetPayment_RefusesWhatCannotBePaid covers each refusal, and
// proves none of them reached Stripe.
func TestClientGetPayment_RefusesWhatCannotBePaid(t *testing.T) {
	w := newPayWorld(t, "pay-refusals")
	paid := seedInvoice(t, w.db, w.me.practiceID, w.me.contractID, "in_pr_paid", invoiceStatusPaid, 100, time.Now())
	voided := seedInvoice(t, w.db, w.me.practiceID, w.me.contractID, "in_pr_void", invoiceStatusVoid, 100, time.Now())
	written := seedInvoice(t, w.db, w.me.practiceID, w.me.contractID, "in_pr_unc", invoiceStatusUncollectible, 100, time.Now())
	byHand := seedByHandInvoice(t, w.db, w.me.practiceID, w.me.contractID)

	cases := map[string]struct {
		id     string
		status int
		text   string
	}{
		"paid":                     {paid, http.StatusConflict, textNotWaiting},
		invoiceStatusVoid:          {voided, http.StatusConflict, textNotWaiting},
		invoiceStatusUncollectible: {written, http.StatusConflict, textNotWaiting},
		"by hand":                  {byHand, http.StatusConflict, "Your Practice collects this Invoice directly."},
	}
	for name, c := range cases {
		status, body := w.get(t, c.id)
		if status != c.status || !strings.Contains(body, c.text) {
			t.Errorf("%s: status %d body %s, want %d containing %q", name, status, body, c.status, c.text)
		}
	}
	if len(w.stripe.ClientSecretCalls) != 0 {
		t.Fatalf("a refused Invoice reached Stripe: %+v", w.stripe.ClientSecretCalls)
	}
}

// TestClientGetPayment_NeverReachesAnInvoiceThatIsNotHers is the boundary:
// a draft, another Client's, another Engagement's and a malformed id all
// read as the same 404, and none reaches Stripe.
func TestClientGetPayment_NeverReachesAnInvoiceThatIsNotHers(t *testing.T) {
	w := newPayWorld(t, "pay-notmine")
	other := seedClientMoney(t, w.db, "pay-notmine-other", "Other Client", "other@example.com")
	seedConnectAccount(t, w.db, other.practiceID, "acct_other")
	var second string
	if err := w.db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, kind) VALUES ($1, $2, 'birth') RETURNING id`,
		w.me.clientID, w.me.practiceID).Scan(&second); err != nil {
		t.Fatalf("seed second engagement: %v", err)
	}
	draft := seedInvoice(t, w.db, w.me.practiceID, w.me.contractID, "in_nm_draft", invoiceStatusDraft, 100, time.Now())
	theirs := seedInvoice(t, w.db, other.practiceID, other.contractID, "in_nm_theirs", invoiceStatusOpen, 100, time.Now())
	elsewhere := seedInvoice(t, w.db, w.me.practiceID, seedSignedContract(t, w.db, second), "in_nm_second", invoiceStatusOpen, 100, time.Now())

	for name, id := range map[string]string{invoiceStatusDraft: draft, "another Client's": theirs, "another Engagement's": elsewhere, "malformed": "not-a-uuid"} {
		if status, body := w.get(t, id); status != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404; body %s", name, status, body)
		}
	}
	if len(w.stripe.ClientSecretCalls) != 0 {
		t.Fatalf("an Invoice that is not hers reached Stripe: %+v", w.stripe.ClientSecretCalls)
	}
}

func TestClientGetPayment_RefusesWhileThePracticeCannotTakeACard(t *testing.T) {
	w := newPayWorld(t, "pay-nocard")
	if _, err := w.db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET stripe_connect_card_payments_status = 'restricted' WHERE id = $1`, w.me.practiceID); err != nil {
		t.Fatalf("restrict card payments: %v", err)
	}
	invoiceID := seedInvoice(t, w.db, w.me.practiceID, w.me.contractID, "in_nc", invoiceStatusOpen, 100, time.Now())

	status, body := w.get(t, invoiceID)
	if status != http.StatusConflict || !strings.Contains(body, "cannot take this payment yet") {
		t.Fatalf("status %d body %s, want 409 naming the Practice", status, body)
	}
	if len(w.stripe.ClientSecretCalls) != 0 {
		t.Fatal("a Practice that cannot take a card reached Stripe")
	}
}

// TestClientGetPayment_RefusesWithNoPublishableKey: a deployment that has
// not been given the key hands a browser nothing, and never reaches Stripe.
func TestClientGetPayment_RefusesWithNoPublishableKey(t *testing.T) {
	w := newPayWorld(t, "pay-nokey")
	invoiceID := seedInvoice(t, w.db, w.me.practiceID, w.me.contractID, "in_nk", invoiceStatusOpen, 100, time.Now())
	srv := newClientMoneyServerKeyed(t, w.db, w.stripe, "")

	status, body := getPortal(t, srv, w.session, clientPaymentPath(w.me.engagementID, invoiceID))
	if status != http.StatusConflict || strings.Contains(body, "clientSecret") {
		t.Fatalf("status %d body %s, want a 409 and no secret", status, body)
	}
	if len(w.stripe.ClientSecretCalls) != 0 {
		t.Fatal("a deployment with no publishable key reached Stripe")
	}
}

func TestClientGetPayment_AStripeFailureIsARetryableRefusalNotACredential(t *testing.T) {
	w := newPayWorld(t, "pay-stripe-down")
	invoiceID := seedInvoice(t, w.db, w.me.practiceID, w.me.contractID, "in_sd", invoiceStatusOpen, 100, time.Now())
	w.stripe.ClientSecretErr = errors.New("stripe: boom")

	status, body := w.get(t, invoiceID)
	if status != http.StatusBadGateway || !strings.Contains(body, "Please try again") {
		t.Fatalf("status %d body %s, want 502 and a retry sentence", status, body)
	}
	if strings.Contains(body, "boom") || strings.Contains(body, "clientSecret") {
		t.Fatalf("body leaks the failure or a secret: %s", body)
	}
}

// TestClientGetPayment_RefusesAStaffSession: the gate is her own portal
// access, never a Staff role, Owner included.
func TestClientGetPayment_RefusesAStaffSession(t *testing.T) {
	db := testdb.New(t)
	me := seedClientMoney(t, db, "pay-staff-gate", "Jordan Client", "jordan@example.com")
	seedConnectAccount(t, db, me.practiceID, payAccountID)
	invoiceID := seedInvoice(t, db, me.practiceID, me.contractID, "in_sg2", invoiceStatusOpen, 100, time.Now())
	const staffUID = "owner-not-a-client-pay"
	testdb.SeedStaffAtPractice(t, db, me.practiceID, staffUID, []string{ownerRole}, "employee")

	srv, session := newPracticeInvoiceServer(t, db, staffUID)
	defer srv.Close()
	if status, body := getPortal(t, srv, session, clientPaymentPath(me.engagementID, invoiceID)); status == http.StatusOK {
		t.Fatalf("Staff session read a pay secret: %s", body)
	}
}

// TestClientGetPayment_IsRateLimitedAsAHeavyEndpoint: every call reaches
// Stripe, so one session is capped (docs/api-design.md section 6).
func TestClientGetPayment_IsRateLimitedAsAHeavyEndpoint(t *testing.T) {
	w := newPayWorld(t, "pay-limit")
	invoiceID := seedInvoice(t, w.db, w.me.practiceID, w.me.contractID, "in_rl", invoiceStatusOpen, 100, time.Now())

	var last int
	for range 31 {
		last, _ = w.get(t, invoiceID)
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("31st call status = %d, want 429", last)
	}
	if got := len(w.stripe.ClientSecretCalls); got != 30 {
		t.Fatalf("stripe was called %d times, want 30: the refused call must not reach it", got)
	}
}
