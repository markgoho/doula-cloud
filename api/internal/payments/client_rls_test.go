package payments_test

import (
	"database/sql"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
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
	tx := db.BeginAs(t, testdb.Client(clientID))
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
			mine := seedBilled(t, db, fixture.Practice(t, db), "in_client_"+status, status, 5000)

			if got := visibleCount(t, clientTx(t, db, mine.clientID), "invoices"); got != want {
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
	practiceA, practiceB := fixture.Practice(t, db), fixture.Practice(t, db)
	own := seedBilled(t, db, practiceA, "in_client_own", invoiceStatusOpen, 5000)
	seedBilled(t, db, practiceA, "in_client_sibling", invoiceStatusOpen, 6000)
	seedBilled(t, db, practiceB, "in_client_other_practice", invoiceStatusOpen, 7000)

	tx := clientTx(t, db, own.clientID)
	id := testdb.Scalar[string](t, tx, `SELECT id FROM invoices`)
	if id != own.invoiceID {
		t.Fatalf("visible Invoice = %q, want %q", id, own.invoiceID)
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
	practice := fixture.Practice(t, db)
	mine := seedBilled(t, db, practice, "in_client_write", invoiceStatusOpen, 5000)

	tx := clientTx(t, db, mine.clientID)
	res, err := tx.ExecContext(t.Context(), `UPDATE invoices SET status = 'paid'`)
	if err != nil {
		t.Fatalf("update invoices: %v", err)
	}
	if n, _ := res.RowsAffected(); n != 0 {
		t.Fatalf("Client update touched %d invoices rows, want 0", n)
	}

	if _, err := tx.ExecContext(t.Context(),
		`INSERT INTO invoices (practice_id, contract_id, stripe_invoice_id, amount_cents, reference, due_at) VALUES ($1, $2, 'in_client_insert', 5000, 'in_client_insert', now())`,
		practice.ID, mine.contractID,
	); err == nil {
		t.Fatal("expected a Client-portal insert into invoices to be rejected by RLS, got no error")
	}
}

// TestRLS_PaymentsClientReadsOwnPaymentsOnly proves payments_client_visibility
// reaches the same test one join away: her own Payment is visible, another
// Client's and another Practice's are not.
func TestRLS_PaymentsClientReadsOwnPaymentsOnly(t *testing.T) {
	db := testdb.New(t)
	practiceA, practiceB := fixture.Practice(t, db), fixture.Practice(t, db)
	own := seedBilled(t, db, practiceA, "in_pay_own", invoiceStatusPaid, 5000)
	sibling := seedBilled(t, db, practiceA, "in_pay_sibling", invoiceStatusPaid, 6000)
	other := seedBilled(t, db, practiceB, "in_pay_other", invoiceStatusPaid, 7000)
	ownPayment := seedPayment(t, db, own.invoiceID, "pi_client_own", 5000, time.Now())
	seedPayment(t, db, sibling.invoiceID, "pi_client_sibling", 6000, time.Now())
	seedPayment(t, db, other.invoiceID, "pi_client_other", 7000, time.Now())

	tx := clientTx(t, db, own.clientID)
	id := testdb.Scalar[string](t, tx, `SELECT id FROM payments`)
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
	draft := seedBilled(t, db, fixture.Practice(t, db), "in_pay_draft", invoiceStatusDraft, 5000)
	seedPayment(t, db, draft.invoiceID, "pi_client_draft", 5000, time.Now())

	if got := visibleCount(t, clientTx(t, db, draft.clientID), "payments"); got != 0 {
		t.Fatalf("Client sees %d Payments on a draft Invoice, want 0", got)
	}
}

// TestRLS_PaymentsClientCannotWrite proves the Client population gains no
// INSERT on payments.
func TestRLS_PaymentsClientCannotWrite(t *testing.T) {
	db := testdb.New(t)
	open := seedBilled(t, db, fixture.Practice(t, db), "in_pay_write", invoiceStatusOpen, 5000)

	tx := clientTx(t, db, open.clientID)
	if _, err := tx.ExecContext(t.Context(),
		`INSERT INTO payments (invoice_id, stripe_payment_reference, amount_cents, paid_at, kind) VALUES ($1, 'pi_client_insert', 5000, now(), 'stripe')`,
		open.invoiceID,
	); err == nil {
		t.Fatal("expected a Client-portal insert into payments to be rejected by RLS, got no error")
	}
}
