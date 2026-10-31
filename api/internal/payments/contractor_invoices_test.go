package payments_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/testdb"
)

// The money a contractor's Invoice list must never carry (#2094). Each
// figure is one no other value in the fixture can produce by chance, so
// finding it anywhere in the body is a leak, whatever key it rides under.
const (
	leakInvoiceCents = 987654
	leakPaymentCents = 876543
	leakRefundCents  = 765432
)

// contractorInvoiceKeys is every key a contractor's Invoice row may hold
// (#2094, ADR-0008's amendment of 2026-10-26): its identity, its dates and
// its status. Anything else is a field the shape grew without a decision.
var contractorInvoiceKeys = map[string]bool{
	"id":          true,
	"status":      true,
	"raisedOn":    true,
	"dueOn":       true,
	"paidOn":      true,
	"daysOverdue": true,
	// #2225: whether a payment is on its way is a status, never an amount.
	"paymentProcessing": true,
}

// TestGetInvoicesHandler_AContractorReadsDatesAndStatusNeverMoney proves
// #2094: a contractor Doula with a granted attachment reads each Invoice
// on the Engagement as its dates and status, and no amount, currency,
// Payment or Refund reaches her -- not as a hidden or empty field, and not
// as a figure under any other key. The fixture holds every kind of money
// the Owner's list carries: an Invoice amount, a Payment that covers it,
// and a Refund against that Payment.
func TestGetInvoicesHandler_AContractorReadsDatesAndStatusNeverMoney(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-list-contractor-rows"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "contractor")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	testdb.SeedGrantedAttachment(t, db, engagementID, staffID)
	contractID := seedSignedContract(t, db, engagementID)

	base := time.Now().Add(-90 * 24 * time.Hour)
	overdueID := seedInvoiceDue(t, db, practiceID, contractID, "in_ctr_overdue", invoiceStatusOpen, leakInvoiceCents, base, base.Add(24*time.Hour))
	paidID := seedInvoice(t, db, practiceID, contractID, "in_ctr_paid", invoiceStatusPaid, leakInvoiceCents, base.Add(time.Minute))
	paymentID := seedPayment(t, db, paidID, "pi_ctr_paid", leakPaymentCents, time.Now())
	if _, err := db.Admin.ExecContext(t.Context(), `UPDATE invoices SET paid_at = now() WHERE id = $1`, paidID); err != nil {
		t.Fatalf("stamp paid_at: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, target_payment_id, method)
		 VALUES ($1, $2, now(), 'refund', $3, 'check')`, paidID, -leakRefundCents, paymentID); err != nil {
		t.Fatalf("seed refund: %v", err)
	}
	voidID := seedInvoice(t, db, practiceID, contractID, "in_ctr_void", invoiceStatusVoid, leakInvoiceCents, base.Add(2*time.Minute))
	openID := seedInvoice(t, db, practiceID, contractID, "in_ctr_open", invoiceStatusOpen, leakInvoiceCents, time.Now())

	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := getInvoices(t, srv, session, practiceID, engagementID, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	body := string(raw)

	for _, figure := range []int{leakInvoiceCents, leakPaymentCents, leakRefundCents} {
		if strings.Contains(body, strconv.Itoa(figure)) {
			t.Fatalf("a contractor's Invoice list carries the figure %d: %s", figure, body)
		}
	}
	if strings.Contains(strings.ToLower(body), "usd") {
		t.Fatalf("a contractor's Invoice list names a currency: %s", body)
	}

	var envelope struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if len(envelope.Items) != 4 {
		t.Fatalf("items = %d, want 4", len(envelope.Items))
	}
	for _, item := range envelope.Items {
		for key := range item {
			if !contractorInvoiceKeys[key] {
				t.Fatalf("a contractor's Invoice row carries %q; ADR-0008 gives her its dates and status only", key)
			}
		}
	}

	var out payments.ContractorInvoicesResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode typed: %v", err)
	}
	want := []struct {
		id, status string
		isOverdue  bool
		isPaid     bool
	}{
		{openID, invoiceStatusOpen, false, false},
		{voidID, invoiceStatusVoid, false, false},
		{paidID, invoiceStatusPaid, false, true},
		{overdueID, invoiceStatusOpen, true, false},
	}
	for i, w := range want {
		got := out.Items[i]
		if got.ID != w.id || got.Status != w.status {
			t.Fatalf("items[%d] = %+v, want id %q status %q (newest first)", i, got, w.id, w.status)
		}
		if (got.DaysOverdue > 0) != w.isOverdue {
			t.Fatalf("items[%d].daysOverdue = %d, want overdue %v", i, got.DaysOverdue, w.isOverdue)
		}
		if (got.PaidOn != nil) != w.isPaid {
			t.Fatalf("items[%d].paidOn = %v, want set %v", i, got.PaidOn, w.isPaid)
		}
		if got.RaisedOn == "" || got.DueOn == "" {
			t.Fatalf("items[%d] = %+v, want a raised day and a due day", i, got)
		}
	}
}

// TestGetInvoicesHandler_AnUnattachedContractorGets404 proves the read
// keeps ADR-0008's reach: a contractor with no granted attachment on the
// Engagement is told it is not found, the same answer an Engagement at
// another Practice gets, so she cannot learn that it exists.
func TestGetInvoicesHandler_AnUnattachedContractorGets404(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-list-contractor-unattached"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "contractor")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	seedInvoice(t, db, practiceID, contractID, "in_ctr_unattached", invoiceStatusOpen, leakInvoiceCents, time.Now())

	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := getInvoices(t, srv, session, practiceID, engagementID, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestGetInvoicesHandler_AContractorPages proves her list pages the same
// way the Owner's does, so a long billing history is not cut off for her.
func TestGetInvoicesHandler_AContractorPages(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-list-contractor-pages"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "contractor")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	testdb.SeedGrantedAttachment(t, db, engagementID, staffID)
	contractID := seedSignedContract(t, db, engagementID)
	base := time.Now().Add(-time.Hour)
	for i := range 31 {
		seedInvoice(t, db, practiceID, contractID, "in_ctr_page_"+strconv.Itoa(i), invoiceStatusOpen, 1000, base.Add(time.Duration(i)*time.Second))
	}

	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	first := getInvoices(t, srv, session, practiceID, engagementID, "")
	defer first.Body.Close()
	var page payments.ContractorInvoicesResponse
	if err := json.NewDecoder(first.Body).Decode(&page); err != nil {
		t.Fatalf("decode first page: %v", err)
	}
	if !page.HasMore || page.NextCursor == nil || len(page.Items) != 30 {
		t.Fatalf("first page = %d items, hasMore %v, cursor %v; want 30, true, a cursor", len(page.Items), page.HasMore, page.NextCursor)
	}

	second := getInvoices(t, srv, session, practiceID, engagementID, *page.NextCursor)
	defer second.Body.Close()
	var next payments.ContractorInvoicesResponse
	if err := json.NewDecoder(second.Body).Decode(&next); err != nil {
		t.Fatalf("decode second page: %v", err)
	}
	if next.HasMore || len(next.Items) != 1 {
		t.Fatalf("second page = %d items, hasMore %v; want 1, false", len(next.Items), next.HasMore)
	}
}
