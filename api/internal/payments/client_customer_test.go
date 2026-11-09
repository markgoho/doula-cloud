package payments_test

import (
	"database/sql"
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
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
	clientID := testdb.Scalar[string](t, db.Admin, `SELECT client_id FROM engagements WHERE id = $1`, engagementID)
	return clientID
}

// TestPostInvoiceHandler_SecondInvoiceBillsTheSameCustomer proves #780's
// central rule at the seam that decides it: a Client has at most one
// Stripe Customer per connected account, so her second Invoice bills the
// Customer her first one made rather than raising a fresh one. Before
// this, a Client billed six times had six Customers in her Practice's
// Stripe account, which is not the single Customer GLOSSARY.md's Erasure
// entry has always described.
func TestPostInvoiceHandler_SecondInvoiceBillsTheSameCustomer(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Doula()), connected())
	engagementID, _ := e.signedContract()

	first := e.raise(engagementID)
	second := e.raise(engagementID)
	if first.ID == second.ID {
		t.Fatal("both posts returned the same Invoice, want two")
	}

	client := e.Stripe
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

	mapped, createdBy := mappedCustomer(t, e.DB, clientOfEngagement(t, e.DB, engagementID), e.AccountID)
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
	e := newEnv(t, signedInAs(fixture.Doula()), connected())
	engagementID, _ := e.signedContract()
	const preAllocated = "cus_on_a_test_clock"
	if _, err := e.DB.Admin.ExecContext(t.Context(),
		`INSERT INTO client_stripe_customers (practice_id, client_id, stripe_account_id, stripe_customer_id)
		 VALUES ($1, $2, $3, $4)`,
		e.Practice.ID, clientOfEngagement(t, e.DB, engagementID), e.AccountID, preAllocated,
	); err != nil {
		t.Fatalf("seed pre-allocated customer: %v", err)
	}

	e.raise(engagementID)

	client := e.Stripe
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
	stripe := payments.NewFakeClient()
	stripe.CreateCustomerErr = errStripeFake
	e := newEnv(t, signedInAs(fixture.Doula()), withStripe(stripe), connected())
	engagementID, _ := e.signedContract()

	e.refuseRaise(engagementID, http.StatusInternalServerError)

	e.requireNothingRaised()
}
