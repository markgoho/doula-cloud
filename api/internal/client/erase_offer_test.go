package client_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// TestEraseHandler_RedactsOfferClientFields proves erasure empties the
// initial, area and due date every Offer under her Engagements copied from
// her (#2385), open or decided, counts them on the erased row, and keeps
// the Engagement's own due date.
func TestEraseHandler_RedactsOfferClientFields(t *testing.T) {
	e := newEraseEnv(t)
	engagementID := e.activeEngagement()
	lenaID := testdb.SeedContractorAtPractice(t, e.db, e.Practice.ID, "erase-offer-lena")
	if _, err := e.db.Admin.ExecContext(t.Context(),
		`UPDATE engagements SET due_date = '2027-02-14' WHERE id = $1`, engagementID); err != nil {
		t.Fatalf("seed engagement due date: %v", err)
	}
	if _, err := e.db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_offers
		     (engagement_id, staff_id, employment_type, amount_cents, client_first_initial, client_area,
		      due_date, state, offered_by, expires_at)
		 SELECT $1, $2, 'contractor', 45000, 'A', 'North side', '2027-02-14', s, $3, now() + interval '7 days'
		   FROM unnest(ARRAY['accepted', 'offered']::offer_state[]) AS s`,
		engagementID, lenaID, e.Owner.ID,
	); err != nil {
		t.Fatalf("seed offers: %v", err)
	}
	reqtest.ExpectStatus(t, http.MethodPost, e.erasureURL(), http.StatusOK, e.signedIn)

	var scope struct {
		Offers int `json:"offers"`
	}
	e.readErasureScope(&scope)
	if scope.Offers != 2 {
		t.Fatalf("erasureScope.offers = %d, want 2", scope.Offers)
	}

	var remaining int
	if err := e.db.Admin.QueryRowContext(t.Context(),
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

	var engagementDue string
	if err := e.db.Admin.QueryRowContext(t.Context(),
		`SELECT due_date::text FROM engagements WHERE id = $1`, engagementID,
	).Scan(&engagementDue); err != nil {
		t.Fatalf("read engagement due date: %v", err)
	}
	if engagementDue != "2027-02-14" {
		t.Fatalf("engagements.due_date = %q, want it kept", engagementDue)
	}
}
