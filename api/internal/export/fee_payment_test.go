package export_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/testdb"
)

// TestHandler_ExportCarriesFeePayments proves a Fee Payment (#2271) is
// part of the Practice's archive, joined back to its Attachment.
func TestHandler_ExportCarriesFeePayments(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-export-fee"
	practiceID, ownerID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Ada Lovelace", "ada@example.com")
	lenaID := testdb.SeedContractorAtPractice(t, db, practiceID, "export-fee-lena")

	var attachmentID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by, fee_amount_cents)
		 VALUES ($1, $2, 'granted', $2, 45000) RETURNING id`, engagementID, lenaID,
	).Scan(&attachmentID); err != nil {
		t.Fatalf("seed attachment: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO attachment_fee_payments
		     (practice_id, attachment_id, kind, amount_cents, method, paid_on, recorded_by, recorded_at)
		 VALUES ($1, $2, 'recorded', 45000, 'check', '2026-10-01', $3, now())`,
		practiceID, attachmentID, ownerID,
	); err != nil {
		t.Fatalf("seed fee payment: %v", err)
	}

	srv, session := newServer(t, db, uid)
	defer srv.Close()
	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/export")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	rows := parseCSV(t, readZIP(t, resp)["doula_fee_payment.csv"])
	if len(rows) != 2 {
		t.Fatalf("doula_fee_payment.csv rows = %d, want header + 1", len(rows))
	}
	if got := rows[1][indexOf(t, rows[0], "attachment_id")]; got != attachmentID {
		t.Fatalf("attachment_id = %q, want %q", got, attachmentID)
	}
	if got := rows[1][indexOf(t, rows[0], "amount_cents")]; got != "45000" {
		t.Fatalf("amount_cents = %q, want 45000", got)
	}
}
