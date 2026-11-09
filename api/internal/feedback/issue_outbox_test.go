package feedback_test

import (
	"database/sql"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"doula-cloud/api/internal/feedback"
	"doula-cloud/api/internal/testdb"
)

const testIssueOpsBaseURL = "https://ops.example.test"

// feedback_issue_outbox_status's own two states this file asserts
// against, named once so goconst has one spelling to point at.
const (
	statusSent    = "sent"
	statusPending = "pending"
)

// seedStaffFeedbackRow inserts a bare feedback row directly, bypassing
// #1523's own handlers -- their coverage is staffauth's and clientauth's,
// not this worker's. sent from staffID with no Practice in context.
func seedStaffFeedbackRow(t *testing.T, db *testdb.DB, staffID, kind, routeID, text string) string {
	t.Helper()
	id := uuid.NewString()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO feedback (id, kind, text, page_url, route_id, app_build, screen_width, browser, staff_id, sent_at)
		 VALUES ($1, $2::feedback_kind, $3, '/account', $4, 'abc1234', 390, 'Safari 18', $5, now())`,
		id, kind, text, routeID, staffID,
	); err != nil {
		t.Fatalf("seed feedback row: %v", err)
	}
	return id
}

// seedStaffFeedbackRowWithRoles inserts a feedback row sent from under a
// Practice, carrying roles -- the shape #1524's role-on-the-issue AC
// needs a real practice_role[] literal for.
func seedStaffFeedbackRowWithRoles(t *testing.T, db *testdb.DB, staffID, practiceID, kind, routeID string) string {
	t.Helper()
	id := uuid.NewString()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO feedback (id, kind, text, page_url, route_id, app_build, screen_width, browser, staff_id, practice_id, roles, sent_at)
		 VALUES ($1, $2::feedback_kind, '', '/practices/x', $3, 'abc1234', 390, 'Safari 18', $4, $5, '{owner,admin}', now())`,
		id, kind, routeID, staffID, practiceID,
	); err != nil {
		t.Fatalf("seed feedback row: %v", err)
	}
	return id
}

// seedStaffFeedbackRowZeroRoles inserts a feedback row sent from under a
// real Practice by a Staff member whose membership carries zero roles --
// staffauth.staffRolesAt's own "a membership can start with zero roles"
// case, distinct from no Practice in context at all.
func seedStaffFeedbackRowZeroRoles(t *testing.T, db *testdb.DB, staffID, practiceID string) string {
	t.Helper()
	id := uuid.NewString()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO feedback (id, kind, text, page_url, route_id, app_build, screen_width, browser, staff_id, practice_id, roles, sent_at)
		 VALUES ($1, 'not_working', '', '/practices/x', '/practices/[practiceId]', 'abc1234', 390, 'Safari 18', $2, $3, '{}', now())`,
		id, staffID, practiceID,
	); err != nil {
		t.Fatalf("seed feedback row: %v", err)
	}
	return id
}

// seedPortalFeedbackRow inserts a feedback row sent by a Portal Account.
func seedPortalFeedbackRow(t *testing.T, db *testdb.DB, portalAccount, kind, routeID string) string {
	t.Helper()
	id := uuid.NewString()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO feedback (id, kind, text, page_url, route_id, app_build, screen_width, browser, portal_account, sent_at)
		 VALUES ($1, $2::feedback_kind, '', '/portal', $3, 'abc1234', 390, 'Safari 18', $4, now())`,
		id, kind, routeID, portalAccount,
	); err != nil {
		t.Fatalf("seed feedback row: %v", err)
	}
	return id
}

