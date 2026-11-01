package main

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"
	"time"

	"doula-cloud/api/internal/testdb"
)

var feedbackSent = time.Date(2026, 9, 20, 14, 30, 0, 0, time.UTC)

type feedbackStack struct {
	*opsStack
	practiceID  string
	staffPiece  string
	clientPiece string
}

// newFeedbackStack seeds a Practice, a piece from its Owner whose issue
// opened, and a piece from one of its Clients whose issue dead-lettered.
func newFeedbackStack(t *testing.T) *feedbackStack {
	t.Helper()
	s := &feedbackStack{opsStack: newOpsStack(t, false)}
	s.practiceID = testdb.SeedPractice(t, s.db, "Rooted Birth Collective")
	owner := testdb.SeedNamedStaffAtPractice(t, s.db, s.practiceID, "fb-owner", "Renata Alvarez", []string{ownerRole, doulaRole}, "employee")
	clientID := testdb.SeedNamedClient(t, s.db, s.practiceID, "Nadia Haddad", "nadia@example.com")
	testdb.SeedPortalAccount(t, s.db, "portal-nadia", "nadia.portal@example.com")

	s.staffPiece = s.insertPiece(`INSERT INTO feedback (id, kind, text, page_url, route_id, app_build, screen_width, browser, staff_id, practice_id, roles, issue_number, sent_at)
	        VALUES (gen_random_uuid(), 'idea_or_request', 'Line one.
Line two.', '/practices/x/invoices', '/practices/[practiceId]/invoices', 'abc1234', 1280, 'Firefox 131', $1, $2, '{owner,doula}', 17, $3) RETURNING id`,
		owner, s.practiceID, feedbackSent)
	s.exec(`INSERT INTO feedback_issue_outbox (feedback_id, status, attempt_count, sent_at) VALUES ($1, 'sent', 1, $2)`, s.staffPiece, feedbackSent)

	s.clientPiece = s.insertPiece(`INSERT INTO feedback (id, kind, text, page_url, route_id, app_build, screen_width, browser, portal_account, practice_id, client_id, sent_at)
	        VALUES (gen_random_uuid(), 'not_working', 'The page froze.', '/portal/x', '/portal/[clientId]', 'abc1234', 320, 'Safari 18', 'portal-nadia', $1, $2, $3) RETURNING id`,
		s.practiceID, clientID, feedbackSent.Add(time.Hour))
	s.exec(`INSERT INTO feedback_issue_outbox (feedback_id, status, attempt_count, last_error, dead_letter_reason)
	        VALUES ($1, 'dead_lettered', 8, '401 Bad credentials', 'retries_exhausted')`, s.clientPiece)
	return s
}

func (s *feedbackStack) insertPiece(query string, args ...any) string {
	s.t.Helper()
	var id string
	if err := s.db.Admin.QueryRowContext(s.t.Context(), query, args...).Scan(&id); err != nil {
		s.t.Fatalf("seed piece: %v", err)
	}
	return id
}

func itemIDs(t *testing.T, page listPage) []string {
	t.Helper()
	ids := []string{}
	for _, item := range page.Items {
		var id string
		_ = json.Unmarshal(item["id"], &id)
		ids = append(ids, id)
	}
	return ids
}

func TestFeedbackList_ShowsEachPieceNewestFirstWithItsIssue(t *testing.T) {
	s := newFeedbackStack(t)

	page := decodeList(t, s.list("/api/feedback", s.signedInCookie()))

	if !slices.Equal(itemIDs(t, page), []string{s.clientPiece, s.staffPiece}) || page.HasMore {
		t.Fatalf("list = %v hasMore %v, want the Client's piece then the Owner's", itemIDs(t, page), page.HasMore)
	}
	got, _ := json.Marshal(page.Items[0])
	want := compactJSON(t, `{
		"id": "`+s.clientPiece+`",
		"kind": "not_working",
		"routeId": "/portal/[clientId]",
		"sentAt": "2026-09-20T15:30:00Z",
		"issue": {"state": "dead_lettered", "attempts": 8, "lastError": "401 Bad credentials", "deadLetterReason": "retries_exhausted"}
	}`)
	if compactJSON(t, string(got)) != want {
		t.Errorf("dead-lettered item =\n%s\nwant\n%s", got, want)
	}
	if got := compactJSON(t, string(page.Items[1]["issue"])); got != `{"attempts":0,"number":17,"state":"opened","url":"https://github.com/acme/feedback/issues/17"}` {
		t.Errorf("opened issue = %s", got)
	}
}

