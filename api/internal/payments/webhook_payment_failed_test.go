package payments_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/testdb"
)

const (
	stripeEventTypeInvoiceMarkedUncollectible = "invoice.marked_uncollectible"
	actionInvoicePaymentFailed                = "invoice_payment_failed"
	actionInvoiceWrittenOff                   = "invoice_written_off"
)

// invoiceMarkedUncollectiblePayload builds a raw invoice.marked_uncollectible
// event body referencing stripeInvoiceID.
func invoiceMarkedUncollectiblePayload(t *testing.T, eventID, accountID, stripeInvoiceID string) []byte {
	t.Helper()
	return buildConnectEventPayload(t, eventID, stripeEventTypeInvoiceMarkedUncollectible, accountID, map[string]any{
		"id":      stripeInvoiceID,
		objectKey: stripeObjectInvoice,
	})
}

// webhookActivityRow is one activity row an Invoice webhook left on an
// Engagement, read back via the superuser Admin connection.
type webhookActivityRow struct {
	actorKind     string
	actorClientID sql.NullString
	diff          map[string]any
}

func webhookActivityRows(t *testing.T, db *testdb.DB, engagementID, action string) []webhookActivityRow {
	t.Helper()
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT actor_kind, actor_client_id, diff FROM activity
		 WHERE subject_kind = 'engagement' AND subject_id = $1 AND action = $2`, engagementID, action,
	)
	if err != nil {
		t.Fatalf("query activity: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var out []webhookActivityRow
	for rows.Next() {
		var row webhookActivityRow
		var diff []byte
		if err := rows.Scan(&row.actorKind, &row.actorClientID, &diff); err != nil {
			t.Fatalf("scan activity: %v", err)
		}
		if err := json.Unmarshal(diff, &row.diff); err != nil {
			t.Fatalf("decode diff: %v", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate activity: %v", err)
	}
	return out
}

func failedAttemptClient(methodType string, methodErr error) *referenceClient {
	return &referenceClient{
		StripeAPIClient: payments.NewStripeAPIClient("sk_test_unused", "https://app.test"),
		methodType:      methodType,
		methodErr:       methodErr,
	}
}

// TestPostConnectWebhookHandler_FailedAttemptLeavesTheInvoicePayable is
// #2224: Stripe sends invoice.payment_failed on every failed attempt and
// keeps the Invoice open, so ours stays open, the Client's own pay route
// still hands her the secret to try again, and the attempt is recorded as
// her act with the method it was made with.
func TestPostConnectWebhookHandler_FailedAttemptLeavesTheInvoicePayable(t *testing.T) {
	cases := map[string]string{
		"declined card":     "card",
		"failed bank debit": "us_bank_account",
	}
	for name, methodType := range cases {
		t.Run(name, func(t *testing.T) {
			w := newPayWorld(t, "failed-"+methodType)
			invoiceID := seedInvoice(t, w.db, w.me.practiceID, w.me.contractID, "in_failed_"+methodType, invoiceStatusOpen, 5000, time.Now())
			srv := newConnectWebhookServerWith(w.db, failedAttemptClient(methodType, nil))
			defer srv.Close()

			resp := postConnectWebhook(t, srv, invoicePaymentFailedPayload(t, "evt_failed_"+methodType, payAccountID, "in_failed_"+methodType), stripeConnectWebhookSecret)
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			}

			if status, _ := invoiceStatusAndPaidAt(t, w.db, invoiceID); status != invoiceStatusOpen {
				t.Fatalf("invoice status = %q, want %q", status, invoiceStatusOpen)
			}
			if got := paymentsForInvoice(t, w.db, invoiceID); len(got) != 0 {
				t.Fatalf("payments for invoice = %d, want 0", len(got))
			}
			if status, body := w.get(t, invoiceID); status != http.StatusOK {
				t.Fatalf("pay again: status %d body %s, want 200", status, body)
			}

			rows := webhookActivityRows(t, w.db, w.me.engagementID, actionInvoicePaymentFailed)
			if len(rows) != 1 {
				t.Fatalf("%s rows = %d, want 1", actionInvoicePaymentFailed, len(rows))
			}
			if rows[0].actorKind != string(activity.ActorClient) || rows[0].actorClientID.String != w.me.clientID {
				t.Fatalf("actor = %s %v, want the Client %s", rows[0].actorKind, rows[0].actorClientID, w.me.clientID)
			}
			if rows[0].diff[activity.DiffKeyPaymentMethodType] != methodType {
				t.Fatalf("diff = %v, want %s %q", rows[0].diff, activity.DiffKeyPaymentMethodType, methodType)
			}
		})
	}
}

// TestPostConnectWebhookHandler_FailedAttemptRecordedWhenTheMethodLookupFails
// holds the rule invoice.paid follows for its reference: a failure to read
// the method from Stripe leaves it out of the row, and is no reason to 500.
func TestPostConnectWebhookHandler_FailedAttemptRecordedWhenTheMethodLookupFails(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "Method Lookup Practice", "acct_method_fail")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedDraftContract(t, db, engagementID)
	invoiceID := seedInvoice(t, db, practiceID, contractID, "in_method_fail", invoiceStatusOpen, 5000, time.Now())
	srv := newConnectWebhookServerWith(db, failedAttemptClient("", errRetrieveFailed))
	defer srv.Close()

	resp := postConnectWebhook(t, srv, invoicePaymentFailedPayload(t, "evt_method_fail", "acct_method_fail", "in_method_fail"), stripeConnectWebhookSecret)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if status, _ := invoiceStatusAndPaidAt(t, db, invoiceID); status != invoiceStatusOpen {
		t.Fatalf("invoice status = %q, want %q", status, invoiceStatusOpen)
	}
	rows := webhookActivityRows(t, db, engagementID, actionInvoicePaymentFailed)
	if len(rows) != 1 {
		t.Fatalf("%s rows = %d, want 1", actionInvoicePaymentFailed, len(rows))
	}
	if _, ok := rows[0].diff[activity.DiffKeyPaymentMethodType]; ok {
		t.Fatalf("diff = %v, want no method", rows[0].diff)
	}
}

// TestPostConnectWebhookHandler_FailedAttemptReplayRecordsOnce proves a
// redelivered invoice.payment_failed leaves one row, not two.
func TestPostConnectWebhookHandler_FailedAttemptReplayRecordsOnce(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "Failed Replay Practice", "acct_failed_replay")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedDraftContract(t, db, engagementID)
	seedInvoice(t, db, practiceID, contractID, "in_failed_replay", invoiceStatusOpen, 5000, time.Now())
	srv := newConnectWebhookServerWith(db, failedAttemptClient("card", nil))
	defer srv.Close()

	payload := invoicePaymentFailedPayload(t, "evt_failed_replay", "acct_failed_replay", "in_failed_replay")
	for range 2 {
		resp := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
	}
	if rows := webhookActivityRows(t, db, engagementID, actionInvoicePaymentFailed); len(rows) != 1 {
		t.Fatalf("%s rows = %d, want 1", actionInvoicePaymentFailed, len(rows))
	}
}

// markedUncollectibleWorld is a connected Practice with one Stripe-backed
// Invoice in status, and the Connect webhook in front of it.
func markedUncollectibleWorld(t *testing.T, accountID, stripeInvoiceID, status string) (db *testdb.DB, engagementID, invoiceID string, srv *httptest.Server) {
	t.Helper()
	db = testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "Marked Uncollectible Practice", accountID)
	_, engagementID = testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedDraftContract(t, db, engagementID)
	invoiceID = seedInvoice(t, db, practiceID, contractID, stripeInvoiceID, status, 5000, time.Now())
	srv = newConnectWebhookServer(db)
	t.Cleanup(srv.Close)
	return db, engagementID, invoiceID, srv
}

// TestPostConnectWebhookHandler_MarkedUncollectibleWritesTheInvoiceOff is
// the one way a Stripe-backed Invoice becomes uncollectible: Stripe says
// so, and the row is DoulaCloud's, since no Staff member here did it.
func TestPostConnectWebhookHandler_MarkedUncollectibleWritesTheInvoiceOff(t *testing.T) {
	db, engagementID, invoiceID, srv := markedUncollectibleWorld(t, "acct_marked", "in_marked", invoiceStatusOpen)

	payload := invoiceMarkedUncollectiblePayload(t, "evt_marked", "acct_marked", "in_marked")
	for range 2 {
		resp := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
	}

	if status, _ := invoiceStatusAndPaidAt(t, db, invoiceID); status != invoiceStatusUncollectible {
		t.Fatalf("invoice status = %q, want %q", status, invoiceStatusUncollectible)
	}
	rows := webhookActivityRows(t, db, engagementID, actionInvoiceWrittenOff)
	if len(rows) != 1 {
		t.Fatalf("%s rows = %d, want 1 across a delivery and its replay", actionInvoiceWrittenOff, len(rows))
	}
	if rows[0].actorKind != string(activity.ActorSystem) {
		t.Fatalf("actor_kind = %q, want %q", rows[0].actorKind, activity.ActorSystem)
	}
}

// TestPostConnectWebhookHandler_MarkedUncollectibleLeavesAPaidInvoiceAlone
// proves only an open Invoice moves: a paid one keeps its status and gains
// no row, and the event is still acknowledged.
func TestPostConnectWebhookHandler_MarkedUncollectibleLeavesAPaidInvoiceAlone(t *testing.T) {
	db, engagementID, invoiceID, srv := markedUncollectibleWorld(t, "acct_marked_paid", "in_marked_paid", invoiceStatusPaid)

	resp := postConnectWebhook(t, srv, invoiceMarkedUncollectiblePayload(t, "evt_marked_paid", "acct_marked_paid", "in_marked_paid"), stripeConnectWebhookSecret)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if status, _ := invoiceStatusAndPaidAt(t, db, invoiceID); status != invoiceStatusPaid {
		t.Fatalf("invoice status = %q, want %q", status, invoiceStatusPaid)
	}
	if rows := webhookActivityRows(t, db, engagementID, actionInvoiceWrittenOff); len(rows) != 0 {
		t.Fatalf("%s rows = %d, want 0", actionInvoiceWrittenOff, len(rows))
	}
}

// TestPostConnectWebhookHandler_MarkedUncollectibleUnknownInvoiceDroppedButAcknowledged
// mirrors the other Invoice events: an Invoice DoulaCloud never raised is
// logged and dropped with a 200, so Stripe does not retry it forever.
func TestPostConnectWebhookHandler_MarkedUncollectibleUnknownInvoiceDroppedButAcknowledged(t *testing.T) {
	_, _, _, srv := markedUncollectibleWorld(t, "acct_marked_unknown", "in_marked_known", invoiceStatusOpen)

	resp := postConnectWebhook(t, srv, invoiceMarkedUncollectiblePayload(t, "evt_marked_unknown", "acct_marked_unknown", "in_marked_missing"), stripeConnectWebhookSecret)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// TestPostConnectWebhookHandler_MarkedUncollectibleMalformedObjectRejected
// proves an Invoice object that cannot be read is refused rather than
// applied with zero values.
func TestPostConnectWebhookHandler_MarkedUncollectibleMalformedObjectRejected(t *testing.T) {
	srv := newConnectWebhookServer(testdb.New(t))
	defer srv.Close()

	payload := buildConnectEventPayload(t, "evt_marked_malformed", stripeEventTypeInvoiceMarkedUncollectible, "acct_malformed", map[string]any{"id": 12345})
	resp := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}
