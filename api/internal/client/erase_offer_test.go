package client_test

import (
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

// TestEraseHandler_RedactsOfferClientFields proves erasure empties the
// initial, area and due date every Offer under her Engagements copied from
// her (#2385), open or decided, counts them on the erased row, and keeps
// the Engagement's own due date.
func TestEraseHandler_RedactsOfferClientFields(t *testing.T) {
	e := newEraseEnv(t)
	engagementID := e.activeEngagement()
	lena := e.Colleague(fixture.Contractor())
	e.Exec(`UPDATE engagements SET due_date = '2027-02-14' WHERE id = $1`, engagementID)
	e.Exec(`INSERT INTO engagement_offers
	            (engagement_id, staff_id, employment_type, amount_cents, client_first_initial, client_area,
	             due_date, state, offered_by, expires_at)
	        SELECT $1, $2, 'contractor', 45000, 'A', 'North side', '2027-02-14', s, $3, now() + interval '7 days'
	          FROM unnest(ARRAY['accepted', 'offered']::offer_state[]) AS s`,
		engagementID, lena.ID, e.Staff.ID)

	e.erase()

	var scope struct {
		Offers int `json:"offers"`
	}
	e.readErasureScope(&scope)
	if scope.Offers != 2 {
		t.Fatalf("erasureScope.offers = %d, want 2", scope.Offers)
	}
	if remaining := testdb.Scalar[int](t, e.DB.Admin,
		`SELECT count(*) FROM engagement_offers
		  WHERE engagement_id = $1
		    AND (client_first_initial IS NOT NULL OR client_area IS NOT NULL OR due_date IS NOT NULL)`,
		engagementID); remaining != 0 {
		t.Fatalf("offers still carrying her fields = %d, want 0", remaining)
	}
	if due := testdb.Scalar[string](t, e.DB.Admin, `SELECT due_date::text FROM engagements WHERE id = $1`, engagementID); due != "2027-02-14" {
		t.Fatalf("engagements.due_date = %q, want it kept", due)
	}
}
