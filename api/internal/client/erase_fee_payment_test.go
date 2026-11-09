package client_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// TestEraseHandler_RedactsFeePaymentText proves erasure empties a Fee
// Payment's note and its reversal's reason (#2271) on her Engagement, the
// same sweep a Payment's note and reason already get.
func TestEraseHandler_RedactsFeePaymentText(t *testing.T) {
	e := newEraseEnv(t)
	engagementID := e.activeEngagement()
	lenaID := testdb.SeedContractorAtPractice(t, e.db, e.Practice.ID, "erase-fee-lena")

	var attachmentID, paymentID string
	if err := e.db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by, fee_amount_cents)
		 VALUES ($1, $2, 'granted', $2, 45000) RETURNING id`, engagementID, lenaID,
	).Scan(&attachmentID); err != nil {
		t.Fatalf("seed attachment: %v", err)
	}
	if err := e.db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO attachment_fee_payments
		     (practice_id, attachment_id, kind, amount_cents, method, note, paid_on, recorded_by, recorded_at)
		 VALUES ($1, $2, 'recorded', 45000, 'other', 'for Ada''s birth', '2026-10-01', $3, now()) RETURNING id`,
		e.Practice.ID, attachmentID, e.Owner.ID,
	).Scan(&paymentID); err != nil {
		t.Fatalf("seed fee payment: %v", err)
	}
	if _, err := e.db.Admin.ExecContext(t.Context(),
		`INSERT INTO attachment_fee_payments
		     (practice_id, attachment_id, kind, amount_cents, target_id, reason, recorded_by, recorded_at)
		 VALUES ($1, $2, 'reversal', -45000, $3, 'not Ada''s birth', $4, now())`,
		e.Practice.ID, attachmentID, paymentID, e.Owner.ID,
	); err != nil {
		t.Fatalf("seed reversal: %v", err)
	}
	reqtest.ExpectStatus(t, http.MethodPost, e.erasureURL(), http.StatusOK, e.signedIn)

	var scope struct {
		PaymentNotes           int `json:"paymentNotes"`
		PaymentReversalReasons int `json:"paymentReversalReasons"`
	}
	e.readErasureScope(&scope)
	if scope.PaymentNotes != 1 || scope.PaymentReversalReasons != 1 {
		t.Fatalf("erasure scope = %+v, want one note and one reason", scope)
	}

	var remaining int
	if err := e.db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM attachment_fee_payments WHERE attachment_id = $1 AND (note IS NOT NULL OR reason IS NOT NULL)`,
		attachmentID,
	).Scan(&remaining); err != nil {
		t.Fatalf("count text: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("fee payment rows still carrying text = %d, want 0", remaining)
	}
}
