package staffauth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/feedback"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// readAt is the instant every test here pins the request clock to, so a
// feedback_reads row's read_at is compared against a literal.
var readAt = time.Date(2026, 11, 3, 14, 30, 0, 0, time.UTC)

// seededPiece is one piece of Feedback as a test seeds it. Zero values
// are a Staff piece sent from /account: no Practice, no roles, no issue.
type seededPiece struct {
	text          string
	staffID       string
	portalAccount string
	practiceID    string
	clientID      string
	roles         string // a practice_role[] literal, e.g. "{owner,admin}"
	issueNumber   int
	sentAt        time.Time
}

func seedPiece(t *testing.T, db *testdb.DB, p seededPiece) string {
	t.Helper()
	id := uuid.NewString()
	if p.sentAt.IsZero() {
		p.sentAt = time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	}
	nullable := func(s string) any {
		if s == "" {
			return nil
		}
		return s
	}
	var issue any
	if p.issueNumber != 0 {
		issue = p.issueNumber
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO feedback
		    (id, kind, text, page_url, route_id, app_build, screen_width, browser,
		     staff_id, portal_account, practice_id, client_id, roles, issue_number, sent_at)
		 VALUES ($1, 'not_working', $2, '/practices/p-1/clients/c-1', '/practices/[practiceId]/clients/[clientId]',
		         'abc1234', 390, 'Safari 18', $3, $4, $5, $6, $7::practice_role[], $8, $9)`,
		id, p.text, nullable(p.staffID), nullable(p.portalAccount), nullable(p.practiceID), nullable(p.clientID),
		nullable(p.roles), issue, p.sentAt,
	); err != nil {
		t.Fatalf("seed feedback: %v", err)
	}
	return id
}

// seedOpenJob gives feedbackID the open job a send queues, in the state
// a test needs it in.
func seedOpenJob(t *testing.T, db *testdb.DB, feedbackID, status string, attempts int, lastError string) {
	t.Helper()
	var lastErr any
	if lastError != "" {
		lastErr = lastError
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO feedback_issue_outbox (feedback_id, status, attempt_count, last_error)
		 VALUES ($1, $2::feedback_issue_outbox_status, $3, $4)`,
		feedbackID, status, attempts, lastErr,
	); err != nil {
		t.Fatalf("seed open job: %v", err)
	}
}

// founderFixture is a founder with a second-factor session and a server
// whose request clock is pinned to readAt.
type founderFixture struct {
	db        *testdb.DB
	srv       *httptest.Server
	founderID string
	session   string
}

func newFounderFixture(t *testing.T) founderFixture {
	t.Helper()
	db := testdb.New(t)
	founderID := testdb.SeedStaff(t, db, founderUID)
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	staffauth.MountFounderRoutes(g, db.App, staffauth.FounderConfig{StaffID: founderID, FeedbackRepo: feedbackRepo})
	srv := httptest.NewServer(clock.Middleware(func() time.Time { return readAt })(mux))
	t.Cleanup(srv.Close)
	// Minted at readAt, not at the wall clock: authn.Begin reads the
	// request clock, and a session minted today is long expired by then.
	return founderFixture{db: db, srv: srv, founderID: founderID, session: authntest.SeedSessionAt(t, db.App, founderUID, readAt)}
}

func (f founderFixture) get(t *testing.T, path string) reply {
	t.Helper()
	return getAsSession(t, f.srv, f.session, path)
}

func decodeBody[T any](t *testing.T, r reply) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(r.body, &v); err != nil {
		t.Fatalf("decode body %q: %v", r.body, err)
	}
	return v
}

