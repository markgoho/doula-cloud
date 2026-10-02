package feedback_test

import (
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"doula-cloud/api/internal/feedback"
	"doula-cloud/api/internal/internalauth"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/testdb"
)

const testIssueAppBaseURL = "https://app.example.test"

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

// TestIssueWorker_FirstCreateOpensAnIssueAndWritesTheNumberBack is
// #1524's own AC's first case: a first create.
func TestIssueWorker_FirstCreateOpensAnIssueAndWritesTheNumberBack(t *testing.T) {
	db := testdb.New(t)
	staffID := testdb.SeedStaff(t, db, "feedback-worker-first-create")
	feedbackID := seedStaffFeedbackRow(t, db, staffID, feedback.KindNotWorking, "/clients/[clientId]", "")
	enqueueIssueOutbox(t, db, feedbackID)

	creator := feedback.NewFakeIssueCreator()
	worker := feedback.IssueWorker{Creator: creator, AppBaseURL: testIssueAppBaseURL, Now: time.Now}
	runIssueWorker(t, db, worker)

	if status := readIssueOutboxStatus(t, db, feedbackID); status != statusSent {
		t.Fatalf("outbox status = %q, want sent", status)
	}
	number := readIssueNumber(t, db, feedbackID)
	if !number.Valid {
		t.Fatal("issue_number is NULL, want set")
	}
	issue, ok := creator.Issues[int(number.Int64)]
	if !ok {
		t.Fatalf("no fake issue #%d", number.Int64)
	}
	if want := "Something is not working: /clients/[clientId]"; issue.Title != want {
		t.Errorf("title = %q, want %q", issue.Title, want)
	}
	if len(issue.Labels) != 1 || issue.Labels[0] != testNotWorkingLabel {
		t.Errorf("labels = %v, want [%q]", issue.Labels, testNotWorkingLabel)
	}
	if !strings.Contains(issue.Body, "/feedback/"+feedbackID) {
		t.Errorf("body = %q, want the founder read page link", issue.Body)
	}
	if !strings.Contains(issue.Body, "Role: Staff") {
		t.Errorf("body = %q, want Role: Staff for a sender with no Practice in context", issue.Body)
	}
}

