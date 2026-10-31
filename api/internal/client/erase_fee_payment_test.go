package client_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/testdb"
)

// TestEraseHandler_RedactsFeePaymentText proves erasure empties a Fee
// Payment's note and its reversal's reason (#2271) on her Engagement, the
// same sweep a Payment's note and reason already get.
func TestEraseHandler_RedactsFeePaymentText(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-erase-fee-payment"
	practiceID, ownerID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	clientID, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Ada Lovelace", "ada@example.com", "active")
	lenaID := testdb.SeedContractorAtPractice(t, db, practiceID, "erase-fee-lena")

	var attachmentID, paymentID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by, fee_amount_cents)
		 VALUES ($1, $2, 'granted', $2, 45000) RETURNING id`, engagementID, lenaID,
	).Scan(&attachmentID); err != nil {
		t.Fatalf("seed attachment: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO attachment_fee_payments
		     (practice_id, attachment_id, kind, amount_cents, method, note, paid_on, recorded_by, recorded_at)
		 VALUES ($1, $2, 'recorded', 45000, 'other', 'for Ada''s birth', '2026-10-01', $3, now()) RETURNING id`,
		practiceID, attachmentID, ownerID,
	).Scan(&paymentID); err != nil {
		t.Fatalf("seed fee payment: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO attachment_fee_payments
		     (practice_id, attachment_id, kind, amount_cents, target_id, reason, recorded_by, recorded_at)
		 VALUES ($1, $2, 'reversal', -45000, $3, 'not Ada''s birth', $4, now())`,
		practiceID, attachmentID, paymentID, ownerID,
	); err != nil {
		t.Fatalf("seed reversal: %v", err)
	}

	srv, session := newServer(t, db, uid)
	defer srv.Close()
	resp := postErasure(t, session, srv, practiceID, clientID)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var diff []byte
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT diff FROM activity WHERE subject_kind = 'client' AND subject_id = $1 AND action = 'erased'`, clientID,
	).Scan(&diff); err != nil {
		t.Fatalf("query erased activity row: %v", err)
	}
	var scope struct {
		PaymentNotes           int `json:"paymentNotes"`
		PaymentReversalReasons int `json:"paymentReversalReasons"`
	}
	if err := json.Unmarshal(diff, &scope); err != nil {
		t.Fatalf("decode erasure scope: %v", err)
	}
	if scope.PaymentNotes != 1 || scope.PaymentReversalReasons != 1 {
		t.Fatalf("erasure scope = %+v, want one note and one reason", scope)
	}

	var remaining int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM attachment_fee_payments WHERE attachment_id = $1 AND (note IS NOT NULL OR reason IS NOT NULL)`,
		attachmentID,
	).Scan(&remaining); err != nil {
		t.Fatalf("count text: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("fee payment rows still carrying text = %d, want 0", remaining)
	}
}
