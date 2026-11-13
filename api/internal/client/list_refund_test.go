package client_test

import (
	"strings"
	"testing"

	"doula-cloud/api/internal/testdb"
)

const paidStatus = "paid"

// TestListHandler_OpenEngagementsRollup_RefundedCents proves #2849 on the
// Clients list: a paid Invoice some of which went back carries how much,
// the figure the Invoices list's row carries, so both lists say it was
// paid and the refund alike. A contractor, who receives no Invoice state
// (ADR-0008), receives no refund either.
func TestListHandler_OpenEngagementsRollup_RefundedCents(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Refund Rollup Practice")
	const ownerUID = "staff-refund-rollup-owner"
	testdb.SeedStaffAtPractice(t, db, practiceID, ownerUID, []string{ownerRole}, "employee")
	const contractorUID = "contractor-refund-rollup"
	contractorID := testdb.SeedContractorAtPractice(t, db, practiceID, contractorUID)
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Camille Boyd", "camille@example.com", "active")
	seedGrantedAttachmentWithFee(t, db, engagementID, contractorID, 120000)
	contractID := seedClientContract(t, db, engagementID, "signed")
	invoiceID := testdb.Scalar[string](t, db.Admin,
		`INSERT INTO invoices (practice_id, contract_id, status, amount_cents, currency, reference, due_at, paid_at)
		 VALUES ($1, $2, 'paid', 150000, 'usd', 'INV-REFUND', $3, $3) RETURNING id`, practiceID, contractID, lateDueAt)
	paymentID := testdb.Scalar[string](t, db.Admin,
		`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, method) VALUES ($1, 150000, now(), 'manual', 'check') RETURNING id`, invoiceID)
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, target_payment_id) VALUES ($1, -30000, now(), 'refund', $2)`,
		invoiceID, paymentID,
	); err != nil {
		t.Fatalf("seed refund: %v", err)
	}

	line := onlyLine(t, db, ownerUID, practiceID)
	if line.InvoiceStatus == nil || *line.InvoiceStatus != paidStatus {
		t.Fatalf("invoiceStatus = %v, want paid", line.InvoiceStatus)
	}
	if line.RefundedCents == nil || *line.RefundedCents != 30000 {
		t.Fatalf("refundedCents = %v, want 30000", line.RefundedCents)
	}

	contractorSrv, contractorSession := newServerAt(t, db, contractorUID, overdueNow)
	defer contractorSrv.Close()
	_, raw := listAt(t, contractorSrv, contractorSession, practiceID)
	if strings.Contains(raw, "refundedCents") {
		t.Fatalf("a contractor's list carries a refund: %s", raw)
	}
}
