package payments_test

import (
	"strings"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

// These tests exercise the client_stripe_customers_practice_visibility
// policy from 00076_client_stripe_customers.sql directly via db.App and
// set_config, the same way invoice_rls_test.go does for the invoices
// policy it is modeled on. The mapping names a Client and a Stripe
// Customer, so a Practice reading another Practice's row would be reading
// which of its Clients it bills.

// seedClientCustomer writes one mapping row directly, for the Client the
// Engagement is for.
func seedClientCustomer(t *testing.T, db *testdb.DB, engagement fixture.EngagementRecord, accountID, customerID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO client_stripe_customers (practice_id, client_id, stripe_account_id, stripe_customer_id)
		 VALUES ($1, $2, $3, $4)`,
		engagement.PracticeID, engagement.Client.ID, accountID, customerID,
	); err != nil {
		t.Fatalf("seed client_stripe_customers: %v", err)
	}
}

// TestRLS_ClientStripeCustomersFailsClosedWithNoSessionVarSet proves the
// mapping denies all rows when app.current_practice_id is unset.
func TestRLS_ClientStripeCustomersFailsClosedWithNoSessionVarSet(t *testing.T) {
	db := testdb.New(t)
	seedClientCustomer(t, db, fixture.Engagement(t, db, fixture.Practice(t, db)), "acct_rls_closed", "cus_rls_closed")

	count := testdb.Scalar[int](t, db.App, `SELECT count(*) FROM client_stripe_customers`)
	if count != 0 {
		t.Fatalf("expected 0 rows with no session variables set, got %d", count)
	}
}

// TestRLS_ClientStripeCustomersIsScopedToCurrentPractice proves Practice
// A's session sees only Practice A's mappings.
func TestRLS_ClientStripeCustomersIsScopedToCurrentPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA, practiceB := fixture.Practice(t, db), fixture.Practice(t, db)
	seedClientCustomer(t, db, fixture.Engagement(t, db, practiceA), "acct_rls_a", "cus_rls_a")
	seedClientCustomer(t, db, fixture.Engagement(t, db, practiceB), "acct_rls_b", "cus_rls_b")

	tx := db.BeginAs(t, testdb.Practice(practiceA.ID))
	visible := testdb.Column[string](t, tx, `SELECT stripe_customer_id FROM client_stripe_customers`)

	if len(visible) != 1 || visible[0] != "cus_rls_a" {
		t.Fatalf("visible mappings = %v, want only %q", visible, "cus_rls_a")
	}
}

// TestRLS_ClientStripeCustomersCannotInsertForAnotherPractice proves the
// WITH CHECK side, derived from the same USING clause: Practice B's
// session cannot claim a Customer for Practice A's Client.
func TestRLS_ClientStripeCustomersCannotInsertForAnotherPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA, practiceB := fixture.Practice(t, db), fixture.Practice(t, db)
	engagementA := fixture.Engagement(t, db, practiceA)

	tx := db.BeginAs(t, testdb.Practice(practiceB.ID))
	_, err := tx.ExecContext(t.Context(),
		`INSERT INTO client_stripe_customers (practice_id, client_id, stripe_account_id, stripe_customer_id)
		 VALUES ($1, $2, $3, $4)`,
		practiceA.ID, engagementA.Client.ID, "acct_rls_cross", "cus_rls_cross")
	if err == nil {
		t.Fatal("expected the cross-practice INSERT to be rejected, got no error")
	}
	if !strings.Contains(err.Error(), "row-level security") {
		t.Fatalf("expected a row-level security error, got: %v", err)
	}
}

// TestGrant_ClientStripeCustomersHasNoUpdateOrDelete proves the mapping
// is written once and read after that: repointing a Client at a different
// Stripe Customer is not an operation this product has, and erasure
// deletes the Customer at Stripe rather than the row recording that it
// existed.
func TestGrant_ClientStripeCustomersHasNoUpdateOrDelete(t *testing.T) {
	db := testdb.New(t)
	seedClientCustomer(t, db, fixture.Engagement(t, db, fixture.Practice(t, db)), "acct_grant", "cus_grant")

	if _, err := db.App.ExecContext(t.Context(),
		`UPDATE client_stripe_customers SET stripe_customer_id = 'cus_other'`,
	); err == nil {
		t.Fatal("expected UPDATE to be rejected, got no error")
	} else if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected a permission-denied error, got: %v", err)
	}

	if _, err := db.App.ExecContext(t.Context(),
		`DELETE FROM client_stripe_customers`,
	); err == nil {
		t.Fatal("expected DELETE to be rejected, got no error")
	} else if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected a permission-denied error, got: %v", err)
	}
}
