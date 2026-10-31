package export_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/testdb"
)

// TestHandler_ExportWritesAnErasedClientsOfferRedacted proves an Offer
// whose Client was erased (#2385) exports with the three fields empty.
func TestHandler_ExportWritesAnErasedClientsOfferRedacted(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-export-erased-offer"
	practiceID, ownerID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Ada Lovelace", "ada@example.com")
	lenaID := testdb.SeedContractorAtPractice(t, db, practiceID, "export-erased-offer-lena")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_offers
		     (engagement_id, staff_id, employment_type, amount_cents, client_first_initial, client_area,
		      due_date, state, offered_by, expires_at)
		 VALUES ($1, $2, 'contractor', 45000, NULL, NULL, NULL, 'accepted', $3, now() + interval '7 days')`,
		engagementID, lenaID, ownerID,
	); err != nil {
		t.Fatalf("seed offer: %v", err)
	}

	srv, session := newServer(t, db, uid)
	defer srv.Close()
	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/export")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	rows := parseCSV(t, readZIP(t, resp)["engagement_offer.csv"])
	if len(rows) != 2 {
		t.Fatalf("engagement_offer.csv rows = %d, want header + 1", len(rows))
	}
	for _, column := range []string{"client_first_initial", "client_area", "due_date"} {
		if got := rows[1][indexOf(t, rows[0], column)]; got != "" {
			t.Fatalf("%s = %q, want empty", column, got)
		}
	}
	if got := rows[1][indexOf(t, rows[0], "amount_cents")]; got != "45000" {
		t.Fatalf("amount_cents = %q, want 45000", got)
	}
}
