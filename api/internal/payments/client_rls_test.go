package payments_test

import (
	"database/sql"
	"testing"
	"time"

	"doula-cloud/api/internal/testdb"
)

// These tests exercise invoices_client_visibility and
// payments_client_visibility from 00119 directly via db.App and
// set_config('app.current_client_id'), in the shape of invoice_rls_test.go
// and payments_rls_test.go (#1019). A Client-portal session sets the Client
// variable and never the Practice one, so that is all they set.

// clientTx begins a db.App transaction that looks like a Client-portal
// session for clientID. The caller rolls it back.
func clientTx(t *testing.T, db *testdb.DB, clientID string) *sql.Tx {
	t.Helper()
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_client_id', $1, true)`, clientID); err != nil {
		t.Fatalf("set_config: %v", err)
	}
	return tx
}

// visibleCount is how many rows of table the session sees.
func visibleCount(t *testing.T, tx *sql.Tx, table string) int {
	t.Helper()
	var count int
	// table is always a literal from this file, never input.
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM `+table).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}

// TestRLS_InvoicesClientReadsEveryStatusButDraft proves the policy's
// status filter: a Client sees her own open, paid, void and uncollectible
// Invoices and never a draft one.
func TestRLS_InvoicesClientReadsEveryStatusButDraft(t *testing.T) {
	cases := map[string]int{
		invoiceStatusDraft:         0,
		invoiceStatusOpen:          1,
		invoiceStatusPaid:          1,
		invoiceStatusVoid:          1,
		invoiceStatusUncollectible: 1,
	}
	for status, want := range cases {
		t.Run(status, func(t *testing.T) {
			db := testdb.New(t)
			practiceID := testdb.SeedPractice(t, db, "Practice")
			clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
			contractID := seedSignedContract(t, db, engagementID)
			seedInvoice(t, db, practiceID, contractID, "in_client_"+status, status, 5000, time.Now())

			if got := visibleCount(t, clientTx(t, db, clientID), "invoices"); got != want {
				t.Fatalf("Client sees %d %s Invoices, want %d", got, status, want)
			}
		})
	}
}

// TestRLS_InvoicesClientCannotReadAnotherClientsOrPracticesInvoice proves
// the ownership test: another Client's Invoice at the same Practice, and
// an Invoice at another Practice, both stay invisible.
func TestRLS_InvoicesClientCannotReadAnotherClientsOrPracticesInvoice(t *testing.T) {
	db := testdb.New(t)
	practiceA := testdb.SeedPractice(t, db, "Practice A")
	practiceB := testdb.SeedPractice(t, db, "Practice B")
	clientA, engagementA := testdb.SeedNamedEngagement(t, db, practiceA, "Client A", "a@example.com")
	_, engagementSibling := testdb.SeedNamedEngagement(t, db, practiceA, "Client Sibling", "sibling@example.com")
	_, engagementB := testdb.SeedNamedEngagement(t, db, practiceB, "Client B", "b@example.com")
	own := seedInvoice(t, db, practiceA, seedSignedContract(t, db, engagementA), "in_client_own", invoiceStatusOpen, 5000, time.Now())
	seedInvoice(t, db, practiceA, seedSignedContract(t, db, engagementSibling), "in_client_sibling", invoiceStatusOpen, 6000, time.Now())
	seedInvoice(t, db, practiceB, seedSignedContract(t, db, engagementB), "in_client_other_practice", invoiceStatusOpen, 7000, time.Now())

	tx := clientTx(t, db, clientA)
	var id string
	if err := tx.QueryRowContext(t.Context(), `SELECT id FROM invoices`).Scan(&id); err != nil {
		t.Fatalf("Client A should see exactly her own Invoice: %v", err)
	}
	if id != own {
		t.Fatalf("visible Invoice = %q, want %q", id, own)
	}
	if got := visibleCount(t, tx, "invoices"); got != 1 {
		t.Fatalf("Client A sees %d Invoices, want 1", got)
	}
}

// TestRLS_InvoicesClientCannotWrite proves the Client population gains no
// INSERT or UPDATE on invoices: the insert is rejected, and the update
// touches no row.
func TestRLS_InvoicesClientCannotWrite(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Practice")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	seedInvoice(t, db, practiceID, contractID, "in_client_write", invoiceStatusOpen, 5000, time.Now())

	tx := clientTx(t, db, clientID)
	res, err := tx.ExecContext(t.Context(), `UPDATE invoices SET status = 'paid'`)
	if err != nil {
		t.Fatalf("update invoices: %v", err)
	}
	if n, _ := res.RowsAffected(); n != 0 {
		t.Fatalf("Client update touched %d invoices rows, want 0", n)
	}

	if _, err := tx.ExecContext(t.Context(),
		`INSERT INTO invoices (practice_id, contract_id, stripe_invoice_id, amount_cents, reference, due_at) VALUES ($1, $2, 'in_client_insert', 5000, 'in_client_insert', now())`,
		practiceID, contractID,
	); err == nil {
		t.Fatal("expected a Client-portal insert into invoices to be rejected by RLS, got no error")
	}
}

// TestRLS_PaymentsClientReadsOwnPaymentsOnly proves payments_client_visibility
// reaches the same test one join away: her own Payment is visible, another
// Client's and another Practice's are not.
func TestRLS_PaymentsClientReadsOwnPaymentsOnly(t *testing.T) {
	db := testdb.New(t)
	practiceA := testdb.SeedPractice(t, db, "Practice A")
	practiceB := testdb.SeedPractice(t, db, "Practice B")
	clientA, engagementA := testdb.SeedNamedEngagement(t, db, practiceA, "Client A", "a@example.com")
	_, engagementSibling := testdb.SeedNamedEngagement(t, db, practiceA, "Client Sibling", "sibling@example.com")
	_, engagementB := testdb.SeedNamedEngagement(t, db, practiceB, "Client B", "b@example.com")
	own := seedInvoice(t, db, practiceA, seedSignedContract(t, db, engagementA), "in_pay_own", invoiceStatusPaid, 5000, time.Now())
	sibling := seedInvoice(t, db, practiceA, seedSignedContract(t, db, engagementSibling), "in_pay_sibling", invoiceStatusPaid, 6000, time.Now())
	other := seedInvoice(t, db, practiceB, seedSignedContract(t, db, engagementB), "in_pay_other", invoiceStatusPaid, 7000, time.Now())
	ownPayment := seedPayment(t, db, own, "pi_client_own", 5000, time.Now())
	seedPayment(t, db, sibling, "pi_client_sibling", 6000, time.Now())
	seedPayment(t, db, other, "pi_client_other", 7000, time.Now())

	tx := clientTx(t, db, clientA)
	var id string
	if err := tx.QueryRowContext(t.Context(), `SELECT id FROM payments`).Scan(&id); err != nil {
		t.Fatalf("Client A should see exactly her own Payment: %v", err)
	}
	if id != ownPayment {
		t.Fatalf("visible Payment = %q, want %q", id, ownPayment)
	}
	if got := visibleCount(t, tx, "payments"); got != 1 {
		t.Fatalf("Client A sees %d Payments, want 1", got)
	}
}

// TestRLS_PaymentsClientCannotReadPaymentOnDraftInvoice proves the draft
// exclusion is in the payments policy itself, not only inherited from the
// invoices one.
func TestRLS_PaymentsClientCannotReadPaymentOnDraftInvoice(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Practice")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	invoiceID := seedInvoice(t, db, practiceID, seedSignedContract(t, db, engagementID), "in_pay_draft", invoiceStatusDraft, 5000, time.Now())
	seedPayment(t, db, invoiceID, "pi_client_draft", 5000, time.Now())

	if got := visibleCount(t, clientTx(t, db, clientID), "payments"); got != 0 {
		t.Fatalf("Client sees %d Payments on a draft Invoice, want 0", got)
	}
}

// TestRLS_PaymentsClientCannotWrite proves the Client population gains no
// INSERT on payments.
func TestRLS_PaymentsClientCannotWrite(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Practice")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	invoiceID := seedInvoice(t, db, practiceID, seedSignedContract(t, db, engagementID), "in_pay_write", invoiceStatusOpen, 5000, time.Now())

	tx := clientTx(t, db, clientID)
	if _, err := tx.ExecContext(t.Context(),
		`INSERT INTO payments (invoice_id, stripe_payment_reference, amount_cents, paid_at, kind) VALUES ($1, 'pi_client_insert', 5000, now(), 'stripe')`,
		invoiceID,
	); err == nil {
		t.Fatal("expected a Client-portal insert into payments to be rejected by RLS, got no error")
	}
}
