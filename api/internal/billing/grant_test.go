package billing_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/testdb"
)

// testGrantor is who issues every grant in these tests -- the platform
// operator's own name, which is what a real request carries.
const testGrantor = "mark@doula.cloud"

// seedRoster puts n Staff on practiceID's roster, which is what a
// founding grant is sized from.
func seedRoster(t *testing.T, db *testdb.DB, practiceID string, n int) {
	t.Helper()
	for i := range n {
		testdb.SeedStaffAtPractice(t, db, practiceID, t.Name()+"-staff-"+string(rune('a'+i)), []string{"doula"}, "employee")
	}
}

// foundingGrantRow reads the one founding grant row a Practice holds.
func foundingGrantRow(t *testing.T, db *testdb.DB, practiceID string) (quantity int, grantedBy string, createdAt time.Time) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT quantity, granted_by, created_at FROM credit_ledger
		 WHERE practice_id = $1 AND origin = 'founding_grant'`, practiceID,
	).Scan(&quantity, &grantedBy, &createdAt); err != nil {
		t.Fatalf("read founding grant row: %v", err)
	}
	return quantity, grantedBy, createdAt
}

// TestFoundingGrant_SizesTheGrantFromTheRosterAndNamesWhoIssuedIt is the
// ticket in one test: the fourteen-doula Rochester agency #439 sized this
// for gets forty-two Credits, and "who gave this Practice its Credits?"
// has an answer on the row itself.
func TestFoundingGrant_SizesTheGrantFromTheRosterAndNamesWhoIssuedIt(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Rochester Agency")
	seedRoster(t, db, practiceID, 14)

	tx := practiceTx(t, db, practiceID)
	receipt, err := billing.FoundingGrant(t.Context(), tx, practiceID, testGrantor)
	if err != nil {
		t.Fatalf("FoundingGrant: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if receipt.StaffCount != 14 || receipt.Credits != 42 {
		t.Fatalf("receipt = %+v, want 14 staff and 42 credits", receipt)
	}

	quantity, grantedBy, createdAt := foundingGrantRow(t, db, practiceID)
	if quantity != 42 {
		t.Fatalf("granted quantity = %d, want 42", quantity)
	}
	if grantedBy != testGrantor {
		t.Fatalf("granted_by = %q, want mark@doula.cloud", grantedBy)
	}
	if createdAt.IsZero() {
		t.Fatalf("created_at = %v, want a recorded timestamp", createdAt)
	}
}

// TestFoundingGrant_GrantsThreePerStaffForASoloPractice proves the same
// rule at the other end of the pilot: one doula, three Credits.
func TestFoundingGrant_GrantsThreePerStaffForASoloPractice(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Solo Doula")
	seedRoster(t, db, practiceID, 1)

	tx := practiceTx(t, db, practiceID)
	receipt, err := billing.FoundingGrant(t.Context(), tx, practiceID, testGrantor)
	if err != nil {
		t.Fatalf("FoundingGrant: %v", err)
	}
	if receipt.Credits != billing.FoundingGrantPerStaff {
		t.Fatalf("credits = %d, want %d", receipt.Credits, billing.FoundingGrantPerStaff)
	}
}

// TestFoundingGrant_RefusesASecondGrant proves the grant is counted once
// and never topped up (#439): a second issue is refused, not silently
// doubled.
func TestFoundingGrant_RefusesASecondGrant(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Granted Twice")
	seedRoster(t, db, practiceID, 2)

	tx := practiceTx(t, db, practiceID)
	if _, err := billing.FoundingGrant(t.Context(), tx, practiceID, testGrantor); err != nil {
		t.Fatalf("first grant: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	second := practiceTx(t, db, practiceID)
	if _, err := billing.FoundingGrant(t.Context(), second, practiceID, testGrantor); !errors.Is(err, billing.ErrAlreadyGranted) {
		t.Fatalf("second grant error = %v, want ErrAlreadyGranted", err)
	}

	var balance int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT COALESCE(SUM(quantity), 0) FROM credit_ledger WHERE practice_id = $1`, practiceID,
	).Scan(&balance); err != nil {
		t.Fatalf("sum balance: %v", err)
	}
	if balance != 6 {
		t.Fatalf("balance = %d, want 6 -- the refused grant must not have been written", balance)
	}
}

