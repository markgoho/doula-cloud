package export_test

import (
	"testing"

	"doula-cloud/api/internal/fixture"
)

// TestHandler_ExportWritesAnErasedClientsOfferRedacted proves an Offer
// whose Client was erased (#2385) exports with the three fields empty.
func TestHandler_ExportWritesAnErasedClientsOfferRedacted(t *testing.T) {
	e := newEnv(t, fixture.WithEngagement())
	lena := e.Colleague(fixture.Contractor())
	e.Exec(`INSERT INTO engagement_offers
		     (engagement_id, staff_id, employment_type, amount_cents, client_first_initial, client_area,
		      due_date, state, offered_by, expires_at, decided_at, decided_by, decided_by_kind)
		 VALUES ($1, $2, 'contractor', 45000, NULL, NULL, NULL, 'accepted', $3, now() + interval '7 days',
		         now(), $2, 'staff')`,
		e.Engagement.ID, lena.ID, e.Staff.ID)

	offer := onlyRow(t, "engagement_offer.csv", e.csvFile("engagement_offer.csv"))

	for _, column := range []string{"client_first_initial", "client_area", "due_date"} {
		if got := offer.get(column); got != "" {
			t.Fatalf("%s = %q, want empty", column, got)
		}
	}
	if got := offer.get("amount_cents"); got != "45000" {
		t.Fatalf("amount_cents = %q, want 45000", got)
	}
}
