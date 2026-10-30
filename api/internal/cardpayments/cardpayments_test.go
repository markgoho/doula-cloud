package cardpayments_test

import (
	"testing"

	"doula-cloud/api/internal/cardpayments"
	"doula-cloud/api/internal/testdb"
)

func TestClientsCanPay(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Card Payments Practice")

	read := func() bool {
		t.Helper()
		tx, err := db.Admin.BeginTx(t.Context(), nil)
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		defer func() { _ = tx.Rollback() }()
		canPay, err := cardpayments.ClientsCanPay(t.Context(), tx, practiceID)
		if err != nil {
			t.Fatalf("ClientsCanPay: %v", err)
		}
		return canPay
	}

	if read() {
		t.Fatal("ClientsCanPay = true before card payments are active, want false")
	}

	for _, status := range []cardpayments.Status{cardpayments.Pending, cardpayments.Restricted} {
		if _, err := db.Admin.ExecContext(t.Context(),
			`UPDATE practices SET stripe_connect_card_payments_status = $1 WHERE id = $2`, string(status), practiceID,
		); err != nil {
			t.Fatalf("set status %q: %v", status, err)
		}
		if read() {
			t.Fatalf("ClientsCanPay = true at %q, want false", status)
		}
	}

	testdb.SeedClientsCanPay(t, db, practiceID)

	if !read() {
		t.Fatal("ClientsCanPay = false once card payments are active, want true")
	}
}