func enqueueIssueOutbox(t *testing.T, db *testdb.DB, feedbackID string) {
	t.Helper()
	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := feedback.EnqueueIssueOutbox(t.Context(), tx, feedbackID); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

// runIssueWorker runs one pass of worker in its own transaction, the way
// outbox.ProcessHandler does in production -- the superuser Admin
// connection, matching client.ErasureWorker's own test convention, since
// this file is about the worker's own logic rather than the RLS door
// TestProcessIssueOutboxHandler_RunsBehindTheDoor below proves.
func runIssueWorker(t *testing.T, db *testdb.DB, worker feedback.IssueWorker) {
	t.Helper()
	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := worker.ProcessPending(t.Context(), tx); err != nil {
		t.Fatalf("process pending: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func readIssueOutboxStatus(t *testing.T, db *testdb.DB, feedbackID string) string {
	t.Helper()
	var status string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status::text FROM feedback_issue_outbox WHERE feedback_id = $1`, feedbackID,
	).Scan(&status); err != nil {
		t.Fatalf("read outbox status: %v", err)
	}
	return status
}

func readIssueNumber(t *testing.T, db *testdb.DB, feedbackID string) sql.NullInt64 {
	t.Helper()
	var n sql.NullInt64
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT issue_number FROM feedback WHERE id = $1`, feedbackID,
	).Scan(&n); err != nil {
		t.Fatalf("read issue number: %v", err)
	}
	return n
}

// issueEnv is a Feedback piece with its issue job pending, a fake GitHub
// and a worker pointed at it.
type issueEnv struct {
	T          *testing.T
	DB         *testdb.DB
	FeedbackID string
	Creator    *feedback.FakeIssueCreator
	Worker     feedback.IssueWorker
}

// newIssueEnvFor queues feedbackID's issue job.
func newIssueEnvFor(t *testing.T, db *testdb.DB, feedbackID string) *issueEnv {
	t.Helper()
	enqueueIssueOutbox(t, db, feedbackID)
	creator := feedback.NewFakeIssueCreator()
	return &issueEnv{T: t, DB: db, FeedbackID: feedbackID, Creator: creator, Worker: newTestIssueWorker(creator)}
}

// newStaffIssueEnv is a bare Staff piece of the given kind sent from
// routeID, with its job queued.
func newStaffIssueEnv(t *testing.T, uid, kind, routeID string) *issueEnv {
	t.Helper()
	db := testdb.New(t)
	staffID := testdb.SeedStaff(t, db, uid)
	return newIssueEnvFor(t, db, seedStaffFeedbackRow(t, db, staffID, kind, routeID, ""))
}

func (e *issueEnv) run() {
	e.T.Helper()
	runIssueWorker(e.T, e.DB, e.Worker)
}

func (e *issueEnv) requireStatus(want string) {
	e.T.Helper()
	if got := readIssueOutboxStatus(e.T, e.DB, e.FeedbackID); got != want {
		e.T.Fatalf("outbox status = %q, want %s", got, want)
	}
}

// openedIssue is the issue the worker wrote back onto the piece.
func (e *issueEnv) openedIssue() feedback.FakeIssue {
	e.T.Helper()
	number := readIssueNumber(e.T, e.DB, e.FeedbackID)
	if !number.Valid {
		e.T.Fatal("issue_number is NULL, want set")
	}
	issue, ok := e.Creator.Issues[int(number.Int64)]
	if !ok {
		e.T.Fatalf("no fake issue #%d", number.Int64)
	}
	return issue
}

// seedMarkedIssue is the crash window #1500's research names: GitHub
// answered 201 and the row is still pending, so an issue carrying the
// piece's marker already exists.
func (e *issueEnv) seedMarkedIssue() int {
	e.T.Helper()
	number, err := e.Creator.CreateIssue(e.T.Context(), "stale title", markerBody(e.FeedbackID))
	if err != nil {
		e.T.Fatalf("seed existing issue: %v", err)
	}
	return number
}

// TestIssueWorker_FirstCreateOpensAnIssueAndWritesTheNumberBack is
// #1524's own AC's first case: a first create.
func TestIssueWorker_FirstCreateOpensAnIssueAndWritesTheNumberBack(t *testing.T) {
	e := newStaffIssueEnv(t, "feedback-worker-first-create", feedback.KindNotWorking, "/clients/[clientId]")
	e.run()

	e.requireStatus(statusSent)
	issue := e.openedIssue()
	if want := "Something is not working: /clients/[clientId]"; issue.Title != want {
		t.Errorf("title = %q, want %q", issue.Title, want)
	}
	if len(issue.Labels) != 1 || issue.Labels[0] != testNotWorkingLabel {
		t.Errorf("labels = %v, want [%q]", issue.Labels, testNotWorkingLabel)
	}
	if !strings.Contains(issue.Body, "/feedback/"+e.FeedbackID) {
		t.Errorf("body = %q, want the founder read page link", issue.Body)
	}
	if !strings.Contains(issue.Body, "Role: Staff") {
		t.Errorf("body = %q, want Role: Staff for a sender with no Practice in context", issue.Body)
	}
}

// TestIssueWorker_RetryAdoptsAnIssueFoundByItsMarker is #1524's own AC's
// second case: a retry that finds its marker creates nothing.
func TestIssueWorker_RetryAdoptsAnIssueFoundByItsMarker(t *testing.T) {
	e := newStaffIssueEnv(t, "feedback-worker-retry", feedback.KindIdeaOrRequest, testAccountPage)
	existingNumber := e.seedMarkedIssue()
	e.run()

	e.requireStatus(statusSent)
	number := readIssueNumber(t, e.DB, e.FeedbackID)
	if !number.Valid || int(number.Int64) != existingNumber {
		t.Fatalf("issue_number = %v, want %d -- adopted, not a second issue", number, existingNumber)
	}
	if len(e.Creator.Issues) != 1 {
		t.Fatalf("issues = %d, want 1 -- no second issue opened", len(e.Creator.Issues))
	}
	if labels := e.Creator.Issues[existingNumber].Labels; len(labels) != 1 || labels[0] != testIdeaOrRequestLabel {
		t.Errorf("labels = %v, want [%q] -- an adopted issue gets its kind label too (#1587)", labels, testIdeaOrRequestLabel)
	}
}

// makeIssueOutboxRowDue pulls a row's next_attempt_at back to now, the
// same statement the offer outbox tests run inline:
// outbox.Worker.MarkFailed schedules the retry a backoff step out, and
// issueClaimQuery compares against Postgres's now(), so a second pass
// would otherwise claim nothing.
func makeIssueOutboxRowDue(t *testing.T, db *testdb.DB, feedbackID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE feedback_issue_outbox SET next_attempt_at = now() WHERE feedback_id = $1`, feedbackID,
	); err != nil {
		t.Fatalf("make outbox row due: %v", err)
	}
}

// TestIssueWorker_RetryAfterAFailedAddLabelsAdoptsAndLabels is #1587's
// own sequence, end to end: the create succeeds, the labels call fails,
// and the retry adopts the issue by its marker. That retry must add the
// kind label -- before #1587 it marked the row sent and left the issue
// with no label for good.
func TestIssueWorker_RetryAfterAFailedAddLabelsAdoptsAndLabels(t *testing.T) {
	e := newStaffIssueEnv(t, "feedback-worker-labels-retry", feedback.KindNotWorking, testAccountPage)
	e.Creator.AddLabelsErr = errGitHubDown
	e.run()

	e.requireStatus(statusPending)
	if len(e.Creator.Issues) != 1 {
		t.Fatalf("issues = %d, want 1 -- the create itself succeeded", len(e.Creator.Issues))
	}

	e.Creator.AddLabelsErr = nil
	makeIssueOutboxRowDue(t, e.DB, e.FeedbackID)
	e.run()

	e.requireStatus(statusSent)
	if len(e.Creator.Issues) != 1 {
		t.Fatalf("issues = %d, want 1 -- the retry adopts, it opens no second issue", len(e.Creator.Issues))
	}
	if labels := e.openedIssue().Labels; len(labels) != 1 || labels[0] != testNotWorkingLabel {
		t.Errorf("labels = %v, want [%q] -- the retry adds the label the first attempt could not", labels, testNotWorkingLabel)
	}
}

// TestFakeIssueCreator_AddLabelsDoesNotRepeatALabel holds the fake to
// the real add-labels endpoint's own behavior: a label an issue already
// has is not added a second time. The worker labels on every attempt
// (#1587), so a retry that adopts an already-labeled issue must leave
// the fake showing what GitHub would show.
func TestFakeIssueCreator_AddLabelsDoesNotRepeatALabel(t *testing.T) {
	creator := feedback.NewFakeIssueCreator()
	number, err := creator.CreateIssue(t.Context(), "title", "body")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	for range 2 {
		if err := creator.AddLabels(t.Context(), number, []string{testNotWorkingLabel}); err != nil {
			t.Fatalf("add labels: %v", err)
		}
	}
	if labels := creator.Issues[number].Labels; len(labels) != 1 || labels[0] != testNotWorkingLabel {
		t.Errorf("labels = %v, want [%q] exactly once", labels, testNotWorkingLabel)
	}
	if len(creator.LabelCalls) != 2 {
		t.Errorf("label calls = %d, want 2 -- every call is still recorded", len(creator.LabelCalls))
	}
}

// TestIssueWorker_GitHubErrorLeavesTheRowPending covers each call that
// can fail: the create, the list before it, the labels call after a
// create (the number is not written yet, so a retry still owns finishing
// the job) and the labels call while adopting (the same, on the adopt
// path).
func TestIssueWorker_GitHubErrorLeavesTheRowPending(t *testing.T) {
	cases := []struct {
		name       string
		adopting   bool
		breakIt    func(*feedback.FakeIssueCreator)
		wantIssues int
	}{
		{"create", false, func(c *feedback.FakeIssueCreator) { c.CreateIssueErr = errGitHubDown }, 0},
		{"list", false, func(c *feedback.FakeIssueCreator) { c.ListIssuesErr = errGitHubDown }, 0},
		{"add labels after a create", false, func(c *feedback.FakeIssueCreator) { c.AddLabelsErr = errGitHubDown }, 1},
		{"add labels while adopting", true, func(c *feedback.FakeIssueCreator) { c.AddLabelsErr = errGitHubDown }, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newStaffIssueEnv(t, "feedback-worker-github-error", feedback.KindSomethingElse, testAccountPage)
			if tc.adopting {
				e.seedMarkedIssue()
			}
			tc.breakIt(e.Creator)
			e.run()

			e.requireStatus(statusPending)
			if number := readIssueNumber(t, e.DB, e.FeedbackID); number.Valid {
				t.Fatalf("issue_number = %v, want NULL", number)
			}
			if len(e.Creator.Issues) != tc.wantIssues {
				t.Fatalf("issues = %d, want %d", len(e.Creator.Issues), tc.wantIssues)
			}
		})
	}
}

// TestIssueWorker_BodyNeverCarriesTheFreeText is #1524's own AC's fourth
// case: a body that holds no free text. issueClaimQuery does not even
// select feedback.text, so this proves the guarantee at the boundary a
// future edit to issueBody could otherwise break unnoticed.
func TestIssueWorker_BodyNeverCarriesTheFreeText(t *testing.T) {
	db := testdb.New(t)
	staffID := testdb.SeedStaff(t, db, "feedback-worker-no-text")
	sentinel := "the client's name is Ada Lovelace, her diagnosis is..."
	e := newIssueEnvFor(t, db, seedStaffFeedbackRow(t, db, staffID, feedback.KindNotWorking, testAccountPage, sentinel))
	e.run()

	issue := e.openedIssue()
	if strings.Contains(issue.Title, sentinel) || strings.Contains(issue.Body, sentinel) {
		t.Fatalf("issue leaked the free text: title=%q body=%q", issue.Title, issue.Body)
	}
	if strings.Contains(issue.Body, "Ada Lovelace") {
		t.Fatal("issue body carries the sender's typed text")
	}
}

// TestIssueWorker_BodyNamesTheSenderRole holds the Role line to #1501
// Q2's rule for each sender shape that is not a Staff member with no
// Practice in context: a Portal sender is always "Client" because a role
// with no Practice identifies nobody; a Staff sender at a Practice
// reports her actual roles; and one whose membership carries zero roles
// reports no role name at all, per feedbackRole's own doc comment --
// "Staff" would misstate that a Practice is in context.
func TestIssueWorker_BodyNamesTheSenderRole(t *testing.T) {
	cases := []struct {
		name    string
		seed    func(*testing.T, *testdb.DB) string
		want    string
		notWant string
	}{
		{"Portal sender", func(t *testing.T, db *testdb.DB) string {
			testdb.SeedPortalAccount(t, db, "feedback-worker-portal", "feedback-worker-portal@example.com")
			return seedPortalFeedbackRow(t, db, "feedback-worker-portal", feedback.KindIdeaOrRequest, "/portal")
		}, "Role: Client", ""},
		{"Staff sender with roles", func(t *testing.T, db *testdb.DB) string {
			practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, "feedback-worker-roles", []string{testOwnerRole, testAdminRole}, "employee")
			return seedStaffFeedbackRowWithRoles(t, db, staffID, practiceID, feedback.KindNotWorking, "/practices/[practiceId]")
		}, "Role: owner, admin", ""},
		{"zero-role membership", func(t *testing.T, db *testdb.DB) string {
			practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, "feedback-worker-zero-role", nil, "employee")
			return seedStaffFeedbackRowZeroRoles(t, db, staffID, practiceID)
		}, "Role: \n", "Role: Staff"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			e := newIssueEnvFor(t, db, tc.seed(t, db))
			e.run()

			body := e.openedIssue().Body
			if !strings.Contains(body, tc.want) {
				t.Errorf("body = %q, want %q", body, tc.want)
			}
			if tc.notWant != "" && strings.Contains(body, tc.notWant) {
				t.Errorf("body = %q, must not contain %q", body, tc.notWant)
			}
		})
	}
}

// TestProcessIssueOutboxHandler_RunsBehindTheDoor proves the migration's
// own RLS policies, not just the worker's Go logic: run through
// outbox.ProcessHandler against db.App (the app_runtime role, not the
// superuser Admin connection every other test in this file uses), the
// worker must still be able to read the feedback row it needs and write
// issue_number back, licensed by the same notification_worker_trusted
// door every mailing outbox already opens.
func TestProcessIssueOutboxHandler_RunsBehindTheDoor(t *testing.T) {
	e := newStaffIssueEnv(t, "feedback-worker-endpoint", feedback.KindNotWorking, testAccountPage)
	postProcessIssueOutbox(t, e.DB, e.Worker)

	if status := readIssueOutboxStatus(t, e.DB, e.FeedbackID); status != statusSent {
		t.Fatalf("outbox status = %q, want sent -- app_runtime must reach feedback through the door", status)
	}
	if number := readIssueNumber(t, e.DB, e.FeedbackID); !number.Valid {
		t.Fatal("issue_number is NULL, want set through the door's UPDATE policy")
	}
}

// testErasedLabel is the label #1519 names for an erased piece
// (issue_outbox.go's unexported erasedLabel), repeated here for the same
// reason testNotWorkingLabel is.
const testErasedLabel = "erased"

// seedStaffFeedbackRowSentAt inserts a bare Staff feedback row with a
// chosen sent_at -- what the retention tests move the clock against.
func seedStaffFeedbackRowSentAt(t *testing.T, db *testdb.DB, staffID string, sentAt time.Time) string {
	t.Helper()
	id := uuid.NewString()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO feedback (id, kind, text, page_url, route_id, app_build, screen_width, browser, staff_id, sent_at)
		 VALUES ($1, 'not_working', '', '/account', '/account', 'abc1234', 390, 'Safari 18', $2, $3)`,
		id, staffID, sentAt,
	); err != nil {
		t.Fatalf("seed feedback row: %v", err)
	}
	return id
}

// seedCloseJob inserts a pending "close as erased" job for issueNumber,
// the row erase_client_feedback (00121) writes inside a Client's
// Erasure. Seeded directly because the Erasure's own coverage is the
// client package's, not this worker's.
func seedCloseJob(t *testing.T, db *testdb.DB, issueNumber int) (outboxID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO feedback_issue_outbox (act, issue_number) VALUES ('close_erased', $1) RETURNING id`,
		issueNumber,
	).Scan(&outboxID); err != nil {
		t.Fatalf("seed close job: %v", err)
	}
	return outboxID
}

func readOutboxStatusByID(t *testing.T, db *testdb.DB, outboxID string) string {
	t.Helper()
	var status string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status::text FROM feedback_issue_outbox WHERE id = $1`, outboxID,
	).Scan(&status); err != nil {
		t.Fatalf("read outbox status: %v", err)
	}
	return status
}

