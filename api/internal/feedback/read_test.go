package feedback_test

import (
	"slices"
	"testing"
	"time"

	"doula-cloud/api/internal/feedback"
	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/testdb"
)

const readRepo = "markgoho/doula-cloud-feedback"

// The two roles this package's tests seed a membership with, named once
// for goconst.
const (
	testOwnerRole = "owner"
	testAdminRole = "admin"
)

// setOpenJob puts feedbackID's open job in the state a test needs: a
// failed attempt count, an error, and whether the retries are spent.
func setOpenJob(t *testing.T, db *testdb.DB, feedbackID, status string, attempts int, lastError string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO feedback_issue_outbox (feedback_id, status, attempt_count, last_error)
		 VALUES ($1, $2::feedback_issue_outbox_status, $3, NULLIF($4, ''))`,
		feedbackID, status, attempts, lastError,
	); err != nil {
		t.Fatalf("seed open job: %v", err)
	}
}

func summaryIDs(page feedback.SummaryPage) []string {
	ids := make([]string, len(page.Items))
	for i, item := range page.Items {
		ids[i] = item.ID
	}
	return ids
}

// TestList_PagesNewestFirstBehindTheDoor: List reads a page of
// ReadPageSize, newest first, and the cursor it hands back carries on
// from exactly where the page stopped.
func TestList_PagesNewestFirstBehindTheDoor(t *testing.T) {
	db := testdb.New(t)
	const uid = "feedback-list-reader"
	readerID := testdb.SeedStaff(t, db, uid)
	oldest := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	var newestFirst []string
	for i := range feedback.ReadPageSize + 1 {
		id := seedStaffFeedbackRowSentAt(t, db, readerID, oldest.Add(time.Duration(i)*time.Hour))
		newestFirst = append([]string{id}, newestFirst...)
	}
	tx := readerTx(t, db, uid, readerID)

	first, err := feedback.List(t.Context(), tx, readRepo, false, nil)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page hasMore = %v, nextCursor = %v, want more to follow", first.HasMore, first.NextCursor)
	}
	if got := summaryIDs(first); !slices.Equal(got, newestFirst[:feedback.ReadPageSize]) {
		t.Fatalf("first page = %v, want the %d newest, newest first", got, feedback.ReadPageSize)
	}

	after, err := pagecursor.Decode(*first.NextCursor)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	second, err := feedback.List(t.Context(), tx, readRepo, false, &after)
	if err != nil {
		t.Fatalf("list after cursor: %v", err)
	}
	if second.HasMore || second.NextCursor != nil {
		t.Fatalf("second page hasMore = %v, want the end of the list", second.HasMore)
	}
	if got := summaryIDs(second); !slices.Equal(got, newestFirst[feedback.ReadPageSize:]) {
		t.Fatalf("second page = %v, want the one oldest piece", got)
	}
}

// TestList_ReportsEachIssueStateAndNarrowsToTheUnopened: the four states
// a piece's issue can be in, and the narrowing that keeps only the two
// that mean the open has failed.
func TestList_ReportsEachIssueStateAndNarrowsToTheUnopened(t *testing.T) {
	db := testdb.New(t)
	const uid = "feedback-list-states"
	const badCredentials = "feedback: create issue: 401 Bad credentials"
	readerID := testdb.SeedStaff(t, db, uid)
	at := func(day int) time.Time { return time.Date(2026, 10, day, 12, 0, 0, 0, time.UTC) }

	opened := seedStaffFeedbackRowSentAt(t, db, readerID, at(1))
	if _, err := db.Admin.ExecContext(t.Context(), `UPDATE feedback SET issue_number = 7 WHERE id = $1`, opened); err != nil {
		t.Fatalf("set issue number: %v", err)
	}
	setOpenJob(t, db, opened, "sent", 0, "")
	queued := seedStaffFeedbackRowSentAt(t, db, readerID, at(2))
	setOpenJob(t, db, queued, "pending", 0, "")
	retrying := seedStaffFeedbackRowSentAt(t, db, readerID, at(3))
	setOpenJob(t, db, retrying, "pending", 2, badCredentials)
	dead := seedStaffFeedbackRowSentAt(t, db, readerID, at(4))
	setOpenJob(t, db, dead, "dead_lettered", 5, badCredentials)
	// A piece whose send queued no job at all reads as pending, not as a
	// failure: nothing has gone wrong that anybody recorded.
	jobless := seedStaffFeedbackRowSentAt(t, db, readerID, at(5))

	tx := readerTx(t, db, uid, readerID)
	all, err := feedback.List(t.Context(), tx, readRepo, false, nil)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if got, want := summaryIDs(all), []string{jobless, dead, retrying, queued, opened}; !slices.Equal(got, want) {
		t.Fatalf("list = %v, want all five, newest first", got)
	}
	number, url := 7, "https://github.com/markgoho/doula-cloud-feedback/issues/7"
	want := []feedback.IssueStatus{
		{State: feedback.IssuePending},
		{State: feedback.IssueDeadLettered, Attempts: 5, LastError: badCredentials},
		{State: feedback.IssueRetrying, Attempts: 2, LastError: badCredentials},
		{State: feedback.IssuePending},
		{State: feedback.IssueOpened, Number: &number, URL: &url},
	}
	for i, item := range all.Items {
		got := item.Issue
		if got.State != want[i].State || got.Attempts != want[i].Attempts || got.LastError != want[i].LastError {
			t.Errorf("items[%d].issue = %+v, want %+v", i, got, want[i])
		}
	}
	last := all.Items[4].Issue
	if last.Number == nil || *last.Number != number || last.URL == nil || *last.URL != url {
		t.Errorf("opened issue = %+v, want number %d and link %s", last, number, url)
	}

	unopened, err := feedback.List(t.Context(), tx, readRepo, true, nil)
	if err != nil {
		t.Fatalf("list unopened: %v", err)
	}
	if got, want := summaryIDs(unopened), []string{dead, retrying}; !slices.Equal(got, want) {
		t.Fatalf("unopened = %v, want the dead-lettered piece then the retrying one", got)
	}

	// No repository configured: the number is still there, the link is not.
	noRepo, err := feedback.List(t.Context(), tx, "", false, nil)
	if err != nil {
		t.Fatalf("list with no repo: %v", err)
	}
	if got := noRepo.Items[4].Issue; got.Number == nil || got.URL != nil {
		t.Errorf("opened issue with no repo = %+v, want a number and no link", got)
	}
}

// TestGet_ReadsAPieceWithItsSenderAndPractice: every stored field, plus
// the three facts feedback_sender (00122) supplies from tables the
// reader's transaction has no door onto.
func TestGet_ReadsAPieceWithItsSenderAndPractice(t *testing.T) {
	db := testdb.New(t)
	const uid = "feedback-get-reader"
	readerID := testdb.SeedStaff(t, db, uid)
	practiceID := testdb.SeedPractice(t, db, "Riverside Doula Collective")
	senderID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "feedback-get-sender", "Anne-Marie Ochieng", []string{testOwnerRole, testAdminRole}, "employee")
	staffPiece := seedStaffFeedbackRowWithRoles(t, db, senderID, practiceID, feedback.KindIdeaOrRequest, "/practices/[practiceId]")
	portalUID := testdb.PortalUID("feedback-get-portal")
	testdb.SeedPortalAccount(t, db, portalUID, "priya.signin@example.com")
	portalPiece := seedPortalFeedbackRow(t, db, portalUID, feedback.KindSomethingElse, "/portal/engagements")
	tx := readerTx(t, db, uid, readerID)

	got, found, err := feedback.Get(t.Context(), tx, readRepo, staffPiece)
	if err != nil || !found {
		t.Fatalf("get staff piece: found = %v, err = %v", found, err)
	}
	if got.ID != staffPiece || got.Kind != feedback.KindIdeaOrRequest || got.RouteID != "/practices/[practiceId]" {
		t.Errorf("piece = %+v, want its own id, kind and route", got)
	}
	if want := (feedback.Sender{Kind: feedback.SenderStaff, Name: "Anne-Marie Ochieng", Email: "feedback-get-sender@example.com"}); got.Sender != want {
		t.Errorf("sender = %+v, want %+v", got.Sender, want)
	}
	if got.PracticeName != "Riverside Doula Collective" || got.Issue.State != feedback.IssuePending {
		t.Errorf("practice = %q, issue = %+v, want the Practice named and a pending issue", got.PracticeName, got.Issue)
	}

	got, found, err = feedback.Get(t.Context(), tx, readRepo, portalPiece)
	if err != nil || !found {
		t.Fatalf("get portal piece: found = %v, err = %v", found, err)
	}
	if want := (feedback.Sender{Kind: feedback.SenderClient, Email: "priya.signin@example.com"}); got.Sender != want {
		t.Errorf("portal sender = %+v, want %+v", got.Sender, want)
	}
	if got.Role != "Client" || got.PracticeName != "" {
		t.Errorf("portal role = %q, practice = %q, want Client and no Practice", got.Role, got.PracticeName)
	}

	if _, found, err := feedback.Get(t.Context(), tx, readRepo, "00000000-0000-4000-8000-000000000000"); err != nil || found {
		t.Fatalf("get absent piece: found = %v, err = %v, want not found", found, err)
	}
}
