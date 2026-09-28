package staffauth_test

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
	"doula-cloud/api/internal/feedback"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// postFeedback posts body to POST /api/staff/feedback with session's
// __session cookie, the same request shape putWorkState builds for the
// neighboring pre-Practice route.
func postFeedback(t *testing.T, srv *httptest.Server, session, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/staff/feedback", strings.NewReader(body))
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

// readFeedbackRow reads the one column set every test here checks, as
// the superuser Admin connection -- feedback carries no SELECT grant for
// app_runtime at all (00118), so this is the only way a test can look at
// what was written.
func readFeedbackRow(t *testing.T, db *testdb.DB, id string) (kind, text, browser string, practiceID, roles sql.NullString) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT kind::text, text, browser, practice_id::text, array_to_string(roles, ',')
		   FROM feedback WHERE id = $1`,
		id,
	).Scan(&kind, &text, &browser, &practiceID, &roles); err != nil {
		t.Fatalf("read feedback row %s: %v", id, err)
	}
	return kind, text, browser, practiceID, roles
}

func TestFeedback_MissingCookie(t *testing.T) {
	db := testdb.New(t)
	srv, _ := newWorkStateServer(t, db, "feedback-no-cookie")
	defer srv.Close()

	resp := postFeedback(t, srv, "", `{"kind":"idea_or_request"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestFeedback_MalformedBody(t *testing.T) {
	db := testdb.New(t)
	testdb.SeedStaff(t, db, "feedback-malformed-body")
	srv, session := newWorkStateServer(t, db, "feedback-malformed-body")
	defer srv.Close()

	resp := postFeedback(t, srv, session, `not json`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestFeedback_RejectsMissingKind(t *testing.T) {
	db := testdb.New(t)
	testdb.SeedStaff(t, db, "feedback-missing-kind")
	srv, session := newWorkStateServer(t, db, "feedback-missing-kind")
	defer srv.Close()

	resp := postFeedback(t, srv, session, `{"text":"something"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	out := apierrtest.Decode(t, resp)
	if out.Details["kind"] != staffauth.MsgFeedbackKindNeeded {
		t.Fatalf("details[kind] = %q, want %q", out.Details["kind"], staffauth.MsgFeedbackKindNeeded)
	}
}

func TestFeedback_RejectsUnknownKind(t *testing.T) {
	db := testdb.New(t)
	testdb.SeedStaff(t, db, "feedback-unknown-kind")
	srv, session := newWorkStateServer(t, db, "feedback-unknown-kind")
	defer srv.Close()

	resp := postFeedback(t, srv, session, `{"kind":"a_kind_that_does_not_exist"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestFeedback_RejectsTextOverFiveThousandCharacters(t *testing.T) {
	db := testdb.New(t)
	testdb.SeedStaff(t, db, "feedback-text-too-long")
	srv, session := newWorkStateServer(t, db, "feedback-text-too-long")
	defer srv.Close()

	overLimit := strings.Repeat("a", feedback.MaxTextRunes+1)
	resp := postFeedback(t, srv, session, `{"kind":"idea_or_request","text":"`+overLimit+`"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	out := apierrtest.Decode(t, resp)
	if out.Details["text"] != staffauth.MsgFeedbackTextTooLong {
		t.Fatalf("details[text] = %q, want %q", out.Details["text"], staffauth.MsgFeedbackTextTooLong)
	}
}

func TestFeedback_UnknownStaff(t *testing.T) {
	db := testdb.New(t)
	srv, session := newWorkStateServer(t, db, "feedback-session-no-staff-row")
	defer srv.Close()

	resp := postFeedback(t, srv, session, `{"kind":"idea_or_request"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestFeedback_EmptyTextFromAccountIsAllowed is #1523's own AC ("Empty
// free text is allowed") plus the /account shape: no practiceId in the
// body leaves practice_id and roles both NULL on the stored row. It also
// proves the browser is parsed from User-Agent and the raw header itself
// never asked for.
func TestFeedback_EmptyTextFromAccountIsAllowed(t *testing.T) {
	db := testdb.New(t)
	testdb.SeedStaff(t, db, "feedback-empty-text")
	srv, session := newWorkStateServer(t, db, "feedback-empty-text")
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/staff/feedback",
		strings.NewReader(`{"kind":"something_else","text":"","page":{"url":"/account","route":{"id":"/account"}},"appBuild":"abc1234","screenWidth":390}`))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.6 Safari/605.1.15")
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var out staffauth.FeedbackResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.ID == "" {
		t.Fatal("response id is empty")
	}

	kind, text, browser, practiceID, roles := readFeedbackRow(t, db, out.ID)
	if kind != "something_else" {
		t.Errorf("kind = %q, want %q", kind, "something_else")
	}
	if text != "" {
		t.Errorf("text = %q, want empty", text)
	}
	if browser != "Safari 17" {
		t.Errorf("browser = %q, want %q -- family and major version only, never the raw header", browser, "Safari 17")
	}
	if practiceID.Valid {
		t.Errorf("practice_id = %q, want NULL -- sent from /account", practiceID.String)
	}
	if roles.Valid {
		t.Errorf("roles = %q, want NULL -- sent from /account", roles.String)
	}
}

func TestFeedback_RejectsMalformedPracticeID(t *testing.T) {
	db := testdb.New(t)
	testdb.SeedStaff(t, db, "feedback-bad-practice-id")
	srv, session := newWorkStateServer(t, db, "feedback-bad-practice-id")
	defer srv.Close()

	resp := postFeedback(t, srv, session, `{"kind":"idea_or_request","practiceId":"not-a-uuid"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestFeedback_RefusesAPracticeSheDoesNotHold is #1523's own AC: a
// practiceId is checked against the sender's membership, never trusted.
func TestFeedback_RefusesAPracticeSheDoesNotHold(t *testing.T) {
	db := testdb.New(t)
	testdb.SeedStaff(t, db, "feedback-outsider")
	otherPracticeID := testdb.SeedPractice(t, db, "A Different Practice")
	srv, session := newWorkStateServer(t, db, "feedback-outsider")
	defer srv.Close()

	resp := postFeedback(t, srv, session, `{"kind":"idea_or_request","practiceId":"`+otherPracticeID+`"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
	var count int
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM feedback WHERE practice_id = $1`, otherPracticeID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("feedback rows for a Practice she does not hold = %d, want 0", count)
	}
}

// TestFeedback_RecordsPracticeAndHerOwnRoles proves the roles rider: sent
// from under practices/[practiceId], the row carries that Practice and
// the caller's real roles read off practice_memberships -- never a role
// the request itself might have claimed.
func TestFeedback_RecordsPracticeAndHerOwnRoles(t *testing.T) {
	db := testdb.New(t)
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, "feedback-owner-admin", []string{ownerRole, adminRole}, employeeType)
	_ = staffID
	srv, session := newWorkStateServer(t, db, "feedback-owner-admin")
	defer srv.Close()

	resp := postFeedback(t, srv, session, `{"kind":"not_working","practiceId":"`+practiceID+`"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var out staffauth.FeedbackResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}

	_, _, _, gotPracticeID, roles := readFeedbackRow(t, db, out.ID)
	if !gotPracticeID.Valid || gotPracticeID.String != practiceID {
		t.Errorf("practice_id = %v, want %q", gotPracticeID, practiceID)
	}
	if !roles.Valid || roles.String != "owner,admin" {
		t.Errorf("roles = %v, want %q", roles, "owner,admin")
	}
}

// TestFeedback_ZeroRoleMembershipStillRecordsAnEmptyArray proves the
// distinction staffRolesAt draws: a real membership with no roles
// assigned yet stores practice_id with roles as an empty array, not
// NULL -- NULL is reserved for "no {practiceId} in the request at all".
func TestFeedback_ZeroRoleMembershipStillRecordsAnEmptyArray(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Zero Role Practice")
	staffID := testdb.SeedStaff(t, db, "feedback-zero-role")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type) VALUES ($1, $2, '{}', 'employee')`,
		practiceID, staffID,
	); err != nil {
		t.Fatalf("seed zero-role membership: %v", err)
	}
	srv, session := newWorkStateServer(t, db, "feedback-zero-role")
	defer srv.Close()

	resp := postFeedback(t, srv, session, `{"kind":"not_working","practiceId":"`+practiceID+`"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var out staffauth.FeedbackResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}

	_, _, _, gotPracticeID, roles := readFeedbackRow(t, db, out.ID)
	if !gotPracticeID.Valid {
		t.Fatal("practice_id is NULL, want set -- a zero-role membership is still a membership")
	}
	if !roles.Valid || roles.String != "" {
		t.Errorf("roles = %v, want a valid empty array, not NULL", roles)
	}
}

// TestFeedback_RefusesTwentyFirstRequestInAnHour is #1523's own sizing:
// 20 per hour, per sender.
func TestFeedback_RefusesTwentyFirstRequestInAnHour(t *testing.T) {
	db := testdb.New(t)
	testdb.SeedStaff(t, db, "feedback-burst")
	srv, session := newWorkStateServer(t, db, "feedback-burst")
	defer srv.Close()

	const budget = 20
	for i := 1; i <= budget; i++ {
		resp := postFeedback(t, srv, session, `{"kind":"something_else"}`)
		status := resp.StatusCode
		_ = resp.Body.Close()
		if status != http.StatusCreated {
			t.Fatalf("request %d of %d: status = %d, want %d", i, budget, status, http.StatusCreated)
		}
	}

	resp := postFeedback(t, srv, session, `{"kind":"something_else"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("request %d: status = %d, want %d", budget+1, resp.StatusCode, http.StatusTooManyRequests)
	}
	if got := apierrtest.Decode(t, resp).Code; got != apierr.CodeRateLimited {
		t.Errorf("code = %q, want %q", got, apierr.CodeRateLimited)
	}
}
