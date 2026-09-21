package payments_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/testdb"
)

const (
	stripeEventTypeCreditNoteCreated = "credit_note.created"
	stripeEventTypeRefundCreated     = "refund.created"
	// notANumber is a string where Stripe sends an integer -- the shape
	// every malformed-object webhook test uses to make decoding fail.
	notANumber = "not-a-number"
)

// refundWebhookFixture is a connected Practice with a Stripe-backed
// Invoice paid by card -- a 'stripe' Payment whose reference is a
// PaymentIntent id, the row invoice.paid writes.
type refundWebhookFixture struct {
	db           *testdb.DB
	accountID    string
	engagementID string
	invoiceID    string
	stripeInv    string
	paymentID    string
	pi           string
}

func newRefundWebhookFixture(t *testing.T, name string) refundWebhookFixture {
	t.Helper()
	db := testdb.New(t)
	f := refundWebhookFixture{db: db, accountID: "acct_" + name, stripeInv: "in_" + name, pi: "pi_" + name}
	practiceID := seedConnectedPractice(t, db, name, f.accountID)
	_, f.engagementID = testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, f.engagementID)
	f.invoiceID = seedInvoice(t, db, practiceID, contractID, f.stripeInv, invoiceStatusOpen, 50000, time.Now())
	f.paymentID = seedStripePayment(t, db, f.invoiceID, 50000, f.pi)
	return f
}

func creditNotePayload(t *testing.T, eventID, accountID, creditNoteID, stripeInvoiceID string, outOfBand int64, refunds map[string]int64) []byte {
	t.Helper()
	refundList := []map[string]any{}
	for id, amount := range refunds {
		refundList = append(refundList, map[string]any{kindRefund: id, "amount_refunded": amount, typeKey: kindRefund})
	}
	return buildConnectEventPayload(t, eventID, stripeEventTypeCreditNoteCreated, accountID, map[string]any{
		"id":                 creditNoteID,
		objectKey:            "credit_note",
		"invoice":            stripeInvoiceID,
		"out_of_band_amount": outOfBand,
		"refunds":            refundList,
	})
}

func refundPayload(t *testing.T, eventID, accountID, refundID, paymentIntent, status string, amount int64) []byte {
	t.Helper()
	return buildConnectEventPayload(t, eventID, stripeEventTypeRefundCreated, accountID, map[string]any{
		"id":             refundID,
		objectKey:        kindRefund,
		"amount":         amount,
		"payment_intent": paymentIntent,
		"status":         status,
	})
}

