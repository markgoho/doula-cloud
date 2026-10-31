package client_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/testdb"
)

// TestEraseHandler_RedactsOfferClientFields proves erasure empties the
// initial, area and due date every Offer under her Engagements copied from
// her (#2385), open or decided, counts them on the erased row, and keeps
// the Engagement's own due date.
func TestEraseHandler_RedactsOfferClientFields(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-erase-offer"
	practiceID, ownerID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	clientID, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Ada Lovelace", "ada@example.com", "active")
	lenaID := testdb.SeedContractorAtPractice(t, db, practiceID, "erase-offer-lena")
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagements SET due_date = '2027-02-14' WHERE id = $1`, engagementID); err != nil {
		t.Fatalf("seed engagement due date: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_offers
		     (engagement_id, staff_id, employment_type, amount_cents, client_first_initial, client_area,
		      due_date, state, offered_by, expires_at)
		 SELECT $1, $2, 'contractor', 45000, 'A', 'North side', '2027-02-14', s, $3, now() + interval '7 days'
		   FROM unnest(ARRAY['accepted', 'offered']::offer_state[]) AS s`,
		engagementID, lenaID, ownerID,
	); err != nil {
		t.Fatalf("seed offers: %v", err)
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
		Offers int `json:"offers"`
	}
	if err := json.Unmarshal(diff, &scope); err != nil {
		t.Fatalf("decode erasure scope: %v", err)
	}
	if scope.Offers != 2 {
		t.Fatalf("erasureScope.offers = %d, want 2", scope.Offers)
	}

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

	var engagementDue string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT due_date::text FROM engagements WHERE id = $1`, engagementID,
	).Scan(&engagementDue); err != nil {
		t.Fatalf("read engagement due date: %v", err)
	}
	if engagementDue != "2027-02-14" {
		t.Fatalf("engagements.due_date = %q, want it kept", engagementDue)
	}
}