// TestIssueWorker_RetryAdoptsAnIssueFoundByItsMarker is #1524's own AC's
// second case: a retry that finds its marker creates nothing. It
// simulates the exact crash window #1500's research names -- GitHub
// already answered 201, but the row is still pending -- by seeding the
// fake with an issue that already carries the marker before the worker
// ever runs.
func TestIssueWorker_RetryAdoptsAnIssueFoundByItsMarker(t *testing.T) {
	db := testdb.New(t)
	staffID := testdb.SeedStaff(t, db, "feedback-worker-retry")
	feedbackID := seedStaffFeedbackRow(t, db, staffID, feedback.KindIdeaOrRequest, "/account", "")
	enqueueIssueOutbox(t, db, feedbackID)

	creator := feedback.NewFakeIssueCreator()
	existingNumber, err := creator.CreateIssue(t.Context(), "stale title",
		"stale body\n\n<!-- doula-cloud-feedback-id: "+feedbackID+" -->\n")
	if err != nil {
		t.Fatalf("seed existing issue: %v", err)
	}

	worker := feedback.IssueWorker{Creator: creator, AppBaseURL: testIssueAppBaseURL, Now: time.Now}
	runIssueWorker(t, db, worker)

	if status := readIssueOutboxStatus(t, db, feedbackID); status != statusSent {
		t.Fatalf("outbox status = %q, want sent", status)
	}
	number := readIssueNumber(t, db, feedbackID)
	if !number.Valid || int(number.Int64) != existingNumber {
		t.Fatalf("issue_number = %v, want %d -- adopted, not a second issue", number, existingNumber)
	}
	if len(creator.Issues) != 1 {
		t.Fatalf("issues = %d, want 1 -- no second issue opened", len(creator.Issues))
	}
	if labels := creator.Issues[existingNumber].Labels; len(labels) != 1 || labels[0] != testIdeaOrRequestLabel {
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
	db := testdb.New(t)
	staffID := testdb.SeedStaff(t, db, "feedback-worker-labels-retry")
	feedbackID := seedStaffFeedbackRow(t, db, staffID, feedback.KindNotWorking, "/account", "")
	enqueueIssueOutbox(t, db, feedbackID)

	creator := feedback.NewFakeIssueCreator()
	creator.AddLabelsErr = errors.New("github is down")
	worker := feedback.IssueWorker{Creator: creator, AppBaseURL: testIssueAppBaseURL, Now: time.Now}
	runIssueWorker(t, db, worker)

	if status := readIssueOutboxStatus(t, db, feedbackID); status != statusPending {
		t.Fatalf("outbox status = %q, want pending after the failed labels call", status)
	}
	if len(creator.Issues) != 1 {
		t.Fatalf("issues = %d, want 1 -- the create itself succeeded", len(creator.Issues))
	}

	creator.AddLabelsErr = nil
	makeIssueOutboxRowDue(t, db, feedbackID)
	runIssueWorker(t, db, worker)

	if status := readIssueOutboxStatus(t, db, feedbackID); status != statusSent {
		t.Fatalf("outbox status = %q, want sent after the retry", status)
	}
	if len(creator.Issues) != 1 {
		t.Fatalf("issues = %d, want 1 -- the retry adopts, it opens no second issue", len(creator.Issues))
	}
	number := readIssueNumber(t, db, feedbackID)
	if !number.Valid {
		t.Fatal("issue_number is NULL, want the adopted issue's number")
	}
	issue, ok := creator.Issues[int(number.Int64)]
	if !ok {
		t.Fatalf("no fake issue #%d", number.Int64)
	}
	if len(issue.Labels) != 1 || issue.Labels[0] != testNotWorkingLabel {
		t.Errorf("labels = %v, want [%q] -- the retry adds the label the first attempt could not", issue.Labels, testNotWorkingLabel)
	}
}

// TestIssueWorker_GitHubErrorOnAddLabelsWhileAdoptingLeavesTheRowPending
// is the adopt path's own half of the labels-call failure: the row stays
// pending and the issue number is not written, the same as on the create
// path, so a later retry still owns the label.
func TestIssueWorker_GitHubErrorOnAddLabelsWhileAdoptingLeavesTheRowPending(t *testing.T) {
	db := testdb.New(t)
	staffID := testdb.SeedStaff(t, db, "feedback-worker-adopt-labels-error")
	feedbackID := seedStaffFeedbackRow(t, db, staffID, feedback.KindSomethingElse, "/account", "")
	enqueueIssueOutbox(t, db, feedbackID)

	creator := feedback.NewFakeIssueCreator()
	if _, err := creator.CreateIssue(t.Context(), "stale title",
		"stale body\n\n<!-- doula-cloud-feedback-id: "+feedbackID+" -->\n"); err != nil {
		t.Fatalf("seed existing issue: %v", err)
	}
	creator.AddLabelsErr = errors.New("github is down")

	worker := feedback.IssueWorker{Creator: creator, AppBaseURL: testIssueAppBaseURL, Now: time.Now}
	runIssueWorker(t, db, worker)

	if status := readIssueOutboxStatus(t, db, feedbackID); status != statusPending {
		t.Fatalf("outbox status = %q, want pending after a GitHub error", status)
	}
	if number := readIssueNumber(t, db, feedbackID); number.Valid {
		t.Fatalf("issue_number = %v, want NULL", number)
	}
	if len(creator.Issues) != 1 {
		t.Fatalf("issues = %d, want 1 -- no second issue opened", len(creator.Issues))
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

// TestIssueWorker_GitHubErrorOnCreateLeavesTheRowPending is #1524's own
// AC's third case: a GitHub error leaves the row pending.
func TestIssueWorker_GitHubErrorOnCreateLeavesTheRowPending(t *testing.T) {
	db := testdb.New(t)
	staffID := testdb.SeedStaff(t, db, "feedback-worker-create-error")
	feedbackID := seedStaffFeedbackRow(t, db, staffID, feedback.KindSomethingElse, "/account", "")
	enqueueIssueOutbox(t, db, feedbackID)

	creator := feedback.NewFakeIssueCreator()
	creator.CreateIssueErr = errors.New("github is down")
	worker := feedback.IssueWorker{Creator: creator, AppBaseURL: testIssueAppBaseURL, Now: time.Now}
	runIssueWorker(t, db, worker)

	if status := readIssueOutboxStatus(t, db, feedbackID); status != statusPending {
		t.Fatalf("outbox status = %q, want pending after a GitHub error", status)
	}
	if number := readIssueNumber(t, db, feedbackID); number.Valid {
		t.Fatalf("issue_number = %v, want NULL", number)
	}
}

// TestIssueWorker_GitHubErrorOnListLeavesTheRowPending covers the other
// call that can fail before a create is even attempted.
func TestIssueWorker_GitHubErrorOnListLeavesTheRowPending(t *testing.T) {
	db := testdb.New(t)
	staffID := testdb.SeedStaff(t, db, "feedback-worker-list-error")
	feedbackID := seedStaffFeedbackRow(t, db, staffID, feedback.KindSomethingElse, "/account", "")
	enqueueIssueOutbox(t, db, feedbackID)

	creator := feedback.NewFakeIssueCreator()
	creator.ListIssuesErr = errors.New("github is down")
	worker := feedback.IssueWorker{Creator: creator, AppBaseURL: testIssueAppBaseURL, Now: time.Now}
	runIssueWorker(t, db, worker)

	if status := readIssueOutboxStatus(t, db, feedbackID); status != statusPending {
		t.Fatalf("outbox status = %q, want pending after a GitHub error", status)
	}
	if len(creator.Issues) != 0 {
		t.Fatalf("issues = %d, want 0 -- no create attempted while listing fails", len(creator.Issues))
	}
}

// TestIssueWorker_GitHubErrorOnAddLabelsLeavesTheRowPending covers the
// window between a create succeeding and its labels call failing: the
// row stays pending and the issue number is not written yet, so a retry
// still owns finishing the job.
func TestIssueWorker_GitHubErrorOnAddLabelsLeavesTheRowPending(t *testing.T) {
	db := testdb.New(t)
	staffID := testdb.SeedStaff(t, db, "feedback-worker-labels-error")
	feedbackID := seedStaffFeedbackRow(t, db, staffID, feedback.KindSomethingElse, "/account", "")
	enqueueIssueOutbox(t, db, feedbackID)

	creator := feedback.NewFakeIssueCreator()
	creator.AddLabelsErr = errors.New("github is down")
	worker := feedback.IssueWorker{Creator: creator, AppBaseURL: testIssueAppBaseURL, Now: time.Now}
	runIssueWorker(t, db, worker)

	if status := readIssueOutboxStatus(t, db, feedbackID); status != statusPending {
		t.Fatalf("outbox status = %q, want pending after a GitHub error", status)
	}
	if number := readIssueNumber(t, db, feedbackID); number.Valid {
		t.Fatalf("issue_number = %v, want NULL", number)
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
	feedbackID := seedStaffFeedbackRow(t, db, staffID, feedback.KindNotWorking, "/account", sentinel)
	enqueueIssueOutbox(t, db, feedbackID)

	creator := feedback.NewFakeIssueCreator()
	worker := feedback.IssueWorker{Creator: creator, AppBaseURL: testIssueAppBaseURL, Now: time.Now}
	runIssueWorker(t, db, worker)

	number := readIssueNumber(t, db, feedbackID)
	issue := creator.Issues[int(number.Int64)]
	if strings.Contains(issue.Title, sentinel) || strings.Contains(issue.Body, sentinel) {
		t.Fatalf("issue leaked the free text: title=%q body=%q", issue.Title, issue.Body)
	}
	if strings.Contains(issue.Body, "Ada Lovelace") {
		t.Fatal("issue body carries the sender's typed text")
	}
}

// TestIssueWorker_PortalSenderReportsClientAsTheRole is #1501 Q2's own
// rule: a role with no Practice identifies nobody, so a Portal sender's
// role on the issue is always "Client".
func TestIssueWorker_PortalSenderReportsClientAsTheRole(t *testing.T) {
	db := testdb.New(t)
	testdb.SeedPortalAccount(t, db, "feedback-worker-portal", "feedback-worker-portal@example.com")
	feedbackID := seedPortalFeedbackRow(t, db, "feedback-worker-portal", feedback.KindIdeaOrRequest, "/portal")
	enqueueIssueOutbox(t, db, feedbackID)

	creator := feedback.NewFakeIssueCreator()
	worker := feedback.IssueWorker{Creator: creator, AppBaseURL: testIssueAppBaseURL, Now: time.Now}
	runIssueWorker(t, db, worker)

	number := readIssueNumber(t, db, feedbackID)
	issue := creator.Issues[int(number.Int64)]
	if !strings.Contains(issue.Body, "Role: Client") {
		t.Errorf("body = %q, want Role: Client for a Portal sender", issue.Body)
	}
}

// TestIssueWorker_StaffSenderWithRolesReportsThem is the fourth shape:
// a Staff sender at a real Practice reports her actual roles.
func TestIssueWorker_StaffSenderWithRolesReportsThem(t *testing.T) {
	db := testdb.New(t)
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, "feedback-worker-roles", []string{"owner", "admin"}, "employee")
	feedbackID := seedStaffFeedbackRowWithRoles(t, db, staffID, practiceID, feedback.KindNotWorking, "/practices/[practiceId]")
	enqueueIssueOutbox(t, db, feedbackID)

	creator := feedback.NewFakeIssueCreator()
	worker := feedback.IssueWorker{Creator: creator, AppBaseURL: testIssueAppBaseURL, Now: time.Now}
	runIssueWorker(t, db, worker)

	number := readIssueNumber(t, db, feedbackID)
	issue := creator.Issues[int(number.Int64)]
	if !strings.Contains(issue.Body, "Role: owner, admin") {
		t.Errorf("body = %q, want Role: owner, admin", issue.Body)
	}
}

// TestIssueWorker_ZeroRoleMembershipReportsNoRoleName is the fifth
// shape, distinct from both "Staff" (no Practice in context, roles
// NULL) and a real role list: a Staff sender at a real Practice whose
// membership carries zero roles reports no role name at all, per
// feedbackRole's own doc comment -- "Staff" would misstate that a
// Practice is in context.
func TestIssueWorker_ZeroRoleMembershipReportsNoRoleName(t *testing.T) {
	db := testdb.New(t)
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, "feedback-worker-zero-role", nil, "employee")
	feedbackID := seedStaffFeedbackRowZeroRoles(t, db, staffID, practiceID)
	enqueueIssueOutbox(t, db, feedbackID)

	creator := feedback.NewFakeIssueCreator()
	worker := feedback.IssueWorker{Creator: creator, AppBaseURL: testIssueAppBaseURL, Now: time.Now}
	runIssueWorker(t, db, worker)

	number := readIssueNumber(t, db, feedbackID)
	issue := creator.Issues[int(number.Int64)]
	if !strings.Contains(issue.Body, "Role: \n") {
		t.Errorf("body = %q, want an empty Role: line -- a zero-role membership names no role, and must not read as \"Staff\"", issue.Body)
	}
	if strings.Contains(issue.Body, "Role: Staff") {
		t.Error("body reports Role: Staff for a zero-role membership -- that misstates that a Practice is in context")
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
	db := testdb.New(t)
	staffID := testdb.SeedStaff(t, db, "feedback-worker-endpoint")
	feedbackID := seedStaffFeedbackRow(t, db, staffID, feedback.KindNotWorking, "/account", "")
	enqueueIssueOutbox(t, db, feedbackID)

	creator := feedback.NewFakeIssueCreator()
	worker := feedback.IssueWorker{Creator: creator, AppBaseURL: testIssueAppBaseURL, Now: time.Now}
	mux := http.NewServeMux()
	mux.Handle("POST /internal/feedback/process-issue-outbox",
		outbox.ProcessHandler(db.App, worker, internalauth.FromSecret("correct-secret"), outbox.NotificationDoor))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/internal/feedback/process-issue-outbox", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("X-Internal-Secret", "correct-secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if status := readIssueOutboxStatus(t, db, feedbackID); status != statusSent {
		t.Fatalf("outbox status = %q, want sent -- app_runtime must reach feedback through the door", status)
	}
	if number := readIssueNumber(t, db, feedbackID); !number.Valid {
		t.Fatal("issue_number is NULL, want set through the door's UPDATE policy")
	}
}
