package practicedeletion_test

import (
	"testing"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/testdb"
)

// TestWorker_FinalizeRedactsEveryClientsOfferFields proves a Practice
// Deletion (ADR-0031) reaches an Offer's copy of the Client's fields
// (#2385) through the same client.Erase it runs per Client.
func TestWorker_FinalizeRedactsEveryClientsOfferFields(t *testing.T) {
	db := testdb.New(t)
	practiceID, ownerID := testdb.SeedStaffAtNewPractice(t, db, "owner-finalize-offer", []string{ownerRole}, "employee")
	markPendingDeletion(t, db, practiceID, time.Now())
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	lenaID := testdb.SeedContractorAtPractice(t, db, practiceID, "finalize-offer-lena")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_offers
		     (engagement_id, staff_id, employment_type, amount_cents, client_first_initial, client_area,
		      due_date, offered_by, expires_at)
		 VALUES ($1, $2, 'contractor', 45000, 'A', 'North side', '2027-02-14', $3, now() + interval '7 days')`,
		engagementID, lenaID, ownerID,
	); err != nil {
		t.Fatalf("seed offer: %v", err)
	}

	seedOutboxRow(t, db, practiceID, "finalize", time.Now().Add(-time.Minute))
	runWorker(t, db, newTestWorker(&mail.FakeSender{}))

	var remaining int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM engagement_offers
		  WHERE engagement_id = $1
		    AND (client_first_initial IS NOT NULL OR client_area IS NOT NULL OR due_date IS NOT NULL)`,
		engagementID,
	).Scan(&remaining); err != nil {
		t.Fatalf("count offer fields: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("offers still carrying her fields = %d, want 0", remaining)
	}
}
