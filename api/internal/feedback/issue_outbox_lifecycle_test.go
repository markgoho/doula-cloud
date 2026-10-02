package feedback_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/feedback"
	"doula-cloud/api/internal/internalauth"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/testdb"
)

// errGitHubDown is the one failure every GitHub-error case here injects.
var errGitHubDown = errors.New("github is down")

func newTestIssueWorker(creator feedback.IssueCreator) feedback.IssueWorker {
	return feedback.IssueWorker{Creator: creator, AppBaseURL: testIssueAppBaseURL, Now: time.Now}
}

// TestIssueWorker_CloseAsErasedClosesTheIssueAndAddsTheErasedLabel is
// #1525's first AC: the outbox's second act closes the issue and adds
// the `erased` label, working from the issue number the job itself
// carries -- there is no feedback row left to read it from.
func TestIssueWorker_CloseAsErasedClosesTheIssueAndAddsTheErasedLabel(t *testing.T) {
	db := testdb.New(t)
	creator := feedback.NewFakeIssueCreator()
	number := seedOpenedIssue(t, creator, "body")
	outboxID := seedCloseJob(t, db, number)

	runIssueWorker(t, db, newTestIssueWorker(creator))

	if status := readOutboxStatusByID(t, db, outboxID); status != statusSent {
		t.Fatalf("outbox status = %q, want sent", status)
	}
	issue := creator.Issues[number]
	if !issue.Closed {
		t.Error("issue is still open, want closed")
	}
	if !hasLabel(issue, testErasedLabel) {
		t.Errorf("labels = %v, want %q among them", issue.Labels, testErasedLabel)
	}
	if !hasLabel(issue, testNotWorkingLabel) {
		t.Errorf("labels = %v, want the kind label kept", issue.Labels)
	}
	if len(creator.Issues) != 1 {
		t.Fatalf("issues = %d, want 1 -- a close job opens nothing", len(creator.Issues))
	}
}

// TestIssueWorker_CloseAsErasedGitHubErrorLeavesTheJobPending covers
// both GitHub calls a close makes: either one failing leaves the job
// pending for a retry, and both calls are safe to repeat.
func TestIssueWorker_CloseAsErasedGitHubErrorLeavesTheJobPending(t *testing.T) {
	for name, breakIt := range map[string]func(*feedback.FakeIssueCreator){
		"add labels": func(c *feedback.FakeIssueCreator) { c.AddLabelsErr = errGitHubDown },
		"close":      func(c *feedback.FakeIssueCreator) { c.CloseIssueErr = errGitHubDown },
	} {
		t.Run(name, func(t *testing.T) {
			db := testdb.New(t)
			creator := feedback.NewFakeIssueCreator()
			number := seedOpenedIssue(t, creator, "body")
			outboxID := seedCloseJob(t, db, number)
			breakIt(creator)

			runIssueWorker(t, db, newTestIssueWorker(creator))

			if status := readOutboxStatusByID(t, db, outboxID); status != statusPending {
				t.Fatalf("outbox status = %q, want pending after a GitHub error", status)
			}
			if creator.Issues[number].Closed {
				t.Fatal("issue is closed, want it still open after a GitHub error")
			}
		})
	}
}

// TestIssueWorker_OpenJobWhosePieceIsGoneOpensNoIssue is #1525's third
// AC: a piece erased before its issue was opened opens no issue, and the
// pending open job is marked done.
func TestIssueWorker_OpenJobWhosePieceIsGoneOpensNoIssue(t *testing.T) {
	db := testdb.New(t)
	staffID := testdb.SeedStaff(t, db, "feedback-worker-gone")
	feedbackID := seedStaffFeedbackRow(t, db, staffID, feedback.KindNotWorking, "/account", "")
	enqueueIssueOutbox(t, db, feedbackID)
	deleteFeedbackRow(t, db, feedbackID)

	creator := feedback.NewFakeIssueCreator()
	runIssueWorker(t, db, newTestIssueWorker(creator))

	if status := readIssueOutboxStatus(t, db, feedbackID); status != statusSent {
		t.Fatalf("outbox status = %q, want sent -- the job is done once its piece is gone", status)
	}
	if len(creator.Issues) != 0 {
		t.Fatalf("issues = %d, want 0 -- an erased piece opens no issue", len(creator.Issues))
	}
}

