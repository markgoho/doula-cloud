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

// recommendationServer mounts this package's whole surface the way
// main.go does, and seeds a session whose second-factor fact is given.
type recommendationServer struct {
	t   *testing.T
	db  *testdb.DB
	srv *httptest.Server
}

func newRecommendationServer(t *testing.T, db *testdb.DB) *recommendationServer {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	staffauth.Mount(g, ir, db.App, authntest.Verifier{}, authntest.NewFakeAccountManager(), tasknudge.NoOpEnqueuer{}, neverSuppressed)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &recommendationServer{t: t, db: db, srv: srv}
}

func (s *recommendationServer) session(uid string, secondFactor bool) string {
	s.t.Helper()
	return authntest.SeedSessionWithSecondFactor(s.t, s.db.App, uid, secondFactor)
}

func (s *recommendationServer) do(method, session, practiceID, suffix string) *http.Response {
	s.t.Helper()
	req, err := http.NewRequestWithContext(s.t.Context(), method,
		s.srv.URL+"/api/practices/"+practiceID+"/second-factor-recommendation"+suffix, nil)
	if err != nil {
		s.t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.t.Fatalf("request: %v", err)
	}
	return resp
}

// recommended GETs the recommendation and returns its one fact.
func (s *recommendationServer) recommended(session, practiceID string) bool {
	s.t.Helper()
	resp := s.do(http.MethodGet, session, practiceID, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		s.t.Fatalf("GET status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out staffauth.SecondFactorRecommendationResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		s.t.Fatalf("decode: %v", err)
	}
	return out.Recommended
}

func (s *recommendationServer) close(session, practiceID string) {
	s.t.Helper()
	resp := s.do(http.MethodPost, session, practiceID, "/close")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		s.t.Fatalf("POST close status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func seedInvitationBy(t *testing.T, db *testdb.DB, practiceID, staffID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_invitations (practice_id, address, roles, employment_type, token_digest, invited_by, expires_at)
		 VALUES ($1, 'invited@example.com', ARRAY['doula']::practice_role[], 'employee', 'digest', $2, now() + interval '7 days')`,
		practiceID, staffID,
	); err != nil {
		t.Fatalf("seed invitation: %v", err)
	}
}

func seedStripeConnected(t *testing.T, db *testdb.DB, practiceID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET stripe_connect_account_id = 'acct_recommendation' WHERE id = $1`, practiceID,
	); err != nil {
		t.Fatalf("seed Stripe account: %v", err)
	}
}

// The empty Practice asks for one act and nothing else (ADR-0048), so
// an Owner with no second factor is not told anything there -- even
// after an Invitation or Stripe, which show their notice only once First
// Value has come.
func TestSecondFactorRecommendation_NotBeforeFirstValue(t *testing.T) {
	db := testdb.New(t)
	const uid = "recommendation-empty"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole, adminRole, doulaRole}, employeeType)
	s := newRecommendationServer(t, db)
	session := s.session(uid, false)

	if s.recommended(session, practiceID) {
		t.Fatal("recommended on the empty Practice, want not")
	}

	seedInvitationBy(t, db, practiceID, staffID)
	seedStripeConnected(t, db, practiceID)
	if s.recommended(session, practiceID) {
		t.Fatal("recommended after an Invitation and Stripe but before First Value, want not")
	}

	testdb.SeedEngagement(t, db, practiceID)
	if !s.recommended(session, practiceID) {
		t.Fatal("not recommended after First Value, want recommended")
	}

	// One close covers every act already reached: the two that came
	// before First Value do not each bring it back.
	s.close(session, practiceID)
	if s.recommended(session, practiceID) {
		t.Fatal("recommended after a close with no new act, want not")
	}
}

// She can close it, and it shows again once at each new act on the list.
func TestSecondFactorRecommendation_ReturnsOnceAtEachNewAct(t *testing.T) {
	db := testdb.New(t)
	const uid = "recommendation-acts"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)
	s := newRecommendationServer(t, db)
	session := s.session(uid, false)

	testdb.SeedEngagement(t, db, practiceID)
	if !s.recommended(session, practiceID) {
		t.Fatal("not recommended after First Value, want recommended")
	}
	s.close(session, practiceID)
	if s.recommended(session, practiceID) {
		t.Fatal("recommended after close, want not")
	}

	// A second Engagement is not a new act on the list.
	testdb.SeedNamedEngagement(t, db, practiceID, "Second", "second@example.com")
	if s.recommended(session, practiceID) {
		t.Fatal("recommended after a second Engagement, want not")
	}

	seedInvitationBy(t, db, practiceID, staffID)
	if !s.recommended(session, practiceID) {
		t.Fatal("not recommended after her first Invitation, want recommended")
	}
	s.close(session, practiceID)
	if s.recommended(session, practiceID) {
		t.Fatal("recommended after the second close, want not")
	}

	seedStripeConnected(t, db, practiceID)
	if !s.recommended(session, practiceID) {
		t.Fatal("not recommended after Stripe connected, want recommended")
	}
	s.close(session, practiceID)
	if s.recommended(session, practiceID) {
		t.Fatal("recommended after the third close, want not")
	}

	// A repeated close is a no-op, not a second row.
	s.close(session, practiceID)
	var rows int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM second_factor_recommendation_closures WHERE practice_id = $1`, practiceID,
	).Scan(&rows); err != nil {
		t.Fatalf("count closures: %v", err)
	}
	if rows != 3 {
		t.Fatalf("closure rows = %d, want 3 (one per act)", rows)
	}
}

// The close is kept server-side, so a new session -- another device, or
// tomorrow -- does not see it again; and each close records who closed
// it, for which act, and when.
func TestSecondFactorRecommendation_CloseIsKeptAcrossSessionsAndRecorded(t *testing.T) {
	db := testdb.New(t)
	const uid = "recommendation-devices"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)
	s := newRecommendationServer(t, db)
	testdb.SeedEngagement(t, db, practiceID)

	before := time.Now().Add(-time.Minute)
	s.close(s.session(uid, false), practiceID)

	if s.recommended(s.session(uid, false), practiceID) {
		t.Fatal("recommended on a second session after a close, want not")
	}

	var act string
	var closedAt time.Time
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT act, closed_at FROM second_factor_recommendation_closures WHERE practice_id = $1 AND staff_id = $2`,
		practiceID, staffID,
	).Scan(&act, &closedAt); err != nil {
		t.Fatalf("read closure: %v", err)
	}
	if act != "first_value" {
		t.Fatalf("act = %q, want first_value", act)
	}
	if closedAt.Before(before) {
		t.Fatalf("closed_at = %v, want at or after %v", closedAt, before)
	}
}