// reads returns every feedback_reads row for feedbackID as "staff id @
// instant", oldest first.
func reads(t *testing.T, db *testdb.DB, feedbackID string) []string {
	t.Helper()
	rows, err := db.Admin.QueryContext(context.WithoutCancel(t.Context()),
		`SELECT staff_id::text, read_at FROM feedback_reads WHERE feedback_id = $1 ORDER BY read_at, id`, feedbackID)
	if err != nil {
		t.Fatalf("read feedback_reads: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var got []string
	for rows.Next() {
		var staffID string
		var at time.Time
		if err := rows.Scan(&staffID, &at); err != nil {
			t.Fatalf("scan feedback_reads: %v", err)
		}
		got = append(got, staffID+" @ "+at.UTC().Format(time.RFC3339))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate feedback_reads: %v", err)
	}
	return got
}

// The read page shows every stored field of a Staff member's piece
// (#1526), with the sender and the Practice named.
func TestReadFeedback_ShowsEveryStoredFieldOfAStaffPiece(t *testing.T) {
	f := newFounderFixture(t)
	practiceID := testdb.SeedPractice(t, f.db, "Riverside Doula Collective")
	senderID := testdb.SeedNamedStaffAtPractice(t, f.db, practiceID, "sender-uid", "Anne-Marie Ochieng", []string{ownerRole, adminRole}, employeeType)
	sentAt := time.Date(2026, 10, 20, 8, 15, 0, 0, time.UTC)
	id := seedPiece(t, f.db, seededPiece{
		text: "The invoice total is wrong.", staffID: senderID, practiceID: practiceID,
		roles: "{owner,admin}", issueNumber: 12, sentAt: sentAt,
	})

	resp := f.get(t, "/api/staff/feedback/"+id)
	if resp.status != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.status, http.StatusOK)
	}
	got := decodeBody[feedback.Piece](t, resp)
	// The same instant, whichever zone the database session printed it in.
	got.SentAt = got.SentAt.UTC()

	number := 12
	issueURL := "https://github.com/markgoho/doula-cloud-feedback/issues/12"
	want := feedback.Piece{
		ID: id, Kind: feedback.KindNotWorking, Text: "The invoice total is wrong.",
		PageURL: "/practices/p-1/clients/c-1", RouteID: "/practices/[practiceId]/clients/[clientId]",
		AppBuild: "abc1234", ScreenWidth: 390, Browser: "Safari 18", SentAt: sentAt,
		Role: "owner, admin", PracticeName: "Riverside Doula Collective",
		Sender: feedback.Sender{Kind: feedback.SenderStaff, Name: "Anne-Marie Ochieng", Email: "sender-uid@example.com"},
		Issue:  feedback.IssueStatus{State: feedback.IssueOpened, Number: &number, URL: &issueURL},
	}
	if gotJSON, wantJSON := mustJSON(t, got), mustJSON(t, want); gotJSON != wantJSON {
		t.Fatalf("piece =\n%s\nwant\n%s", gotJSON, wantJSON)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

// A Portal sender is named by her Portal Account's sign-in address and
// nothing else, and her role is "Client" (#1501 Q2).
func TestReadFeedback_NamesAPortalSenderByHerSignInAddress(t *testing.T) {
	f := newFounderFixture(t)
	practiceID := testdb.SeedPractice(t, f.db, "Riverside Doula Collective")
	clientID := testdb.SeedNamedClient(t, f.db, practiceID, "Priya", "priya@example.com")
	portalUID := testdb.PortalUID("priya")
	testdb.SeedPortalAccount(t, f.db, portalUID, "priya.signin@example.com")
	id := seedPiece(t, f.db, seededPiece{portalAccount: portalUID, practiceID: practiceID, clientID: clientID})
	seedOpenJob(t, f.db, id, "pending", 0, "")

	resp := f.get(t, "/api/staff/feedback/"+id)
	if resp.status != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.status, http.StatusOK)
	}
	got := decodeBody[feedback.Piece](t, resp)

	if want := (feedback.Sender{Kind: feedback.SenderClient, Email: "priya.signin@example.com"}); got.Sender != want {
		t.Errorf("sender = %+v, want %+v", got.Sender, want)
	}
	if got.Role != "Client" {
		t.Errorf("role = %q, want Client", got.Role)
	}
	if got.PracticeName != "Riverside Doula Collective" {
		t.Errorf("practice name = %q, want the Practice the screen was under", got.PracticeName)
	}
	if got.Issue.State != feedback.IssuePending || got.Issue.Number != nil || got.Issue.URL != nil {
		t.Errorf("issue = %+v, want pending with no number and no link", got.Issue)
	}
}

