package client_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/testdb"
)

// countFixture is the Practice both count tests read: four Client
// records a reader can be shown, two of them in active care, and one
// record merged into another that no reader is shown at all.
type countFixture struct {
	practiceID         string
	activeEngagementID string
	twiceEngagementID  string
}

func seedCountFixture(t *testing.T, db *testdb.DB, practiceID string) countFixture {
	t.Helper()
	activeID, activeEngagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Amara", "count-active@example.com", "active")
	testdb.SeedEngagementInStatus(t, db, practiceID, "Bex", "count-intake@example.com", "intake")
	testdb.SeedNamedClient(t, db, practiceID, "Cleo", "count-no-work@example.com")
	// Two active Engagements are still one Client in care.
	twiceID, twiceEngagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Dana", "count-twice@example.com", "active")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, status, kind) VALUES ($1, $2, 'active', 'postpartum')`,
		twiceID, practiceID); err != nil {
		t.Fatalf("seed second engagement: %v", err)
	}
	absorbedID := testdb.SeedNamedClient(t, db, practiceID, "Eve", "count-merged@example.com")
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE clients SET merged_into = $2, merged_at = now() WHERE id = $1`, absorbedID, activeID); err != nil {
		t.Fatalf("merge: %v", err)
	}
	return countFixture{practiceID: practiceID, activeEngagementID: activeEngagementID, twiceEngagementID: twiceEngagementID}
}

func getCount(t *testing.T, session, url string) client.CountResponse {
	t.Helper()
	resp := authedGet(t, session, url)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got client.CountResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return got
}

// TestCountHandler_CountsTheRecordsAndTheClientsInActiveCare is #2095:
// the Overview's Clients card reads "14 active" without loading a page of
// Client rows to count them. A merged record is a pointer, not a Client,
// so it counts in neither number; a Client with two active Engagements is
// one Client in care.
func TestCountHandler_CountsTheRecordsAndTheClientsInActiveCare(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-counting-clients"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	seedCountFixture(t, db, practiceID)
	srv, session := newServer(t, db, uid)
	defer srv.Close()

	got := getCount(t, session, srv.URL+"/api/practices/"+practiceID+"/clients/count")
	if want := (client.CountResponse{Records: 4, Active: 2}); got != want {
		t.Errorf("count = %+v, want %+v", got, want)
	}
}

// TestCountHandler_ContractorCountsOnlyTheClientsSheIsAttachedTo holds
// the Clients list's own narrowing (ADR-0008's contractor column): a
// contractor Doula is not told how many Clients the Practice holds, only
// how many she is attached to through an open, granted attachment.
func TestCountHandler_ContractorCountsOnlyTheClientsSheIsAttachedTo(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "owner-for-counting-contractor", []string{ownerRole}, "employee")
	fixture := seedCountFixture(t, db, practiceID)
	const contractorUID = "contractor-counting-clients"
	contractorID := testdb.SeedContractorAtPractice(t, db, practiceID, contractorUID)
	testdb.SeedAttachment(t, db, fixture.activeEngagementID, contractorID, "granted", false)
	testdb.SeedAttachment(t, db, fixture.twiceEngagementID, contractorID, "granted", true)
	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	got := getCount(t, session, srv.URL+"/api/practices/"+practiceID+"/clients/count")
	if want := (client.CountResponse{Records: 1, Active: 1}); got != want {
		t.Errorf("count = %+v, want %+v", got, want)
	}
}
