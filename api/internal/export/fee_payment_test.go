package export_test

import (
	"testing"

	"doula-cloud/api/internal/fixture"
)

// TestHandler_ExportCarriesFeePayments proves a Fee Payment (#2271) is
// part of the Practice's archive, joined back to its Attachment.
func TestHandler_ExportCarriesFeePayments(t *testing.T) {
	e := newEnv(t, fixture.WithEngagement())
	lena := e.Colleague(fixture.Contractor())

	var attachmentID string
	if err := e.DB.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by, fee_amount_cents)
		 VALUES ($1, $2, 'granted', $2, 45000) RETURNING id`, e.Engagement.ID, lena.ID,
	).Scan(&attachmentID); err != nil {
		t.Fatalf("seed attachment: %v", err)
	}
	e.Exec(`INSERT INTO attachment_fee_payments
		     (practice_id, attachment_id, kind, amount_cents, method, paid_on, recorded_by, recorded_at)
		 VALUES ($1, $2, 'recorded', 45000, 'check', '2026-10-01', $3, now())`,
		e.Practice.ID, attachmentID, e.Staff.ID)

	payment := onlyRow(t, "doula_fee_payment.csv", e.csvFile("doula_fee_payment.csv"))

	if got := payment.get("attachment_id"); got != attachmentID {
		t.Fatalf("attachment_id = %q, want %q", got, attachmentID)
	}
	if got := payment.get("amount_cents"); got != "45000" {
		t.Fatalf("amount_cents = %q, want 45000", got)
	}
}