// TestFoundingGrant_RefusesAPracticeWithNoStaff proves the operator is
// told why rather than being handed a constraint violation.
func TestFoundingGrant_RefusesAPracticeWithNoStaff(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Empty Roster")

	tx := practiceTx(t, db, practiceID)
	if _, err := billing.FoundingGrant(t.Context(), tx, practiceID, testGrantor); !errors.Is(err, billing.ErrNoStaff) {
		t.Fatalf("error = %v, want ErrNoStaff", err)
	}
}

// TestFoundingGrant_RefusesAnUnnamedGrantor proves the audit field cannot
// be skipped, and that whitespace does not stand in for a name.
func TestFoundingGrant_RefusesAnUnnamedGrantor(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Nameless Grantor")
	seedRoster(t, db, practiceID, 1)

	for _, grantedBy := range []string{"", "   "} {
		tx := practiceTx(t, db, practiceID)
		if _, err := billing.FoundingGrant(t.Context(), tx, practiceID, grantedBy); !errors.Is(err, billing.ErrNoGrantor) {
			t.Fatalf("grantedBy %q: error = %v, want ErrNoGrantor", grantedBy, err)
		}
	}
}

// TestFoundingGrant_IsSpentBeforePurchasedCreditsAndIsNotRefundable
// proves the two consequences #449 said the implementation must not
// break, both of which fall out of the lot being priced at a real $0.00:
// FIFO reaches the grant first, and the refund path passes over it
// instead of computing a $0.00 refund.
func TestFoundingGrant_IsSpentBeforePurchasedCreditsAndIsNotRefundable(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Grant Then Purchase")
	seedRoster(t, db, practiceID, 1)

	grantTx := practiceTx(t, db, practiceID)
	if _, err := billing.FoundingGrant(t.Context(), grantTx, practiceID, testGrantor); err != nil {
		t.Fatalf("FoundingGrant: %v", err)
	}
	if err := grantTx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	// An hour later, explicitly, for the reason
	// TestConsumeCredit_DrawsTheOldestLotFirst gives: the grant is
	// stamped with the database's clock and this with Go's.
	purchaseID := seedPurchase(t, db, practiceID, 2, 2000, 100, "pi_after_grant", time.Now().Add(time.Hour))

	engagements := seedEngagements(t, db, practiceID, 3)
	tx := practiceTx(t, db, practiceID)
	for i, engagementID := range engagements {
		if err := billing.ConsumeCredit(t.Context(), tx, practiceID, engagementID); err != nil {
			t.Fatalf("consume %d: %v", i, err)
		}
	}

	// Three Credits spent against a three-Credit grant and a two-Credit
	// purchase: FIFO means all three came out of the grant, so the
	// purchase is untouched.
	var drawsOnPurchase int
	if err := tx.QueryRowContext(t.Context(),
		`SELECT count(*) FROM credit_ledger WHERE drawn_lot_id = $1`, purchaseID,
	).Scan(&drawsOnPurchase); err != nil {
		t.Fatalf("count draws on purchase: %v", err)
	}
	if drawsOnPurchase != 0 {
		t.Fatalf("draws on the purchase = %d, want 0 -- the grant must be spent first", drawsOnPurchase)
	}

	// And the grant itself is worth nothing back: the quote counts the
	// two purchased Credits only, at the price paid for them.
	quote, err := billing.Refundable(t.Context(), tx, practiceID, time.Now())
	if err != nil {
		t.Fatalf("Refundable: %v", err)
	}
	if quote.Credits != 2 || quote.AmountCents != 4000 {
		t.Fatalf("quote = %+v, want 2 purchased credits at 4000 cents", quote)
	}
}

// TestFoundingGrant_IsAllThatIsRefusedWhenOnlyAGrantIsHeld proves the
// refund path says "nothing refundable" rather than issuing a $0.00
// refund against a lot that was never paid for.
func TestFoundingGrant_IsAllThatIsRefusedWhenOnlyAGrantIsHeld(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Grant Only")
	seedRoster(t, db, practiceID, 1)

	tx := practiceTx(t, db, practiceID)
	if _, err := billing.FoundingGrant(t.Context(), tx, practiceID, testGrantor); err != nil {
		t.Fatalf("FoundingGrant: %v", err)
	}
	client := billing.NewFakeStripeClient()
	if _, err := billing.Refund(t.Context(), tx, client, practiceID, t.Name(), 1, time.Now()); !errors.Is(err, billing.ErrNothingRefundable) {
		t.Fatalf("Refund error = %v, want ErrNothingRefundable", err)
	}
	if calls := client.RefundCalls(); len(calls) != 0 {
		t.Fatalf("stripe refund calls = %+v, want none", calls)
	}
}

