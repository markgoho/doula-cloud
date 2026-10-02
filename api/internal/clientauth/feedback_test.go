package clientauth_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/feedback"
	"doula-cloud/api/internal/portalaccount"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

func newPortalFeedbackServer(db *testdb.DB) *httptest.Server {
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	clientauth.Mount(g, db.App, tasknudge.NoOpEnqueuer{})
	return httptest.NewServer(mux)
}

func postPortalFeedback(t *testing.T, srv *httptest.Server, session, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/portal/feedback", strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if session != "" {
		authntest.AddSessionCookie(req, session)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// readPortalFeedbackRow reads the columns each test here checks, via the
// superuser Admin connection -- feedback carries no SELECT grant for
// app_runtime at all (00118).
func readPortalFeedbackRow(t *testing.T, db *testdb.DB, id string) (kind, portalAccount string, clientID, practiceID sql.NullString) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT kind::text, portal_account, client_id::text, practice_id::text FROM feedback WHERE id = $1`,
		id,
	).Scan(&kind, &portalAccount, &clientID, &practiceID); err != nil {
		t.Fatalf("read feedback row %s: %v", id, err)
	}
	return kind, portalAccount, clientID, practiceID
}

// seedSignedInPortalAccount mints a fresh Portal Account with no Client
// attached, and a live session for it -- the "any portal screen, no
// Engagement in view" fixture most tests here start from.
func seedSignedInPortalAccount(t *testing.T, db *testdb.DB) (identifier, session string) {
	t.Helper()
	identifier = portalaccount.NewIdentifier()
	testdb.SeedPortalAccount(t, db, identifier, identifier+"@example.com")
	return identifier, authntest.SeedSession(t, db.App, identifier)
}

func TestPortalFeedback_MissingCookie(t *testing.T) {
	db := testdb.New(t)
	srv := newPortalFeedbackServer(db)
	defer srv.Close()

	resp := postPortalFeedback(t, srv, "", `{"kind":"idea_or_request"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

// TestPortalFeedback_SessionWithNoPortalAccountRefused is the case
// isPortalAccount exists for: a portal_-prefixed session naming no
// portal_accounts row (RequestAddressChangeHandler's own doc comment
// gives the general shape).
func TestPortalFeedback_SessionWithNoPortalAccountRefused(t *testing.T) {
	db := testdb.New(t)
	srv := newPortalFeedbackServer(db)
	defer srv.Close()

	session := authntest.SeedSession(t, db.App, portalaccount.Prefix+"no-account-row")
	resp := postPortalFeedback(t, srv, session, `{"kind":"idea_or_request"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestPortalFeedback_MalformedBody(t *testing.T) {
	db := testdb.New(t)
	_, session := seedSignedInPortalAccount(t, db)
	srv := newPortalFeedbackServer(db)
	defer srv.Close()

	resp := postPortalFeedback(t, srv, session, `not json`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestPortalFeedback_RejectsMissingKind(t *testing.T) {
	db := testdb.New(t)
	_, session := seedSignedInPortalAccount(t, db)
	srv := newPortalFeedbackServer(db)
	defer srv.Close()

	resp := postPortalFeedback(t, srv, session, `{"text":"something"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	out := apierrtest.Decode(t, resp)
	if out.Details["kind"] != clientauth.MsgFeedbackKindNeeded {
		t.Fatalf("details[kind] = %q, want %q", out.Details["kind"], clientauth.MsgFeedbackKindNeeded)
	}
}

func TestPortalFeedback_RejectsUnknownKind(t *testing.T) {
	db := testdb.New(t)
	_, session := seedSignedInPortalAccount(t, db)
	srv := newPortalFeedbackServer(db)
	defer srv.Close()

	resp := postPortalFeedback(t, srv, session, `{"kind":"not_a_real_kind"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestPortalFeedback_RejectsTextOverFiveThousandCharacters(t *testing.T) {
	db := testdb.New(t)
	_, session := seedSignedInPortalAccount(t, db)
	srv := newPortalFeedbackServer(db)
	defer srv.Close()

	overLimit := strings.Repeat("a", feedback.MaxTextRunes+1)
	resp := postPortalFeedback(t, srv, session, `{"kind":"idea_or_request","text":"`+overLimit+`"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	out := apierrtest.Decode(t, resp)
	if out.Details["text"] != clientauth.MsgFeedbackTextTooLong {
		t.Fatalf("details[text] = %q, want %q", out.Details["text"], clientauth.MsgFeedbackTextTooLong)
	}
}

// TestPortalFeedback_NoEngagementLeavesClientAndPracticeNull proves the
// "not inside one Engagement" shape: the row still names the Portal
// Account, but client_id and practice_id both stay NULL, and the browser
// is parsed rather than the raw header ever asked for.
func TestPortalFeedback_NoEngagementLeavesClientAndPracticeNull(t *testing.T) {
	db := testdb.New(t)
	identifier, session := seedSignedInPortalAccount(t, db)
	srv := newPortalFeedbackServer(db)
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/portal/feedback",
		strings.NewReader(`{"kind":"something_else","text":"","page":{"url":"/portal","route":{"id":"/portal"}},"appBuild":"abc1234","screenWidth":375}`))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:130.0) Gecko/20100101 Firefox/130.0")
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var out clientauth.PortalFeedbackResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.ID == "" {
		t.Fatal("response id is empty")
	}

	kind, portalAccount, clientID, practiceID := readPortalFeedbackRow(t, db, out.ID)
	if kind != "something_else" {
		t.Errorf("kind = %q, want %q", kind, "something_else")
	}
	if portalAccount != identifier {
		t.Errorf("portal_account = %q, want %q", portalAccount, identifier)
	}
	if clientID.Valid {
		t.Errorf("client_id = %q, want NULL -- no Engagement in view", clientID.String)
	}
	if practiceID.Valid {
		t.Errorf("practice_id = %q, want NULL -- no Engagement in view", practiceID.String)
	}
}

// TestPortalFeedback_QueuesTheIssueOutboxRow is #1524's own AC: both
// send handlers enqueue the outbox row in the same transaction that
// saves the piece of Feedback.
func TestPortalFeedback_QueuesTheIssueOutboxRow(t *testing.T) {
	db := testdb.New(t)
	_, session := seedSignedInPortalAccount(t, db)
	srv := newPortalFeedbackServer(db)
	defer srv.Close()

	resp := postPortalFeedback(t, srv, session, `{"kind":"idea_or_request"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var out clientauth.PortalFeedbackResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var status string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status::text FROM feedback_issue_outbox WHERE feedback_id = $1`, out.ID,
	).Scan(&status); err != nil {
		t.Fatalf("read outbox row: %v", err)
	}
	if status != "pending" {
		t.Errorf("outbox status = %q, want pending", status)
	}
}

func TestPortalFeedback_RejectsMalformedEngagementID(t *testing.T) {
	db := testdb.New(t)
	_, session := seedSignedInPortalAccount(t, db)
	srv := newPortalFeedbackServer(db)
	defer srv.Close()

	resp := postPortalFeedback(t, srv, session, `{"kind":"idea_or_request","engagementId":"not-a-uuid"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPortalFeedback_RefusesAnEngagementSheDoesNotHold is #1523's own AC:
// an engagementId is checked against the signed-in Portal Account, never
// trusted.
func TestPortalFeedback_RefusesAnEngagementSheDoesNotHold(t *testing.T) {
	db := testdb.New(t)
	_, session := seedSignedInPortalAccount(t, db)
	srv := newPortalFeedbackServer(db)
	defer srv.Close()

	practiceID := testdb.SeedPractice(t, db, "Someone Else's Practice")
	_, otherEngagementID := testdb.SeedEngagement(t, db, practiceID)

	resp := postPortalFeedback(t, srv, session, `{"kind":"idea_or_request","engagementId":"`+otherEngagementID+`"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
	var count int
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM feedback WHERE practice_id = $1`, practiceID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("feedback rows for an Engagement she does not hold = %d, want 0", count)
	}
}

// TestPortalFeedback_RecordsClientAndPracticeForHerOwnEngagement proves
// the paired rider: sent from inside one Engagement, the row carries that
// Engagement's own Client and Practice.
func TestPortalFeedback_RecordsClientAndPracticeForHerOwnEngagement(t *testing.T) {
	db := testdb.New(t)
	identifier := portalaccount.NewIdentifier()
	practiceID := testdb.SeedPractice(t, db, "Her Practice")
	clientID, engagementID := testdb.SeedEngagement(t, db, practiceID)
	testdb.SeedPortalUser(t, db, identifier, clientID)
	session := authntest.SeedSession(t, db.App, identifier)
	srv := newPortalFeedbackServer(db)
	defer srv.Close()

	resp := postPortalFeedback(t, srv, session, `{"kind":"not_working","engagementId":"`+engagementID+`"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var out clientauth.PortalFeedbackResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}

	_, _, gotClientID, gotPracticeID := readPortalFeedbackRow(t, db, out.ID)
	if !gotClientID.Valid || gotClientID.String != clientID {
		t.Errorf("client_id = %v, want %q", gotClientID, clientID)
	}
	if !gotPracticeID.Valid || gotPracticeID.String != practiceID {
		t.Errorf("practice_id = %v, want %q", gotPracticeID, practiceID)
	}
}

// TestPortalFeedback_RefusesTwentyFirstRequestInAnHour is #1523's own
// sizing: 20 per hour, per sender.
func TestPortalFeedback_RefusesTwentyFirstRequestInAnHour(t *testing.T) {
	db := testdb.New(t)
	_, session := seedSignedInPortalAccount(t, db)
	srv := newPortalFeedbackServer(db)
	defer srv.Close()

	const budget = 20
	for i := 1; i <= budget; i++ {
		resp := postPortalFeedback(t, srv, session, `{"kind":"something_else"}`)
		status := resp.StatusCode
		_ = resp.Body.Close()
		if status != http.StatusCreated {
			t.Fatalf("request %d of %d: status = %d, want %d", i, budget, status, http.StatusCreated)
		}
	}

	resp := postPortalFeedback(t, srv, session, `{"kind":"something_else"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("request %d: status = %d, want %d", budget+1, resp.StatusCode, http.StatusTooManyRequests)
	}
	if got := apierrtest.Decode(t, resp).Code; got != apierr.CodeRateLimited {
		t.Errorf("code = %q, want %q", got, apierr.CodeRateLimited)
	}
}