// TestIssueWorker_OpenJobWhosePieceIsGoneClosesAnIssueAnEarlierAttemptOpened
// is #1500's crash window meeting an Erasure: GitHub opened the issue,
// the attempt failed before the number reached the feedback row, and the
// piece was erased before the retry. The Erasure found no number to
// close, so the retry is the only act that can ever close that issue.
func TestIssueWorker_OpenJobWhosePieceIsGoneClosesAnIssueAnEarlierAttemptOpened(t *testing.T) {
	db := testdb.New(t)
	staffID := testdb.SeedStaff(t, db, "feedback-worker-gone-orphan")
	feedbackID := seedStaffFeedbackRow(t, db, staffID, feedback.KindNotWorking, "/account", "")
	enqueueIssueOutbox(t, db, feedbackID)

	creator := feedback.NewFakeIssueCreator()
	number := seedOpenedIssue(t, creator, markerBody(feedbackID))
	deleteFeedbackRow(t, db, feedbackID)

	runIssueWorker(t, db, newTestIssueWorker(creator))

	if status := readIssueOutboxStatus(t, db, feedbackID); status != statusSent {
		t.Fatalf("outbox status = %q, want sent", status)
	}
	if len(creator.Issues) != 1 {
		t.Fatalf("issues = %d, want 1 -- no second issue opened", len(creator.Issues))
	}
	issue := creator.Issues[number]
	if !issue.Closed || !hasLabel(issue, testErasedLabel) {
		t.Fatalf("issue closed = %v, labels = %v; want closed with the erased label", issue.Closed, issue.Labels)
	}
}

// TestIssueWorker_OpenJobWhosePieceIsGoneStaysPendingOnAGitHubError
// covers both calls the gone-piece path can fail on: the marker lookup,
// and the close of an issue that lookup found.
func TestIssueWorker_OpenJobWhosePieceIsGoneStaysPendingOnAGitHubError(t *testing.T) {
	for name, breakIt := range map[string]func(*feedback.FakeIssueCreator){
		"list":  func(c *feedback.FakeIssueCreator) { c.ListIssuesErr = errGitHubDown },
		"close": func(c *feedback.FakeIssueCreator) { c.CloseIssueErr = errGitHubDown },
	} {
		t.Run(name, func(t *testing.T) {
			db := testdb.New(t)
			staffID := testdb.SeedStaff(t, db, "feedback-worker-gone-error")
			feedbackID := seedStaffFeedbackRow(t, db, staffID, feedback.KindNotWorking, "/account", "")
			enqueueIssueOutbox(t, db, feedbackID)

			creator := feedback.NewFakeIssueCreator()
			seedOpenedIssue(t, creator, markerBody(feedbackID))
			deleteFeedbackRow(t, db, feedbackID)
			breakIt(creator)

			runIssueWorker(t, db, newTestIssueWorker(creator))

			if status := readIssueOutboxStatus(t, db, feedbackID); status != statusPending {
				t.Fatalf("outbox status = %q, want pending after a GitHub error", status)
			}
		})
	}
}

