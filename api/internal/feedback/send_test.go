package feedback_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/feedback"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

const sendText = "a send that reaches the issue worker"

type countingEnqueuer struct{ n int }

func (c *countingEnqueuer) Enqueue(context.Context, tasknudge.OutboxType) error {
	c.n++
	return nil
}

func TestSend_SavesCommitsAndNudgesTheIssueWorker(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "feedback-send-staff-uid"
	staffID := testdb.SeedStaff(t, db, identityUID)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_identity_uid', $1, true)`, identityUID); err != nil {
		t.Fatalf("set identity: %v", err)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/feedback", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15")
	enq := &countingEnqueuer{}
	rec := httptest.NewRecorder()
	in := feedback.ClientInput{Kind: feedback.KindIdeaOrRequest, Text: sendText, AppBuild: testAppBuild, ScreenWidth: 390}
	in.Page.URL, in.Page.Route.ID = testAccountPage, testAccountPage

	id, ok := feedback.Send(rec, req, tx, enq, in, feedback.Row{StaffID: &staffID})
	if !ok || id == "" {
		t.Fatalf("Send = (%q, %v), want an id and ok", id, ok)
	}
	if enq.n != 1 {
		t.Fatalf("nudges = %d, want 1", enq.n)
	}

	var kind, text, browser string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT kind::text, text, browser FROM feedback WHERE id = $1`, id,
	).Scan(&kind, &text, &browser); err != nil {
		t.Fatalf("read committed feedback: %v", err)
	}
	if kind != feedback.KindIdeaOrRequest || text != sendText || browser == "" {
		t.Fatalf("row = %q %q %q, want what the client wrote and a parsed browser", kind, text, browser)
	}
}
