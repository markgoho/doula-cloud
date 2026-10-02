package staffauth_test

import (
	"encoding/base64"
	"net/http"
	"slices"
	"testing"
	"time"

	"doula-cloud/api/internal/feedback"
)

// A piece past its retention is gone, the read page answers 404 for it,
// and its read rows went with it (#1526: "deleted with its piece").
func TestReadFeedback_APiecePastRetentionIsNotFoundAndItsReadsAreGone(t *testing.T) {
	f := newFounderFixture(t)
	id := seedPiece(t, f.db, seededPiece{staffID: f.founderID, sentAt: readAt.AddDate(0, -25, 0)})

	if resp := f.get(t, "/api/staff/feedback/"+id); resp.status != http.StatusOK {
		t.Fatalf("status before retention = %d, want %d", resp.status, http.StatusOK)
	}
	if got := reads(t, f.db, id); len(got) != 1 {
		t.Fatalf("feedback_reads = %v, want the one read", got)
	}

	worker := feedback.IssueWorker{Creator: feedback.NewFakeIssueCreator(), Now: func() time.Time { return readAt }}
	tx, err := f.db.Admin.BeginTx(t.Context(), nil)
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

	resp := f.get(t, "/api/staff/feedback/"+id)
	if resp.status != http.StatusNotFound {
		t.Fatalf("status past retention = %d, want %d", resp.status, http.StatusNotFound)
	}
	if got := reads(t, f.db, id); len(got) != 0 {
		t.Fatalf("feedback_reads = %v, want none once the piece is gone", got)
	}
}

func listIDs(page feedback.SummaryPage) []string {
	ids := make([]string, len(page.Items))
	for i, item := range page.Items {
		ids[i] = item.ID
	}
	return ids
}

// The list is newest first and a page at a time (docs/api-design.md
// section 4): the cursor carries on exactly where the page stopped.
func TestListFeedback_IsNewestFirstWithACursor(t *testing.T) {
	f := newFounderFixture(t)
	oldest := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	var newestFirst []string
	for i := range feedback.ReadPageSize + 1 {
		id := seedPiece(t, f.db, seededPiece{staffID: f.founderID, sentAt: oldest.Add(time.Duration(i) * time.Hour)})
		newestFirst = append([]string{id}, newestFirst...)
	}

	first := decodeBody[feedback.SummaryPage](t, f.get(t, "/api/staff/feedback"))
	if !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page hasMore = %v, nextCursor = %v, want more to follow", first.HasMore, first.NextCursor)
	}
	if got, want := listIDs(first), newestFirst[:feedback.ReadPageSize]; !slices.Equal(got, want) {
		t.Fatalf("first page = %v, want the %d newest, newest first", got, feedback.ReadPageSize)
	}

	second := decodeBody[feedback.SummaryPage](t, f.get(t, "/api/staff/feedback?cursor="+*first.NextCursor))
	if second.HasMore || second.NextCursor != nil {
		t.Fatalf("second page hasMore = %v, want the end of the list", second.HasMore)
	}
	if got, want := listIDs(second), newestFirst[feedback.ReadPageSize:]; !slices.Equal(got, want) {
		t.Fatalf("second page = %v, want %v", got, want)
	}
}

// The dead-letter list (#1526, #1500): `?issue=unopened` holds every
// piece whose issue open has dead-lettered or is still pending after a
// failed attempt, with the job's own error -- and none whose issue
// opened or whose job has not failed.
func TestListFeedback_UnopenedListsDeadLetteredAndRetryingPieces(t *testing.T) {
	f := newFounderFixture(t)
	at := func(day int) time.Time { return time.Date(2026, 10, day, 12, 0, 0, 0, time.UTC) }
	const badCredentials = "feedback: create issue: 401 Bad credentials"

	opened := seedPiece(t, f.db, seededPiece{staffID: f.founderID, issueNumber: 7, sentAt: at(1)})
	seedOpenJob(t, f.db, opened, "sent", 0, "")
	queued := seedPiece(t, f.db, seededPiece{staffID: f.founderID, sentAt: at(2)})
	seedOpenJob(t, f.db, queued, "pending", 0, "")
	retrying := seedPiece(t, f.db, seededPiece{staffID: f.founderID, sentAt: at(3)})
	seedOpenJob(t, f.db, retrying, "pending", 2, badCredentials)
	dead := seedPiece(t, f.db, seededPiece{staffID: f.founderID, sentAt: at(4)})
	seedOpenJob(t, f.db, dead, "dead_lettered", 5, badCredentials)

	unopened := decodeBody[feedback.SummaryPage](t, f.get(t, "/api/staff/feedback?issue=unopened"))
	if got, want := listIDs(unopened), []string{dead, retrying}; !slices.Equal(got, want) {
		t.Fatalf("unopened list = %v, want the dead-lettered piece then the retrying one", got)
	}
	wantIssues := []feedback.IssueStatus{
		{State: feedback.IssueDeadLettered, Attempts: 5, LastError: badCredentials},
		{State: feedback.IssueRetrying, Attempts: 2, LastError: badCredentials},
	}
	for i, want := range wantIssues {
		if got := unopened.Items[i].Issue; got != want {
			t.Errorf("unopened[%d].issue = %+v, want %+v", i, got, want)
		}
	}

	// The whole list still holds all four, each in its own state.
	all := decodeBody[feedback.SummaryPage](t, f.get(t, "/api/staff/feedback"))
	if got, want := listIDs(all), []string{dead, retrying, queued, opened}; !slices.Equal(got, want) {
		t.Fatalf("list = %v, want all four, newest first", got)
	}
	var states []string
	for _, item := range all.Items {
		states = append(states, item.Issue.State)
	}
	wantStates := []string{feedback.IssueDeadLettered, feedback.IssueRetrying, feedback.IssuePending, feedback.IssueOpened}
	if !slices.Equal(states, wantStates) {
		t.Fatalf("states = %v, want %v", states, wantStates)
	}
	last := all.Items[3]
	if got := last.Issue; got.Number == nil || *got.Number != 7 || got.URL == nil {
		t.Errorf("opened issue = %+v, want number 7 and a link", got)
	}
	if last.Kind != feedback.KindNotWorking || last.RouteID != "/practices/[practiceId]/clients/[clientId]" || !last.SentAt.Equal(at(1)) {
		t.Errorf("list row = %+v, want its kind, route pattern and time sent", last)
	}
}

func TestListFeedback_RefusesAnUnknownFilterAndABrokenCursor(t *testing.T) {
	f := newFounderFixture(t)
	notAnID := base64.URLEncoding.EncodeToString([]byte("2026-10-01T00:00:00Z|not-an-id"))

	for _, query := range []string{"?issue=everything", "?cursor=%25%25", "?cursor=" + notAnID} {
		if resp := f.get(t, "/api/staff/feedback"+query); resp.status != http.StatusBadRequest {
			t.Errorf("GET %s status = %d, want %d", query, resp.status, http.StatusBadRequest)
		}
	}
}
