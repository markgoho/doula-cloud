package payments_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/testdb"
)

// mappedCustomer reads the Customer client_stripe_customers holds for a
// Client on a connected account, and who is recorded as having caused it
// to exist.
func mappedCustomer(t *testing.T, db *testdb.DB, clientID, accountID string) (customerID string, createdBy sql.NullString) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT stripe_customer_id, created_by_staff_id FROM client_stripe_customers
		  WHERE client_id = $1 AND stripe_account_id = $2`,
		clientID, accountID,
	).Scan(&customerID, &createdBy); err != nil {
		t.Fatalf("read customer mapping: %v", err)
	}
	return customerID, createdBy
}

// clientOfEngagement resolves the Client an Engagement is for.
func clientOfEngagement(t *testing.T, db *testdb.DB, engagementID string) string {
	t.Helper()
	var clientID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT client_id FROM engagements WHERE id = $1`, engagementID,
	).Scan(&clientID); err != nil {
		t.Fatalf("read engagement client: %v", err)
	}
	return clientID
}

// createdInvoice posts an Invoice and returns the created row's view,
// failing the test on any non-201.
func createdInvoice(t *testing.T, srv *httptest.Server, session, practiceID, engagementID string, amountCents int64) payments.InvoiceView {
	t.Helper()
	resp := postInvoice(t, srv, session, practiceID, engagementID, amountCents)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var out payments.PostInvoiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Invoice == nil {
		t.Fatal("invoice is nil, want a created Invoice")
	}
	return *out.Invoice
}

// TestPostInvoiceHandler_SecondInvoiceBillsTheSameCustomer proves #780's
// central rule at the seam that decides it: a Client has at most one
// Stripe Customer per connected account, so her second Invoice bills the
// Customer her first one made rather than raising a fresh one. Before
// this, a Client billed six times had six Customers in her Practice's
// Stripe account, which is not the single Customer CONTEXT.md's Erasure
// entry has always described.
func TestPostInvoiceHandler_SecondInvoiceBillsTheSameCustomer(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-one-customer"
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	seedContract(t, db, engagementID)
	const accountID = "acct_one_customer"
	seedConnectAccount(t, db, practiceID, accountID)

	client := payments.NewFakeClient()
	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	first := createdInvoice(t, srv, session, practiceID, engagementID, 15000)
	second := createdInvoice(t, srv, session, practiceID, engagementID, 22000)
	if first.ID == second.ID {
		t.Fatal("both posts returned the same Invoice, want two")
	}

	if len(client.CreateCustomerCalls) != 1 {
		t.Fatalf("CreateCustomer calls = %d, want 1 across two Invoices", len(client.CreateCustomerCalls))
	}
	if len(client.CreateInvoiceCalls) != 2 {
		t.Fatalf("CreateInvoice calls = %d, want 2", len(client.CreateInvoiceCalls))
	}
	billed := client.CreateInvoiceCalls[0].CustomerID
	if client.CreateInvoiceCalls[1].CustomerID != billed {
		t.Fatalf("second Invoice billed customer %q, want the first one's %q",
			client.CreateInvoiceCalls[1].CustomerID, billed)
	}

	mapped, createdBy := mappedCustomer(t, db, clientOfEngagement(t, db, engagementID), accountID)
	if mapped != billed {
		t.Fatalf("mapped customer = %q, want the billed %q", mapped, billed)
	}
	if !createdBy.Valid || createdBy.String == "" {
		t.Fatal("created_by_staff_id is null, want the Staff who raised the Invoice")
	}
}

// TestPostInvoiceHandler_PreExistingMappingIsUsedUnchanged proves the
// harness path #780 exists for: when the mapping row is already there --
// written from outside the product, against a Stripe test clock -- the
// product finds a Customer and creates none. This is what lets api/ carry
// no test-only parameter at all.
func TestPostInvoiceHandler_PreExistingMappingIsUsedUnchanged(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-preallocated"
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	seedContract(t, db, engagementID)
	const accountID = "acct_preallocated"
	seedConnectAccount(t, db, practiceID, accountID)

	clientID := clientOfEngagement(t, db, engagementID)
	const preAllocated = "cus_on_a_test_clock"
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO client_stripe_customers (practice_id, client_id, stripe_account_id, stripe_customer_id)
		 VALUES ($1, $2, $3, $4)`,
		practiceID, clientID, accountID, preAllocated,
	); err != nil {
		t.Fatalf("seed pre-allocated customer: %v", err)
	}

	client := payments.NewFakeClient()
	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	createdInvoice(t, srv, session, practiceID, engagementID, 15000)

	if len(client.CreateCustomerCalls) != 0 {
		t.Fatalf("CreateCustomer calls = %d, want 0 -- the Customer was already allocated", len(client.CreateCustomerCalls))
	}
	if len(client.CreateInvoiceCalls) != 1 {
		t.Fatalf("CreateInvoice calls = %d, want 1", len(client.CreateInvoiceCalls))
	}
	if got := client.CreateInvoiceCalls[0].CustomerID; got != preAllocated {
		t.Fatalf("billed customer = %q, want the pre-allocated %q", got, preAllocated)
	}
}

// TestPostInvoiceHandler_CustomerFailureRaisesNoInvoice proves a Stripe
// failure while making the Customer stops before any Invoice is raised --
// neither at Stripe nor in this database.
func TestPostInvoiceHandler_CustomerFailureRaisesNoInvoice(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-customer-fails"
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	seedContract(t, db, engagementID)
	seedConnectAccount(t, db, practiceID, "acct_customer_fails")

	client := payments.NewFakeClient()
	client.CreateCustomerErr = errStripeFake
	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	resp := postInvoice(t, srv, session, practiceID, engagementID, 15000)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
	if len(client.CreateInvoiceCalls) != 0 {
		t.Fatalf("CreateInvoice calls = %d, want 0", len(client.CreateInvoiceCalls))
	}
	var invoices int
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM invoices`).Scan(&invoices); err != nil {
		t.Fatalf("count invoices: %v", err)
	}
	if invoices != 0 {
		t.Fatalf("invoices rows = %d, want 0", invoices)
	}
}