// TestFoundingGrantHandler_IssuesTheGrant proves the operator endpoint
// that actually issues one: authorized by the same X-Internal-Secret the
// refund uses, sized from the roster, and recorded.
func TestFoundingGrantHandler_IssuesTheGrant(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Grant Endpoint")
	seedRoster(t, db, practiceID, 4)
	srv := newInternalBillingServer(db, billing.NewFakeStripeClient())

	status, body := internalRequest(t, srv, http.MethodPost, "/api/internal/billing/founding-grants", internalTestSecret,
		`{"practiceId":"`+practiceID+`","grantedBy":"`+testGrantor+`"}`)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", status, body)
	}
	var receipt billing.FoundingGrantReceipt
	if err := json.Unmarshal(body, &receipt); err != nil {
		t.Fatalf("decode receipt: %v", err)
	}
	if receipt.StaffCount != 4 || receipt.Credits != 12 || receipt.GrantedBy != testGrantor {
		t.Fatalf("receipt = %+v, want 4 staff, 12 credits, mark@doula.cloud", receipt)
	}

	quantity, grantedBy, _ := foundingGrantRow(t, db, practiceID)
	if quantity != 12 || grantedBy != testGrantor {
		t.Fatalf("row = {%d, %q}, want {12, mark@doula.cloud}", quantity, grantedBy)
	}
}

// TestFoundingGrantHandler_RefusesTheSecondRequest proves a retyped
// command is answered with a conflict rather than a second grant.
func TestFoundingGrantHandler_RefusesTheSecondRequest(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Twice Over HTTP")
	seedRoster(t, db, practiceID, 1)
	srv := newInternalBillingServer(db, billing.NewFakeStripeClient())
	body := `{"practiceId":"` + practiceID + `","grantedBy":"` + testGrantor + `"}`

	if status, out := internalRequest(t, srv, http.MethodPost, "/api/internal/billing/founding-grants", internalTestSecret, body); status != http.StatusOK {
		t.Fatalf("first status = %d, want 200: %s", status, out)
	}
	status, out := internalRequest(t, srv, http.MethodPost, "/api/internal/billing/founding-grants", internalTestSecret, body)
	if status != http.StatusConflict {
		t.Fatalf("second status = %d, want 409: %s", status, out)
	}

	var rows int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM credit_ledger WHERE practice_id = $1 AND origin = 'founding_grant'`, practiceID,
	).Scan(&rows); err != nil {
		t.Fatalf("count grant rows: %v", err)
	}
	if rows != 1 {
		t.Fatalf("founding grant rows = %d, want 1", rows)
	}
}

// TestFoundingGrantHandler_RefusesAPracticeWithNoStaff and the three
// refusals below prove the endpoint's own guards: an empty roster, a
// missing grantor, a malformed Practice id, a malformed body, and no
// secret at all.
func TestFoundingGrantHandler_Refusals(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Endpoint Refusals")
	srv := newInternalBillingServer(db, billing.NewFakeStripeClient())
	staffed := seedPractice(t, db, "Endpoint Refusals Staffed")
	seedRoster(t, db, staffed, 1)

	for _, tc := range []struct {
		name   string
		secret string
		body   string
		want   int
	}{
		{"no secret", "", `{"practiceId":"` + practiceID + `","grantedBy":"m"}`, http.StatusUnauthorized},
		{"malformed body", internalTestSecret, `{`, http.StatusBadRequest},
		{"malformed practice id", internalTestSecret, `{"practiceId":"not-a-uuid","grantedBy":"m"}`, http.StatusBadRequest},
		{"unnamed grantor", internalTestSecret, `{"practiceId":"` + staffed + `"}`, http.StatusBadRequest},
		{"no staff", internalTestSecret, `{"practiceId":"` + practiceID + `","grantedBy":"m"}`, http.StatusConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, body := internalRequest(t, srv, http.MethodPost, "/api/internal/billing/founding-grants", tc.secret, tc.body)
			if status != tc.want {
				t.Fatalf("status = %d, want %d: %s", status, tc.want, body)
			}
		})
	}
}
