package staffauth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// newWorkStateHistoryServer mounts this package's whole surface through
// staffauth.Mount, the same call main.go makes on the real GatedRouter --
// because the Owner/Admin-vs-Doula boundary lives at that mount and not
// inside the handler (#315).
func newWorkStateHistoryServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	staffauth.Mount(g, ir, db.App, authntest.Verifier{}, authntest.NewFakeAccountManager(), tasknudge.NoOpEnqueuer{})
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func getWorkStateHistory(t *testing.T, srv *httptest.Server, session, practiceID, staffID, query string) *http.Response {
	t.Helper()
	url := srv.URL + "/api/practices/" + practiceID + "/staff/" + staffID + "/work-state-history" + query
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func decodeWorkStateHistory(t *testing.T, resp *http.Response) staffauth.WorkStateHistory {
	t.Helper()
	var out staffauth.WorkStateHistory
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

// seedWorkStateEvent writes one row of the audit trail directly, so a
// test can lay out a history spanning years without driving the
// self-edit endpoint once per entry. previous is the empty string for
// the first assertion, which is stored as NULL.
func seedWorkStateEvent(t *testing.T, db *testdb.DB, staffID, previous, next string, at time.Time) {
	t.Helper()
	var prev any
	if previous != "" {
		prev = previous
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO staff_work_state_events (staff_id, previous_work_state, work_state, actor_staff_id, created_at)
		 VALUES ($1, $2, $3, $1, $4)`,
		staffID, prev, next, at,
	); err != nil {
		t.Fatalf("seed work state event: %v", err)
	}
}

// TestListWorkStateHistory_DoulaForbidden holds #459's reach criterion at
// the boundary that enforces it: the history reads through the same gate
// as the roster it hangs off, and a Doula has neither.
func TestListWorkStateHistory_DoulaForbidden(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-reading-work-state-history"
	staffID, practiceID := seedStaffWithMembership(t, db, identityUID) // '{doula}'

	srv, session := newWorkStateHistoryServer(t, db, identityUID)
	defer srv.Close()

	resp := getWorkStateHistory(t, srv, session, practiceID, staffID, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestListWorkStateHistory_FirstAssertionAndChange is the ticket's core
// case: a first assertion carries no previous value and a move carries
// both sides, so the screen can print two different sentences (#459's
// "a first assertion is not printed as a change").
func TestListWorkStateHistory_FirstAssertionAndChange(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-reads-work-state-history"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)

	doulaID := seedStaff(t, db, "doula-who-moved")
	seedMembership(t, db, practiceID, doulaID)

	joined := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	moved := time.Date(2027, 3, 14, 9, 30, 0, 0, time.UTC)
	seedWorkStateEvent(t, db, doulaID, "", "NY", joined)
	seedWorkStateEvent(t, db, doulaID, "NY", "NJ", moved)

	srv, session := newWorkStateHistoryServer(t, db, ownerUID)
	defer srv.Close()

	resp := getWorkStateHistory(t, srv, session, practiceID, doulaID, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	history := decodeWorkStateHistory(t, resp)
	if len(history.Items) != 2 {
		t.Fatalf("items = %+v, want 2", history.Items)
	}
	// Newest first.
	if history.Items[0].PreviousWorkState != "NY" || history.Items[0].WorkState != "NJ" {
		t.Errorf("newest item = %+v, want NY -> NJ", history.Items[0])
	}
	if history.Items[1].PreviousWorkState != "" {
		t.Errorf("first assertion previousWorkState = %q, want empty", history.Items[1].PreviousWorkState)
	}
	if history.Items[1].WorkState != "NY" {
		t.Errorf("first assertion workState = %q, want NY", history.Items[1].WorkState)
	}
	if history.HasMore {
		t.Error("hasMore = true, want false for a two-entry history")
	}
	if history.NextCursor != nil {
		t.Errorf("nextCursor = %v, want nil", history.NextCursor)
	}
}

// TestListWorkStateHistory_ReAssertionIsNotAChange pins the changes-only
// filter. Saving an unchanged value is a real act (#437) and its row
// stays in the table, but the roster already shows it as the date beside
// the current value, so the history prints the moves and not the
// repetitions.
func TestListWorkStateHistory_ReAssertionIsNotAChange(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-reads-reassertions"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)

	doulaID := seedStaff(t, db, "doula-who-reasserted")
	seedMembership(t, db, practiceID, doulaID)

	seedWorkStateEvent(t, db, doulaID, "", "NY", time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC))
	seedWorkStateEvent(t, db, doulaID, "NY", "NY", time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC))
	seedWorkStateEvent(t, db, doulaID, "NY", "NY", time.Date(2027, 1, 5, 12, 0, 0, 0, time.UTC))

	srv, session := newWorkStateHistoryServer(t, db, ownerUID)
	defer srv.Close()

	resp := getWorkStateHistory(t, srv, session, practiceID, doulaID, "")
	defer resp.Body.Close()

	history := decodeWorkStateHistory(t, resp)
	if len(history.Items) != 1 {
		t.Fatalf("items = %+v, want only the first assertion", history.Items)
	}
	if history.Items[0].WorkState != "NY" || history.Items[0].PreviousWorkState != "" {
		t.Errorf("item = %+v, want the NULL-previous first assertion", history.Items[0])
	}
}

// TestListWorkStateHistory_MemberSinceDatesTheMembership is #459's
// inherited-value criterion. A contractor doula who asserted her work
// state at an earlier Practice carries that row into this one -- 00043's
// table has no practice_id and joining a second Practice writes no event
// -- so the response has to date the Membership, which is what lets the
// screen say the assertion was made before she joined.
func TestListWorkStateHistory_MemberSinceDatesTheMembership(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-reads-inherited-history"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)

	doulaID := seedStaff(t, db, "contractor-on-two-rosters")
	seedMembership(t, db, practiceID, doulaID)

	assertedElsewhere := time.Date(2025, 5, 4, 12, 0, 0, 0, time.UTC)
	seedWorkStateEvent(t, db, doulaID, "", "NY", assertedElsewhere)

	srv, session := newWorkStateHistoryServer(t, db, ownerUID)
	defer srv.Close()

	resp := getWorkStateHistory(t, srv, session, practiceID, doulaID, "")
	defer resp.Body.Close()

	history := decodeWorkStateHistory(t, resp)
	if history.MemberSince.IsZero() {
		t.Fatal("memberSince is zero, want the Membership's created_at")
	}
	if !history.MemberSince.After(assertedElsewhere) {
		t.Errorf("memberSince = %v, want later than the assertion at %v", history.MemberSince, assertedElsewhere)
	}
}

// TestListWorkStateHistory_PaginatesOldestPagesLast walks past the page
// size, which is the AC about a Practice whose people have corrected
// their work state many times: the screen asks for one page and the rest
// stays reachable through the cursor rather than being cut off.
func TestListWorkStateHistory_PaginatesOldestPagesLast(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-pages-work-state-history"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)

	doulaID := seedStaff(t, db, "doula-who-moved-often")
	seedMembership(t, db, practiceID, doulaID)

	// 25 alternating moves, so every row survives the changes-only
	// filter and the page boundary falls inside them.
	const total = 25
	states := [2]string{"NY", "NJ"}
	seedWorkStateEvent(t, db, doulaID, "", states[0], time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	for i := 1; i < total; i++ {
		seedWorkStateEvent(t, db, doulaID, states[(i-1)%2], states[i%2],
			time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, i, 0))
	}

	srv, session := newWorkStateHistoryServer(t, db, ownerUID)
	defer srv.Close()

	first := getWorkStateHistory(t, srv, session, practiceID, doulaID, "")
	defer first.Body.Close()
	page1 := decodeWorkStateHistory(t, first)
	if len(page1.Items) != 20 {
		t.Fatalf("first page items = %d, want 20", len(page1.Items))
	}
	if !page1.HasMore || page1.NextCursor == nil {
		t.Fatalf("first page hasMore = %v, nextCursor = %v, want more", page1.HasMore, page1.NextCursor)
	}

	second := getWorkStateHistory(t, srv, session, practiceID, doulaID, "?cursor="+*page1.NextCursor)
	defer second.Body.Close()
	page2 := decodeWorkStateHistory(t, second)
	if len(page2.Items) != total-20 {
		t.Fatalf("second page items = %d, want %d", len(page2.Items), total-20)
	}
	if page2.HasMore {
		t.Error("second page hasMore = true, want false")
	}
	if page2.Items[0].CreatedAt.After(page1.Items[19].CreatedAt) {
		t.Error("second page starts newer than the first page ended")
	}
}

func TestListWorkStateHistory_RejectsBadCursor(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-sends-bad-cursor"
	ownerID, practiceID := seedOwnerMembership(t, db, ownerUID)

	srv, session := newWorkStateHistoryServer(t, db, ownerUID)
	defer srv.Close()

	resp := getWorkStateHistory(t, srv, session, practiceID, ownerID, "?cursor=not-a-cursor")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestListWorkStateHistory_RejectsBadStaffID(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-sends-bad-staff-id"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)

	srv, session := newWorkStateHistoryServer(t, db, ownerUID)
	defer srv.Close()

	resp := getWorkStateHistory(t, srv, session, practiceID, "not-a-uuid", "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestListWorkStateHistory_StrangerIsNotFound proves the Membership read
// doubles as the existence check: a real Staff member at somebody else's
// Practice is a 404, not an empty history that would confirm she exists.
func TestListWorkStateHistory_StrangerIsNotFound(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-reads-a-stranger"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)

	otherPracticeID := seedPractice(t, db, "Someone Else's Practice")
	strangerID := seedStaff(t, db, "stranger-elsewhere")
	seedMembership(t, db, otherPracticeID, strangerID)
	seedWorkStateEvent(t, db, strangerID, "", "CA", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	srv, session := newWorkStateHistoryServer(t, db, ownerUID)
	defer srv.Close()

	resp := getWorkStateHistory(t, srv, session, practiceID, strangerID, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}