// An Owner who has a second factor never sees it.
func TestSecondFactorRecommendation_NeverWithASecondFactor(t *testing.T) {
	db := testdb.New(t)
	const uid = "recommendation-enrolled"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)
	s := newRecommendationServer(t, db)
	session := s.session(uid, true)

	testdb.SeedEngagement(t, db, practiceID)
	seedInvitationBy(t, db, practiceID, staffID)
	seedStripeConnected(t, db, practiceID)
	if s.recommended(session, practiceID) {
		t.Fatal("recommended to an Owner with a second factor, want never")
	}
}

// Her first Invitation is an act of hers: another Owner's Invitation
// does not bring her notice back, and her close does not close the
// other Owner's.
func TestSecondFactorRecommendation_IsPerOwner(t *testing.T) {
	db := testdb.New(t)
	const firstUID, secondUID = "recommendation-owner-one", "recommendation-owner-two"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, firstUID, []string{ownerRole}, employeeType)
	secondID := testdb.SeedStaffAtPractice(t, db, practiceID, secondUID, []string{ownerRole}, employeeType)
	s := newRecommendationServer(t, db)
	first, second := s.session(firstUID, false), s.session(secondUID, false)

	testdb.SeedEngagement(t, db, practiceID)
	s.close(first, practiceID)
	if !s.recommended(second, practiceID) {
		t.Fatal("the second Owner's notice closed by the first Owner's close, want shown")
	}

	seedInvitationBy(t, db, practiceID, secondID)
	if s.recommended(first, practiceID) {
		t.Fatal("recommended to the first Owner after the second Owner's Invitation, want not")
	}
}

// Only an Owner sees it: the route refuses an Admin and a Doula.
func TestSecondFactorRecommendation_OwnerOnly(t *testing.T) {
	for _, roles := range [][]string{{adminRole}, {doulaRole}} {
		t.Run(roles[0], func(t *testing.T) {
			db := testdb.New(t)
			uid := "recommendation-" + roles[0]
			practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, roles, employeeType)
			s := newRecommendationServer(t, db)
			session := s.session(uid, false)
			testdb.SeedEngagement(t, db, practiceID)

			for _, call := range []struct{ method, suffix string }{
				{http.MethodGet, ""}, {http.MethodPost, "/close"},
			} {
				resp := s.do(call.method, session, practiceID, call.suffix)
				_ = resp.Body.Close()
				if resp.StatusCode != http.StatusForbidden {
					t.Fatalf("%s status = %d, want %d", call.method, resp.StatusCode, http.StatusForbidden)
				}
			}
		})
	}
}

// No message is sent: ADR-0048 names the fourteen-day message as the
// one nudge, so neither the read nor the close queues anything in any
// outbox.
func TestSecondFactorRecommendation_SendsNoMessage(t *testing.T) {
	db := testdb.New(t)
	const uid = "recommendation-no-mail"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)
	s := newRecommendationServer(t, db)
	session := s.session(uid, false)
	testdb.SeedEngagement(t, db, practiceID)

	before := outboxRowCount(t, db)
	s.recommended(session, practiceID)
	s.close(session, practiceID)
	if after := outboxRowCount(t, db); after != before {
		t.Fatalf("outbox rows = %d after the read and the close, want %d", after, before)
	}
}

// outboxRowCount sums the rows of every table whose name ends in
// _outbox, so a new outbox cannot escape the check.
func outboxRowCount(t *testing.T, db *testdb.DB) int {
	t.Helper()
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT table_name FROM information_schema.tables
		  WHERE table_schema = 'public' AND table_name LIKE '%\_outbox'`)
	if err != nil {
		t.Fatalf("list outbox tables: %v", err)
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table name: %v", err)
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("list outbox tables: %v", err)
	}
	if len(tables) == 0 {
		t.Fatal("found no outbox tables, want several")
	}
	total := 0
	for _, table := range tables {
		var n int
		if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM `+table).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		total += n
	}
	return total
}