// TestIssueWorker_PieceErasedWhileItsIssueWasBeingOpenedQueuesAClose is
// the race the Erasure cannot see: the worker has claimed the open job
// and is talking to GitHub when the Erasure deletes the piece, finding
// no issue number on it. The worker's own write of the number then
// matches no row, and that is its signal to queue the close itself --
// otherwise the issue it just opened stays open forever.
func TestIssueWorker_PieceErasedWhileItsIssueWasBeingOpenedQueuesAClose(t *testing.T) {
	db := testdb.New(t)
	staffID := testdb.SeedStaff(t, db, "feedback-worker-race")
	feedbackID := seedStaffFeedbackRow(t, db, staffID, feedback.KindNotWorking, "/account", "")
	enqueueIssueOutbox(t, db, feedbackID)

	creator := feedback.NewFakeIssueCreator()
	creator.AfterCreate = func(int) { deleteFeedbackRow(t, db, feedbackID) }
	worker := newTestIssueWorker(creator)
	runIssueWorker(t, db, worker)

	if status := readIssueOutboxStatus(t, db, feedbackID); status != statusSent {
		t.Fatalf("open job status = %q, want sent", status)
	}
	if len(creator.Issues) != 1 {
		t.Fatalf("issues = %d, want 1", len(creator.Issues))
	}
	var queued int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM feedback_issue_outbox WHERE act = 'close_erased' AND status = 'pending' AND issue_number = 1`,
	).Scan(&queued); err != nil {
		t.Fatalf("count close jobs: %v", err)
	}
	if queued != 1 {
		t.Fatalf("pending close jobs for issue #1 = %d, want 1", queued)
	}

	creator.AfterCreate = nil
	runIssueWorker(t, db, worker)
	if issue := creator.Issues[1]; !issue.Closed || !hasLabel(issue, testErasedLabel) {
		t.Fatalf("issue closed = %v, labels = %v; want closed with the erased label on the next run", issue.Closed, issue.Labels)
	}
}

// TestIssueWorker_RetentionDeletesAPieceAfter24Months is #1525's
// retention ACs: the worker's own run deletes a piece once the clock
// passes 24 months from when it was sent, leaves it at 23 months, and
// leaves its GitHub issue exactly as it is. The clock moves through the
// worker's injected Now (#773's seam), never time.Now().
func TestIssueWorker_RetentionDeletesAPieceAfter24Months(t *testing.T) {
	db := testdb.New(t)
	staffID := testdb.SeedStaff(t, db, "feedback-worker-retention")
	sentAt := time.Date(2026, time.November, 3, 14, 30, 0, 0, time.UTC)
	feedbackID := seedStaffFeedbackRowSentAt(t, db, staffID, sentAt)

	creator := feedback.NewFakeIssueCreator()
	number := seedOpenedIssue(t, creator, "body")
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE feedback SET issue_number = $1 WHERE id = $2`, number, feedbackID,
	); err != nil {
		t.Fatalf("write issue number: %v", err)
	}
	labelCallsBefore := len(creator.LabelCalls)

	now := sentAt.AddDate(0, 23, 0)
	worker := feedback.IssueWorker{Creator: creator, AppBaseURL: testIssueAppBaseURL, Now: func() time.Time { return now }}

	runIssueWorker(t, db, worker)
	if !feedbackRowExists(t, db, feedbackID) {
		t.Fatal("piece is gone at 23 months, want it kept")
	}

	now = sentAt.AddDate(0, 24, 0)
	runIssueWorker(t, db, worker)
	if !feedbackRowExists(t, db, feedbackID) {
		t.Fatal("piece is gone at exactly 24 months, want it kept until the clock passes that instant")
	}

	now = sentAt.AddDate(0, 24, 0).Add(time.Second)
	runIssueWorker(t, db, worker)
	if feedbackRowExists(t, db, feedbackID) {
		t.Fatal("piece is still there past 24 months, want it deleted")
	}

	if creator.Issues[number].Closed {
		t.Error("issue was closed, want retention to leave it as it is")
	}
	if len(creator.LabelCalls) != labelCallsBefore {
		t.Errorf("label calls = %d, want %d -- retention adds no label", len(creator.LabelCalls), labelCallsBefore)
	}
	var jobs int
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM feedback_issue_outbox`).Scan(&jobs); err != nil {
		t.Fatalf("count outbox rows: %v", err)
	}
	if jobs != 0 {
		t.Fatalf("outbox rows = %d, want 0 -- retention queues no close", jobs)
	}
}

// postProcessIssueOutbox runs worker once through outbox.ProcessHandler
// against db.App, the app_runtime role -- the door every RLS claim in
// 00120 and 00121 is actually proved through.
func postProcessIssueOutbox(t *testing.T, db *testdb.DB, worker feedback.IssueWorker) {
	t.Helper()
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
}

// TestProcessIssueOutboxHandler_RetentionAndCloseRunBehindTheDoor proves
// 00121's own grant and policy, not just the worker's Go logic: as
// app_runtime behind the notification door, the retention DELETE reaches
// an expired piece, and a close job -- which names no feedback row at
// all -- is still claimed and performed.
func TestProcessIssueOutboxHandler_RetentionAndCloseRunBehindTheDoor(t *testing.T) {
	db := testdb.New(t)
	staffID := testdb.SeedStaff(t, db, "feedback-worker-door-retention")
	expired := seedStaffFeedbackRowSentAt(t, db, staffID, time.Now().AddDate(0, -25, 0))
	kept := seedStaffFeedbackRowSentAt(t, db, staffID, time.Now().AddDate(0, -23, 0))

	creator := feedback.NewFakeIssueCreator()
	number := seedOpenedIssue(t, creator, "body")
	outboxID := seedCloseJob(t, db, number)

	postProcessIssueOutbox(t, db, newTestIssueWorker(creator))

	if feedbackRowExists(t, db, expired) {
		t.Fatal("expired piece is still there -- app_runtime must reach the retention DELETE through the door")
	}
	if !feedbackRowExists(t, db, kept) {
		t.Fatal("a piece at 23 months is gone, want it kept")
	}
	if status := readOutboxStatusByID(t, db, outboxID); status != statusSent {
		t.Fatalf("close job status = %q, want sent", status)
	}
	if !creator.Issues[number].Closed {
		t.Fatal("issue is still open, want the close job performed behind the door")
	}
}

// TestRLS_FeedbackDeleteIsRefusedWithoutTheDoor is the other half of the
// retention grant: app_runtime now holds DELETE on feedback, and the
// policy is what keeps a Staff or Client transaction -- which never
// opens the worker's door -- from deleting a piece with it.
func TestRLS_FeedbackDeleteIsRefusedWithoutTheDoor(t *testing.T) {
	db := testdb.New(t)
	staffID := testdb.SeedStaff(t, db, "feedback-delete-no-door")
	feedbackID := seedStaffFeedbackRow(t, db, staffID, feedback.KindNotWorking, "/account", "")

	res, err := db.App.ExecContext(t.Context(), `DELETE FROM feedback WHERE id = $1`, feedbackID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if n, _ := res.RowsAffected(); n != 0 {
		t.Fatalf("rows deleted = %d, want 0 without the door", n)
	}
	if !feedbackRowExists(t, db, feedbackID) {
		t.Fatal("piece is gone, want the policy to refuse a delete outside the worker's door")
	}
}
