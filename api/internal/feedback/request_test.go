package feedback_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"doula-cloud/api/internal/feedback"
	"doula-cloud/api/internal/testdb"
)

func TestRequireValid(t *testing.T) {
	const kindNeeded, textTooLong = "pick a kind", "shorten it"
	cases := []struct {
		name       string
		kind, text string
		want       bool
		wantStatus int
	}{
		{"valid", feedback.KindNotWorking, "hello", true, http.StatusOK},
		{"unknown kind", "nope", "hello", false, http.StatusBadRequest},
		{"text over the limit", feedback.KindNotWorking, strings.Repeat("a", feedback.MaxTextRunes+1), false, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			if got := feedback.RequireValid(w, tc.kind, tc.text, kindNeeded, textTooLong); got != tc.want {
				t.Fatalf("RequireValid = %v, want %v", got, tc.want)
			}
			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tc.wantStatus)
			}
		})
	}
}

// TestSave_InsertsTheRowAndQueuesItsIssue proves Save writes the Feedback
// row and its issue-outbox row together.
func TestSave_InsertsTheRowAndQueuesItsIssue(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "feedback-save-staff-uid"
	staffID := testdb.SeedStaff(t, db, identityUID)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_identity_uid', $1, true)`, identityUID); err != nil {
		t.Fatalf("set identity: %v", err)
	}

	id := uuid.NewString()
	row := feedback.Row{
		ID: id, Kind: feedback.KindIdeaOrRequest, Text: "it would help if",
		PageURL: testAccountPage, RouteID: testAccountPage, AppBuild: testAppBuild, ScreenWidth: 390,
		Browser: "Safari 18", StaffID: &staffID, SentAt: time.Now(),
	}
	w := httptest.NewRecorder()
	if !feedback.Save(t.Context(), w, tx, row) {
		t.Fatalf("Save = false, response %d", w.Code)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var count int
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM feedback WHERE id = $1`, id).Scan(&count); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if count != 1 {
		t.Fatalf("feedback rows = %d, want 1", count)
	}
}