func deliver(t *testing.T, f refundWebhookFixture, payload []byte) {
	t.Helper()
	srv := newConnectWebhookServer(f.db)
	defer srv.Close()
	resp := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("webhook status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// refundRowsFor reads every Refund row against invoiceID, oldest first.
func refundRowsFor(t *testing.T, db *testdb.DB, invoiceID string) []refundRow {
	t.Helper()
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT kind::text, amount_cents, target_payment_id::text, method::text, note, stripe_payment_reference
		   FROM payments WHERE invoice_id = $1 AND kind = 'refund' ORDER BY created_at`, invoiceID)
	if err != nil {
		t.Fatalf("query refunds: %v", err)
	}
	defer rows.Close()
	var out []refundRow
	for rows.Next() {
		var r refundRow
		if err := rows.Scan(&r.kind, &r.amountCents, &r.targetPaymentID, &r.method, &r.note, &r.stripeReference); err != nil {
			t.Fatalf("scan refund: %v", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate refunds: %v", err)
	}
	return out
}

// TestConnectWebhook_DashboardCreditNoteIsRecordedAsRefund proves a
// credit note a Practice issued from her own Stripe Dashboard reaches
// Doula Cloud's record: a Refund row against the card Payment, keyed on
// the Refund object's id, recorded by Doula Cloud with its origin named
// -- and the Invoice left at 'paid'.
func TestConnectWebhook_DashboardCreditNoteIsRecordedAsRefund(t *testing.T) {
	f := newRefundWebhookFixture(t, "cn_dashboard")

	deliver(t, f, creditNotePayload(t, "evt_cn_dash", f.accountID, "cn_dash", f.stripeInv, 0, map[string]int64{"re_dash": 20000}))

	got := refundRowsFor(t, f.db, f.invoiceID)
	if len(got) != 1 {
		t.Fatalf("refund rows = %d, want 1", len(got))
	}
	r := got[0]
	if r.amountCents != -20000 || r.targetPaymentID == nil || *r.targetPaymentID != f.paymentID {
		t.Fatalf("refund row = %+v, want -20000 against %s", r, f.paymentID)
	}
	if r.stripeReference == nil || *r.stripeReference != "re_dash" {
		t.Fatalf("refund reference = %v, want re_dash", r.stripeReference)
	}
	if r.method != nil || r.note != nil {
		t.Fatalf("refund row = %+v, want no method and no note from a webhook", r)
	}
	if status := invoiceStatusFor(t, f.db, f.invoiceID); status != invoiceStatusPaid {
		t.Fatalf("invoice status = %q, want %q", status, invoiceStatusPaid)
	}
	diffs := refundActivityDiffs(t, f.db, f.engagementID)
	if len(diffs) != 1 || diffs[0]["_actorKind"] != "system" || diffs[0]["origin"] != "stripe_dashboard" {
		t.Fatalf("activity = %v, want one system entry with origin stripe_dashboard", diffs)
	}
}

// TestConnectWebhook_CardRefundArrivingTwiceIsRecordedOnce proves a card
// Refund, which Stripe reports as both credit_note.created and
// refund.created, becomes one Refund row whichever arrives first.
func TestConnectWebhook_CardRefundArrivingTwiceIsRecordedOnce(t *testing.T) {
	for _, order := range []string{"credit note first", "refund first"} {
		t.Run(order, func(t *testing.T) {
			f := newRefundWebhookFixture(t, "twice")
			cn := creditNotePayload(t, "evt_twice_cn", f.accountID, "cn_twice", f.stripeInv, 0, map[string]int64{"re_twice": 10000})
			re := refundPayload(t, "evt_twice_re", f.accountID, "re_twice", f.pi, "succeeded", 10000)
			if order == "refund first" {
				cn, re = re, cn
			}
			deliver(t, f, cn)
			deliver(t, f, re)
			if got := len(refundRowsFor(t, f.db, f.invoiceID)); got != 1 {
				t.Fatalf("refund rows = %d, want 1", got)
			}
		})
	}
}

// TestConnectWebhook_EchoOfIssuedRefundIsNotRecordedTwice proves the
// credit note PostRefundPaymentHandler itself issued is recognized when
// it comes back, by the reference that handler stored.
func TestConnectWebhook_EchoOfIssuedRefundIsNotRecordedTwice(t *testing.T) {
	f := newRefundWebhookFixture(t, "echo")
	if _, err := f.db.Admin.ExecContext(t.Context(),
		`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, target_payment_id, stripe_payment_reference)
		 VALUES ($1, -15000, now(), 'refund', $2, 're_echo')`, f.invoiceID, f.paymentID,
	); err != nil {
		t.Fatalf("seed issued refund: %v", err)
	}

	deliver(t, f, creditNotePayload(t, "evt_echo", f.accountID, "cn_echo", f.stripeInv, 0, map[string]int64{"re_echo": 15000}))

	if got := len(refundRowsFor(t, f.db, f.invoiceID)); got != 1 {
		t.Fatalf("refund rows = %d, want only the one the handler wrote", got)
	}
	if got := len(refundActivityDiffs(t, f.db, f.engagementID)); got != 0 {
		t.Fatalf("webhook activity rows = %d, want 0 for an echo", got)
	}
}

// TestConnectWebhook_BareDashboardRefundIsRecordedThroughPaymentIntent
// proves a refund made from the Dashboard's Payments page -- which
// creates no credit note, and which Stripe's own Invoice never learns of
// -- still reaches the record, through the PaymentIntent.
func TestConnectWebhook_BareDashboardRefundIsRecordedThroughPaymentIntent(t *testing.T) {
	f := newRefundWebhookFixture(t, "bare")

	deliver(t, f, refundPayload(t, "evt_bare", f.accountID, "re_bare", f.pi, "succeeded", 10000))

	got := refundRowsFor(t, f.db, f.invoiceID)
	if len(got) != 1 || got[0].amountCents != -10000 || *got[0].stripeReference != "re_bare" {
		t.Fatalf("refund rows = %+v, want one -10000 row keyed re_bare", got)
	}
}

// TestConnectWebhook_OutOfBandCreditNoteIsKeyedOnItsOwnID proves an
// out-of-band credit note, which creates no Refund object, is keyed on
// the credit note's id.
func TestConnectWebhook_OutOfBandCreditNoteIsKeyedOnItsOwnID(t *testing.T) {
	f := newRefundWebhookFixture(t, "oob")

	deliver(t, f, creditNotePayload(t, "evt_oob", f.accountID, "cn_oob", f.stripeInv, 50000, nil))

	got := refundRowsFor(t, f.db, f.invoiceID)
	if len(got) != 1 || got[0].amountCents != -50000 || *got[0].stripeReference != "cn_oob" {
		t.Fatalf("refund rows = %+v, want one -50000 row keyed cn_oob", got)
	}
}

// TestConnectWebhook_RefundEventsThatRecordNothing covers every case the
// handlers acknowledge without writing a Refund row: a replay, a credit
// note that is only a balance credit, a Refund that failed, one for a
// Payment Doula Cloud never recorded, an Invoice it never raised, and one
// that would over-return the Payment.
func TestConnectWebhook_RefundEventsThatRecordNothing(t *testing.T) {
	cases := []struct {
		name    string
		payload func(t *testing.T, f refundWebhookFixture) [][]byte
		want    int
	}{
		{"replayed refund.created event id", func(t *testing.T, f refundWebhookFixture) [][]byte {
			p := refundPayload(t, "evt_replay", f.accountID, "re_replay", f.pi, "succeeded", 100)
			return [][]byte{p, p}
		}, 1},
		{"replayed credit_note.created event id", func(t *testing.T, f refundWebhookFixture) [][]byte {
			p := creditNotePayload(t, "evt_replay_cn", f.accountID, "cn_replay", f.stripeInv, 100, nil)
			return [][]byte{p, p}
		}, 1},
		{"balance-only credit note", func(t *testing.T, f refundWebhookFixture) [][]byte {
			return [][]byte{creditNotePayload(t, "evt_credit_only", f.accountID, "cn_credit_only", f.stripeInv, 0, nil)}
		}, 0},
		{"failed refund", func(t *testing.T, f refundWebhookFixture) [][]byte {
			return [][]byte{refundPayload(t, "evt_failed", f.accountID, "re_failed", f.pi, "failed", 100)}
		}, 0},
		{"unknown payment intent", func(t *testing.T, f refundWebhookFixture) [][]byte {
			return [][]byte{refundPayload(t, "evt_unknown_pi", f.accountID, "re_unknown", "pi_never_seen", "succeeded", 100)}
		}, 0},
		{"unknown invoice", func(t *testing.T, f refundWebhookFixture) [][]byte {
			return [][]byte{creditNotePayload(t, "evt_unknown_inv", f.accountID, "cn_unknown", "in_never_raised", 0, map[string]int64{"re_unknown_inv": 100})}
		}, 0},
		{"over-return", func(t *testing.T, f refundWebhookFixture) [][]byte {
			return [][]byte{refundPayload(t, "evt_over", f.accountID, "re_over", f.pi, "succeeded", 50001)}
		}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newRefundWebhookFixture(t, "nothing")
			for _, p := range tc.payload(t, f) {
				deliver(t, f, p)
			}
			if got := len(refundRowsFor(t, f.db, f.invoiceID)); got != tc.want {
				t.Fatalf("refund rows = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestConnectWebhook_MalformedRefundObjectsAreRejected proves a signed
// event whose object does not decode is answered 500, so Stripe retries
// it rather than having it acknowledged and lost -- the same rule
// invoice.paid's malformed-object test holds.
func TestConnectWebhook_MalformedRefundObjectsAreRejected(t *testing.T) {
	cases := map[string]func(f refundWebhookFixture) []byte{
		"credit_note.created": func(f refundWebhookFixture) []byte {
			return buildConnectEventPayload(t, "evt_cn_bad", stripeEventTypeCreditNoteCreated, f.accountID,
				map[string]any{"id": "cn_bad", "out_of_band_amount": notANumber})
		},
		"refund.created": func(f refundWebhookFixture) []byte {
			return buildConnectEventPayload(t, "evt_re_bad", stripeEventTypeRefundCreated, f.accountID,
				map[string]any{"id": "re_bad", "amount": notANumber})
		},
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			f := newRefundWebhookFixture(t, "malformed")
			srv := newConnectWebhookServer(f.db)
			defer srv.Close()

			resp := postConnectWebhook(t, srv, payload(f), stripeConnectWebhookSecret)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusInternalServerError {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
			}
			if got := len(refundRowsFor(t, f.db, f.invoiceID)); got != 0 {
				t.Fatalf("refund rows = %d, want 0", got)
			}
		})
	}
}