// A piece sent from /account has no Practice: the role reads "Staff" and
// the Practice name is absent.
func TestReadFeedback_APieceWithNoPracticeNamesNone(t *testing.T) {
	f := newFounderFixture(t)
	id := seedPiece(t, f.db, seededPiece{staffID: f.founderID})

	got := decodeBody[feedback.Piece](t, f.get(t, "/api/staff/feedback/"+id))

	if got.Role != "Staff" || got.PracticeName != "" {
		t.Errorf("role = %q, practice name = %q, want Staff and none", got.Role, got.PracticeName)
	}
}

// Every open of a piece writes a feedback_reads row: who, and when, from
// the request clock (#1526). Two opens are two rows.
func TestReadFeedback_EveryOpenWritesAReadRow(t *testing.T) {
	f := newFounderFixture(t)
	id := seedPiece(t, f.db, seededPiece{staffID: f.founderID})

	for range 2 {
		if resp := f.get(t, "/api/staff/feedback/"+id); resp.status != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.status, http.StatusOK)
		}
	}

	row := f.founderID + " @ 2026-11-03T14:30:00Z"
	if got := reads(t, f.db, id); len(got) != 2 || got[0] != row || got[1] != row {
		t.Fatalf("feedback_reads = %v, want two rows of %q", got, row)
	}
}

// The list is not an open: it shows no free text and records nothing.
func TestListFeedback_WritesNoReadRow(t *testing.T) {
	f := newFounderFixture(t)
	id := seedPiece(t, f.db, seededPiece{staffID: f.founderID})

	if resp := f.get(t, "/api/staff/feedback"); resp.status != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.status, http.StatusOK)
	}
	if got := reads(t, f.db, id); len(got) != 0 {
		t.Fatalf("feedback_reads = %v, want none after a list", got)
	}
}

// A refused request reads nothing and records nothing.
func TestReadFeedback_ARefusedRequestWritesNoReadRow(t *testing.T) {
	f := newFounderFixture(t)
	id := seedPiece(t, f.db, seededPiece{staffID: f.founderID})
	testdb.SeedStaff(t, f.db, "a-doula")
	notFounder := authntest.SeedSessionAt(t, f.db.App, "a-doula", readAt)
	noFactor := authntest.SeedSessionWithSecondFactor(t, f.db.App, founderUID, false)

	if resp := getAsSession(t, f.srv, notFounder, "/api/staff/feedback/"+id); resp.status != http.StatusNotFound {
		t.Fatalf("non-founder status = %d, want %d", resp.status, http.StatusNotFound)
	}
	// noFactor is minted at the wall clock, so the pinned request clock
	// finds it expired; a server on the wall clock is what reaches the
	// second-factor check.
	wallClock := newFounderServer(t, f.db, f.founderID)
	if resp := getAsSession(t, wallClock, noFactor, "/api/staff/feedback/"+id); resp.status != http.StatusForbidden {
		t.Fatalf("founder with no second factor status = %d, want %d", resp.status, http.StatusForbidden)
	}
	if got := reads(t, f.db, id); len(got) != 0 {
		t.Fatalf("feedback_reads = %v, want none after two refusals", got)
	}
}

// An id that is not an id at all is the same 404 as a piece that is not
// there.
func TestReadFeedback_AMalformedIDIsNotFound(t *testing.T) {
	f := newFounderFixture(t)

	resp := f.get(t, "/api/staff/feedback/not-an-id")
	if resp.status != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.status, http.StatusNotFound)
	}
}
