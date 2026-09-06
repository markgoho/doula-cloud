package client_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/testdb"
)

// seedMappedCustomer writes a client_stripe_customers row for a Client on
// one connected account, dated `age` ago -- the mapping #780 added, which
// is where a Client's Customer is recorded from now on.
func seedMappedCustomer(t *testing.T, db *testdb.DB, practiceID, clientID, accountID, customerID string, age time.Duration) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO client_stripe_customers (practice_id, client_id, stripe_account_id, stripe_customer_id, created_at)
		 VALUES ($1, $2, $3, $4, now() - $5::interval)`,
		practiceID, clientID, accountID, customerID, age.String(),
	); err != nil {
		t.Fatalf("seed client_stripe_customers: %v", err)
	}
}

// TestEraseHandler_ReachesEveryCustomerSheEverHad covers the mixed case
// erasure has to survive: a Client whose Customers are recorded in two
// different places at once. A row written before #780 carries a Customer
// id on the invoice itself and nothing in the mapping, while the ordinary
// case since -- an Invoice raised against the mapped Customer -- is
// recorded in both. Both must be deleted, and the one recorded twice must
// be deleted once. A Customer that is mapped but never billed is the
// third shape, covered by the test below.
func TestEraseHandler_ReachesEveryCustomerSheEverHad(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-erase-every-customer"
	practiceID, staffID := seedOwner(t, db, uid)
	clientID := seedFullClient(t, db, practiceID, staffID)

	// Historical: one Invoice, from before the mapping existed.
	seedInvoicedClient(t, db, practiceID, clientID, "cus_historical", "paid", 100*24*time.Hour)
	// Mapped and billed: recorded in both places, so it must not be
	// deleted twice.
	seedInvoicedClient(t, db, practiceID, clientID, "cus_mapped_and_billed", "paid", 100*24*time.Hour)
	seedMappedCustomer(t, db, practiceID, clientID, testConnectAccount, "cus_mapped_and_billed", 120*24*time.Hour)

	srv, session := newErasureServer(t, db, uid)
	defer srv.Close()

	resp := postErasure(t, session, srv, practiceID, clientID)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	acts := readOutbox(t, db, clientID)
	for _, customerID := range []string{"cus_historical", "cus_mapped_and_billed"} {
		if _, ok := acts["stripe_customer_delete|"+customerID]; !ok {
			t.Fatalf("no customer-delete row queued for %s: %+v", customerID, acts)
		}
		if _, ok := acts["stripe_redaction_job|"+customerID]; !ok {
			t.Fatalf("no redaction row queued for %s: %+v", customerID, acts)
		}
	}
	// Two Customers, one delete and one redaction each. A third delete
	// would mean the Customer recorded in both places was queued twice.
	if len(acts) != 4 {
		t.Fatalf("outbox rows = %+v, want exactly one delete and one redaction per Customer", acts)
	}
}

// TestEraseHandler_MappedCustomerWithNoInvoiceAgesFromItsOwnCreation
// covers the date a Customer with no invoice behind it is judged by: when
// it was made. A run allocates a Client's Customer before it raises
// anything, and a Customer with no transactions must still become
// redactable on a schedule rather than never.
func TestEraseHandler_MappedCustomerWithNoInvoiceAgesFromItsOwnCreation(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-erase-mapped-only"
	practiceID, staffID := seedOwner(t, db, uid)
	clientID := seedFullClient(t, db, practiceID, staffID)
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET stripe_connect_account_id = $2 WHERE id = $1`, practiceID, testConnectAccount,
	); err != nil {
		t.Fatalf("seed connect account: %v", err)
	}
	seedMappedCustomer(t, db, practiceID, clientID, testConnectAccount, "cus_never_billed", 10*24*time.Hour)

	srv, session := newErasureServer(t, db, uid)
	defer srv.Close()

	resp := postErasure(t, session, srv, practiceID, clientID)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	acts := readOutbox(t, db, clientID)
	redact, ok := acts["stripe_redaction_job|cus_never_billed"]
	if !ok {
		t.Fatalf("no redaction row queued: %+v", acts)
	}
	// Made 10 days ago, so 80 days to wait -- the same 90-day floor an
	// invoice is judged by, measured from the Customer's own creation.
	if wait := time.Until(redact); wait < 79*24*time.Hour || wait > 81*24*time.Hour {
		t.Fatalf("redaction due in %v, want about 80 days", wait)
	}
}