func TestFeedbackList_TheUnopenedListHoldsOnlyAPieceWhoseIssueFailed(t *testing.T) {
	s := newFeedbackStack(t)
	retrying := s.insertPiece(`INSERT INTO feedback (id, kind, text, page_url, route_id, app_build, screen_width, browser, staff_id, sent_at)
	        SELECT gen_random_uuid(), 'something_else', '', '/account', '/account', 'abc1234', 800, 'Chrome 130', staff_id, $1 FROM feedback WHERE id = $2 RETURNING id`,
		feedbackSent.Add(2*time.Hour), s.staffPiece)
	s.exec(`INSERT INTO feedback_issue_outbox (feedback_id, attempt_count, last_error) VALUES ($1, 2, '502 Bad Gateway')`, retrying)
	waiting := s.insertPiece(`INSERT INTO feedback (id, kind, text, page_url, route_id, app_build, screen_width, browser, staff_id, sent_at)
	        SELECT gen_random_uuid(), 'something_else', '', '/account', '/account', 'abc1234', 800, 'Chrome 130', staff_id, $1 FROM feedback WHERE id = $2 RETURNING id`,
		feedbackSent.Add(3*time.Hour), s.staffPiece)
	s.exec(`INSERT INTO feedback_issue_outbox (feedback_id) VALUES ($1)`, waiting)

	page := decodeList(t, s.list("/api/feedback?issue=unopened", s.signedInCookie()))

	if !slices.Equal(itemIDs(t, page), []string{retrying, s.clientPiece}) {
		t.Fatalf("unopened list = %v, want the retrying piece and the dead-lettered one", itemIDs(t, page))
	}
	if got := compactJSON(t, string(page.Items[0]["issue"])); got != `{"attempts":2,"lastError":"502 Bad Gateway","state":"retrying"}` {
		t.Errorf("retrying issue = %s", got)
	}
}

func TestFeedbackList_NamesNobodyAndRecordsNoRead(t *testing.T) {
	s := newFeedbackStack(t)

	page := decodeList(t, s.list("/api/feedback", s.signedInCookie()))

	for _, item := range page.Items {
		keys := []string{}
		for k := range item {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		if want := []string{"id", "issue", "kind", "routeId", "sentAt"}; !slices.Equal(keys, want) {
			t.Errorf("fields = %v, want %v", keys, want)
		}
	}
	if n := s.count(`SELECT count(*) FROM operator_acts WHERE action = 'feedback_read'`); n != 0 {
		t.Errorf("read rows = %d, want none for the list", n)
	}
}

func TestFeedbackList_PagesNewestFirstByCursor(t *testing.T) {
	s := newFeedbackStack(t)
	cookie := s.signedInCookie()
	for i := range feedbackPageSize {
		s.insertPiece(`INSERT INTO feedback (id, kind, text, page_url, route_id, app_build, screen_width, browser, staff_id, sent_at)
		        SELECT gen_random_uuid(), 'something_else', '', '/account', '/account', 'abc1234', 800, 'Chrome 130', staff_id, $1 FROM feedback WHERE id = $2 RETURNING id`,
			feedbackSent.Add(time.Duration(i+2)*time.Hour), s.staffPiece)
	}

	first := decodeList(t, s.list("/api/feedback", cookie))
	if len(first.Items) != feedbackPageSize || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page: %d items, hasMore %v, cursor %v", len(first.Items), first.HasMore, first.NextCursor)
	}
	second := decodeList(t, s.list("/api/feedback?cursor="+*first.NextCursor, cookie))
	if !slices.Equal(itemIDs(t, second), []string{s.clientPiece, s.staffPiece}) || second.HasMore {
		t.Fatalf("second page = %v hasMore %v, want the two oldest", itemIDs(t, second), second.HasMore)
	}
}

func TestFeedbackList_RefusesAMalformedCursorOrFilter(t *testing.T) {
	s := newFeedbackStack(t)
	cookie := s.signedInCookie()

	for _, path := range []string{"/api/feedback?cursor=not-a-cursor", "/api/feedback?issue=closed"} {
		if w := s.list(path, cookie); w.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400; body %s", path, w.Code, w.Body)
		}
	}
}

