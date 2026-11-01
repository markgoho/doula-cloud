package main

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/testdb"
)

var (
	dormantSignedUp = time.Date(2023, 5, 1, 14, 0, 0, 0, time.UTC)
	dormantLastSeen = time.Date(2024, 10, 1, 16, 0, 0, 0, time.UTC)
)

// seedHeldCredits gives a Practice a signup bonus of n Credits, signed
// up at signedUp.
func (s *opsStack) seedHeldCredits(practiceID string, n int, signedUp time.Time) {
	s.t.Helper()
	s.exec(`UPDATE practices SET created_at = $2 WHERE id = $1`, practiceID, signedUp)
	s.exec(`INSERT INTO credit_ledger (practice_id, origin, quantity, created_at) VALUES ($1, 'signup_bonus', $2, $3)`,
		practiceID, n, signedUp)
}

// seedLastSeen seeds a Staff member of the Practice last seen at seen.
func (s *opsStack) seedLastSeen(practiceID, uid string, seen time.Time) {
	s.t.Helper()
	staffID := testdb.SeedStaffAtPractice(s.t, s.db, practiceID, uid, []string{ownerRole}, "employee")
	s.exec(`UPDATE staff SET last_active_at = $2 WHERE id = $1`, staffID, seen)
}

func TestDormantList_ShowsAPracticeHoldingCreditsNobodyHasSeenInTwoYears(t *testing.T) {
	s := &listStack{opsStack: newOpsStack(t, false)}
	dormant := testdb.SeedPractice(t, s.db, "Quiet Hills Doulas")
	s.seedHeldCredits(dormant, 4, dormantSignedUp)
	s.seedLastSeen(dormant, "dormant-old", dormantLastSeen)
	s.seedLastSeen(dormant, "dormant-older", dormantLastSeen.AddDate(-1, 0, 0))

	page := decodeList(t, s.list("/api/practices/dormant", s.signedInCookie()))

	if len(page.Items) != 1 || page.HasMore || page.NextCursor != nil {
		t.Fatalf("page = %+v, want one Practice and no more", page)
	}
	got, _ := json.Marshal(page.Items[0])
	want := compactJSON(t, `{
		"id": "`+dormant+`",
		"name": "Quiet Hills Doulas",
		"signedUpAt": "2023-05-01T14:00:00Z",
		"balance": 4,
		"lastContactAt": "2024-10-01T16:00:00Z"
	}`)
	if compactJSON(t, string(got)) != want {
		t.Errorf("item =\n%s\nwant\n%s", got, want)
	}
}

func TestDormantList_CountsAPracticeNobodyHasEverSeen(t *testing.T) {
	s := &listStack{opsStack: newOpsStack(t, false)}
	never := testdb.SeedPractice(t, s.db, "Never Seen")
	s.seedHeldCredits(never, 1, dormantSignedUp)
	testdb.SeedStaffAtPractice(t, s.db, never, "never-owner", []string{ownerRole}, "employee")

	page := decodeList(t, s.list("/api/practices/dormant", s.signedInCookie()))

	if len(page.Items) != 1 || string(page.Items[0]["lastContactAt"]) != "null" {
		t.Fatalf("items = %+v, want one Practice with no recorded contact", page.Items)
	}
}

func TestDormantList_LeavesOutAPracticeSeenLatelyOrHoldingNoCredits(t *testing.T) {
	s := &listStack{opsStack: newOpsStack(t, false)}
	seen := testdb.SeedPractice(t, s.db, "Seen Lately")
	s.seedHeldCredits(seen, 4, dormantSignedUp)
	s.seedLastSeen(seen, "seen-old", dormantLastSeen)
	s.seedLastSeen(seen, "seen-lately", opsStart.AddDate(-2, 0, 1))
	spent := testdb.SeedPractice(t, s.db, "No Credits")
	s.exec(`UPDATE practices SET created_at = $2 WHERE id = $1`, spent, dormantSignedUp)
	s.seedLastSeen(spent, "none-old", dormantLastSeen)

	page := decodeList(t, s.list("/api/practices/dormant", s.signedInCookie()))

	if len(page.Items) != 0 {
		t.Fatalf("items = %+v, want none", page.Items)
	}
}

func TestDormantList_TwoYearsAreCountedFromMidnightInNewYork(t *testing.T) {
	s := &listStack{opsStack: newOpsStack(t, false)}
	midnight := time.Date(2024, 11, 2, 0, 0, 0, 0, newYork)
	atMidnight := testdb.SeedPractice(t, s.db, "At Midnight")
	s.seedHeldCredits(atMidnight, 1, dormantSignedUp)
	s.seedLastSeen(atMidnight, "at-midnight", midnight)
	afterMidnight := testdb.SeedPractice(t, s.db, "After Midnight")
	s.seedHeldCredits(afterMidnight, 1, dormantSignedUp)
	s.seedLastSeen(afterMidnight, "after-midnight", midnight.Add(time.Minute))

	page := decodeList(t, s.list("/api/practices/dormant", s.signedInCookie()))

	if len(page.Items) != 1 || string(page.Items[0]["id"]) != `"`+atMidnight+`"` {
		t.Fatalf("items = %+v, want only the Practice last seen at midnight two years ago", page.Items)
	}
}

func TestDormantList_ShowsNobodysContactSoWritesNoReadRow(t *testing.T) {
	s := &listStack{opsStack: newOpsStack(t, false)}
	dormant := testdb.SeedPractice(t, s.db, "Quiet")
	s.seedHeldCredits(dormant, 2, dormantSignedUp)
	cookie := s.signedInCookie()

	page := decodeList(t, s.list("/api/practices/dormant", cookie))

	if len(page.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(page.Items))
	}
	if n := s.count(`SELECT count(*) FROM operator_acts WHERE action = 'owner_contact_read'`); n != 0 {
		t.Errorf("read rows = %d, want none", n)
	}
}

func TestDormantList_PagesNewestSignupFirstByCursor(t *testing.T) {
	s := &listStack{opsStack: newOpsStack(t, false)}
	cookie := s.signedInCookie()
	for i := range practicePageSize + 1 {
		id := testdb.SeedPractice(t, s.db, "Practice")
		s.seedHeldCredits(id, 1, dormantSignedUp.Add(time.Duration(i)*time.Hour))
	}

	first := decodeList(t, s.list("/api/practices/dormant", cookie))
	if len(first.Items) != practicePageSize || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page: %d items, hasMore %v, cursor %v", len(first.Items), first.HasMore, first.NextCursor)
	}
	second := decodeList(t, s.list("/api/practices/dormant?cursor="+*first.NextCursor, cookie))
	if len(second.Items) != 1 || second.HasMore {
		t.Fatalf("second page: %d items, hasMore %v", len(second.Items), second.HasMore)
	}
	if got := string(second.Items[0]["signedUpAt"]); got != `"2023-05-01T14:00:00Z"` {
		t.Errorf("last Practice signed up %s, want the oldest", got)
	}
}

func TestDormantList_RefusesAMalformedCursor(t *testing.T) {
	s := &listStack{opsStack: newOpsStack(t, false)}

	if w := s.list("/api/practices/dormant?cursor=not-a-cursor", s.signedInCookie()); w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body %s", w.Code, w.Body)
	}
}

func TestDormantList_RefusedWithoutASession(t *testing.T) {
	s := &listStack{opsStack: newOpsStack(t, false)}

	if w := s.list("/api/practices/dormant", ""); w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}