func feedbackRowExists(t *testing.T, db *testdb.DB, feedbackID string) bool {
	t.Helper()
	var exists bool
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT EXISTS (SELECT 1 FROM feedback WHERE id = $1)`, feedbackID,
	).Scan(&exists); err != nil {
		t.Fatalf("read feedback row: %v", err)
	}
	return exists
}

func deleteFeedbackRow(t *testing.T, db *testdb.DB, feedbackID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(), `DELETE FROM feedback WHERE id = $1`, feedbackID); err != nil {
		t.Fatalf("delete feedback row: %v", err)
	}
}

// seedOpenedIssue stands in for an issue the worker opened on an earlier
// run: created and labeled on the fake, its number returned.
func seedOpenedIssue(t *testing.T, creator *feedback.FakeIssueCreator, body string) int {
	t.Helper()
	number, err := creator.CreateIssue(t.Context(), "Something is not working: /account", body)
	if err != nil {
		t.Fatalf("seed issue: %v", err)
	}
	if err := creator.AddLabels(t.Context(), number, []string{testNotWorkingLabel}); err != nil {
		t.Fatalf("seed issue label: %v", err)
	}
	return number
}

func hasLabel(issue feedback.FakeIssue, label string) bool {
	return slices.Contains(issue.Labels, label)
}

// markerBody is an issue body carrying feedbackID's hidden marker, the
// way issueBody writes it.
func markerBody(feedbackID string) string {
	return "body\n\n<!-- doula-cloud-feedback-id: " + feedbackID + " -->\n"
}