func TestFeedbackPiece_ShowsEveryFieldAndAStaffSendersNameAndEmail(t *testing.T) {
	s := newFeedbackStack(t)
	s.exec(`UPDATE staff SET email = 'renata@example.com' WHERE identity_uid = 'fb-owner'`)

	w := s.list("/api/feedback/"+s.staffPiece, s.signedInCookie())

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", w.Code, w.Body)
	}
	want := compactJSON(t, `{
		"id": "`+s.staffPiece+`",
		"kind": "idea_or_request",
		"text": "Line one.\nLine two.",
		"pageUrl": "/practices/x/invoices",
		"routeId": "/practices/[practiceId]/invoices",
		"appBuild": "abc1234",
		"screenWidth": 1280,
		"browser": "Firefox 131",
		"sentAt": "2026-09-20T14:30:00Z",
		"role": "owner, doula",
		"practiceName": "Rooted Birth Collective",
		"sender": {"kind": "staff", "name": "Renata Alvarez", "email": "renata@example.com"},
		"issue": {"state": "opened", "number": 17, "url": "https://github.com/acme/feedback/issues/17", "attempts": 0}
	}`)
	if got := compactJSON(t, w.Body.String()); got != want {
		t.Errorf("piece =\n%s\nwant\n%s", got, want)
	}
}

func TestFeedbackPiece_NamesAClientSenderByHerNameAndSignInAddress(t *testing.T) {
	s := newFeedbackStack(t)

	w := s.list("/api/feedback/"+s.clientPiece, s.signedInCookie())

	var piece struct {
		Role   string          `json:"role"`
		Sender json.RawMessage `json:"sender"`
		Issue  json.RawMessage `json:"issue"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &piece); err != nil || w.Code != http.StatusOK {
		t.Fatalf("status %d, body %s, err %v", w.Code, w.Body, err)
	}
	if got := compactJSON(t, string(piece.Sender)); got != `{"email":"nadia.portal@example.com","kind":"client","name":"Nadia Haddad"}` {
		t.Errorf("sender = %s", got)
	}
	if piece.Role != "Client" {
		t.Errorf("role = %q, want Client", piece.Role)
	}
	if got := compactJSON(t, string(piece.Issue)); got != `{"attempts":8,"deadLetterReason":"retries_exhausted","lastError":"401 Bad credentials","state":"dead_lettered"}` {
		t.Errorf("issue = %s", got)
	}
}

func TestFeedbackPiece_OpeningItRecordsWhoReadWhichPieceAndWhen(t *testing.T) {
	s := newFeedbackStack(t)
	cookie := s.signedInCookie()
	s.now = s.now.Add(time.Minute)

	if w := s.list("/api/feedback/"+s.clientPiece, cookie); w.Code != http.StatusOK {
		t.Fatalf("status = %d; body %s", w.Code, w.Body)
	}

	var subjectKind, subjectID string
	var details []byte
	var stepUpAt *time.Time
	if err := s.db.Admin.QueryRowContext(t.Context(),
		`SELECT a.subject_kind, a.subject_id, a.details, a.step_up_at FROM operator_acts a JOIN operators o ON o.id = a.operator_id
		  WHERE o.identity_uid = $1 AND a.action = 'feedback_read' AND a.created_at = $2`, opUID, s.now,
	).Scan(&subjectKind, &subjectID, &details, &stepUpAt); err != nil {
		t.Fatalf("read the read row: %v", err)
	}
	if subjectKind != "feedback" || subjectID != s.clientPiece || string(details) != "{}" || stepUpAt != nil {
		t.Errorf("read row = %s %s %s %v, want the piece and nothing of its content", subjectKind, subjectID, details, stepUpAt)
	}
}

func TestFeedbackPiece_APieceThatIsGoneIsNotFoundAndRecordsNoRead(t *testing.T) {
	s := newFeedbackStack(t)
	cookie := s.signedInCookie()

	for _, id := range []string{"00000000-0000-4000-8000-000000000000", "not-an-id"} {
		if w := s.list("/api/feedback/"+id, cookie); w.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404; body %s", id, w.Code, w.Body)
		}
	}
	if n := s.count(`SELECT count(*) FROM operator_acts WHERE action = 'feedback_read'`); n != 0 {
		t.Errorf("read rows = %d, want none", n)
	}
}

func TestFeedback_RefusedWithoutASession(t *testing.T) {
	s := newFeedbackStack(t)

	for _, path := range []string{"/api/feedback", "/api/feedback/" + s.staffPiece} {
		if w := s.list(path, ""); w.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401", path, w.Code)
		}
	}
	if n := s.count(`SELECT count(*) FROM operator_acts WHERE action = 'feedback_read'`); n != 0 {
		t.Errorf("read rows = %d, want none", n)
	}
}
