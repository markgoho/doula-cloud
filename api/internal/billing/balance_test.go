package billing_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// ownerRole and doulaRole are named once so golangci-lint's goconst check
// doesn't see repeated "owner"/"doula" literals across this package's
// whole test surface.
const (
	ownerRole = "owner"
	doulaRole = "doula"
)

// seedLedgerRow seeds one lot. A purchase carries what it cost and the
// payment it arrived on (#420); a grant carries the defaults, which are a
// real $0.00 rather than an unknown price.
func seedLedgerRow(t *testing.T, db *testdb.DB, practiceID, origin string, quantity int) string {
	t.Helper()
	unitPriceCents, taxCents := 0, 0
	var paymentIntentID any
	if origin == originPurchase {
		unitPriceCents, paymentIntentID = seedUnitPriceCents, "pi_seed"
	}
	id := testdb.Scalar[string](t, db.Admin, `INSERT INTO credit_ledger (practice_id, origin, quantity, unit_price_cents, tax_cents, stripe_payment_intent_id)
		 VALUES ($1, $2::credit_ledger_origin, $3, $4, $5, $6) RETURNING id`,
		practiceID, origin, quantity, unitPriceCents, taxCents, paymentIntentID)
	return id
}

// seedRefundRow seeds a refund drawn against lotID: a negative row that
// names the lot it reverses, which is the only shape #420 admits.
func seedRefundRow(t *testing.T, db *testdb.DB, practiceID, lotID string, quantity int) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO credit_ledger (practice_id, origin, quantity, unit_price_cents, tax_cents, stripe_refund_id, drawn_lot_id, refund_request_key, operator_act_id)
		 VALUES ($1, 'refund', $2, $3, 0, 're_seed', $4, 'req-seed', $5)`,
		practiceID, -quantity, seedUnitPriceCents, lotID, testdb.SeedOperatorAct(t, db, "Mark Goho"),
	); err != nil {
		t.Fatalf("seed refund row: %v", err)
	}
}

// seedUnitPriceCents is what a seeded purchase charged per Credit -- the
// live $20.00 Price (#448).
const seedUnitPriceCents = 2000

// originPurchase is credit_ledger's purchase origin, the one origin the
// seeds treat specially because it is the only one that cost money.
const originPurchase = "purchase"

// TestBalance_SumsLedgerRows exercises the billing package boundary AC
// #75 calls for: a Practice's derived balance after a sequence of
// credit_ledger rows of different origins and signs, and the coalesced
// zero (not a scan error or an odd value) of a Practice with none.
func TestBalance_SumsLedgerRows(t *testing.T) {
	cases := []struct {
		name string
		seed func(t *testing.T, db *testdb.DB, practiceID string)
		want int
	}{
		{
			name: "rows of different origins and signs",
			seed: func(t *testing.T, db *testdb.DB, practiceID string) {
				seedLedgerRow(t, db, practiceID, "signup_bonus", 3)
				lotID := seedLedgerRow(t, db, practiceID, "purchase", 5)
				seedRefundRow(t, db, practiceID, lotID, 1)
			},
			want: 7,
		},
		{name: "no rows at all", seed: func(*testing.T, *testdb.DB, string) {}, want: 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db := testdb.New(t)
			practiceID := testdb.SeedPractice(t, db, "Test Practice")
			c.seed(t, db, practiceID)
			tx, err := db.Admin.BeginTx(t.Context(), nil)
			if err != nil {
				t.Fatalf("begin: %v", err)
			}
			defer func() { _ = tx.Rollback() }()

			got, err := billing.Balance(t.Context(), tx, practiceID)
			if err != nil {
				t.Fatalf("Balance: %v", err)
			}
			if got != c.want {
				t.Fatalf("balance = %d, want %d", got, c.want)
			}
		})
	}
}

// TestGetBalanceHandler_AdminAllowed proves ADR-0008's read table admits
// Admin, not just Owner (AC #75's "no Owner-only restriction" survives as
// "no Owner-only restriction among Owner/Admin", the read table's own
// widening).
func TestGetBalanceHandler_AdminAllowed(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Admin()))
	e.seedLedger("signup_bonus", 3)
	e.seedLedger("purchase", 5)

	out := e.balance()

	if out.Balance != 8 {
		t.Fatalf("balance = %d, want 8", out.Balance)
	}
	if len(out.Ledger.Items) != 2 {
		t.Fatalf("ledger = %+v, want 2 entries", out.Ledger)
	}
	// Most recent first: the purchase (seeded second) precedes the
	// signup_bonus (seeded first).
	if out.Ledger.Items[0].Origin != "purchase" || out.Ledger.Items[0].Quantity != 5 {
		t.Fatalf("ledger[0] = %+v, want purchase +5", out.Ledger.Items[0])
	}
	if out.Ledger.Items[1].Origin != "signup_bonus" || out.Ledger.Items[1].Quantity != 3 {
		t.Fatalf("ledger[1] = %+v, want signup_bonus +3", out.Ledger.Items[1])
	}
}

// TestGetBalanceHandler_EmptyLedgerReturnsZeroBalance proves a brand-new
// Practice with no ledger rows gets a 0 balance and an empty (not null)
// ledger array, and that the read carries the price of a Credit the
// shipped Terms state today, with no change to come (#285, #1834).
func TestGetBalanceHandler_EmptyLedgerReturnsZeroBalance(t *testing.T) {
	e := newEnv(t)

	out := e.balance()

	if out.Balance != 0 {
		t.Fatalf("balance = %d, want 0", out.Balance)
	}
	if out.Ledger.Items == nil || len(out.Ledger.Items) != 0 {
		t.Fatalf("ledger = %+v, want empty non-nil slice", out.Ledger)
	}
	if out.Price.UnitAmountCents != seedUnitPriceCents || out.Price.Currency != usd {
		t.Fatalf("price = %+v, want {%d usd}", out.Price, seedUnitPriceCents)
	}
	if out.ComingPrice != nil {
		t.Fatalf("comingPrice = %+v, want none: the shipped Terms announce no change", out.ComingPrice)
	}
}

// TestGetBalanceHandler_DoulaForbidden proves ADR-0008's read table cell
// "Credit balance and ledger: Doula ✗" holds through the real
// GatedRouter mount, for both employment types -- neither an employee's
// ambient reach over the Practice nor a contractor's attachment to any
// Engagement extends to the Practice's own finances.
func TestGetBalanceHandler_DoulaForbidden(t *testing.T) {
	for name, staff := range map[string][]fixture.StaffOption{
		"an employee":  {fixture.Doula()},
		"a contractor": {fixture.Doula(), fixture.Contractor()},
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, signedInAs(staff...))

			e.balanceAt("", http.StatusForbidden)
		})
	}
}

// TestGetBalanceHandler_InvalidCursorRejected mirrors
// message.TestListHandler_InvalidCursorRejected.
func TestGetBalanceHandler_InvalidCursorRejected(t *testing.T) {
	e := newEnv(t)

	for _, cursor := range []string{"not!valid!base64!", "YmFkdGltZXxzb21lLWlk"} {
		e.balanceAt("?cursor="+cursor, http.StatusBadRequest)
	}
}

// TestGetBalanceHandler_PaginatesNewestFirst seeds more than one page of
// credit_ledger rows and walks the cursor, mirroring
// message.TestListHandler_PaginatesNewestFirst.
func TestGetBalanceHandler_PaginatesNewestFirst(t *testing.T) {
	e := newEnv(t)
	const total = 31 // pageSize (30) + 1, to force a second page
	for range total {
		e.seedLedger("purchase", 1)
	}

	first := e.balance()
	if len(first.Ledger.Items) != 30 || !first.Ledger.HasMore || first.Ledger.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 30/true/non-nil",
			len(first.Ledger.Items), first.Ledger.HasMore, first.Ledger.NextCursor)
	}

	second := reqtest.Expect[billing.BalanceResponse](t, http.MethodGet, e.PracticeURL("/billing?cursor="+*first.Ledger.NextCursor), http.StatusOK, e.SignedIn)
	if len(second.Ledger.Items) != 1 || second.Ledger.HasMore || second.Ledger.NextCursor != nil {
		t.Fatalf("second page = %d items, hasMore=%v, cursor=%v; want 1/false/nil",
			len(second.Ledger.Items), second.Ledger.HasMore, second.Ledger.NextCursor)
	}
}
