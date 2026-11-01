package feedback

import (
	"database/sql"
	"encoding/json"
	"testing"
)

func TestIssueOf_ReportsEachStateOfAPiecesIssue(t *testing.T) {
	failed := func(status string, attempts int, reason string) OpenJob {
		lastError := sql.NullString{}
		if attempts > 0 {
			lastError = sql.NullString{String: "401 Bad credentials", Valid: true}
		}
		return OpenJob{
			Status:           sql.NullString{String: status, Valid: true},
			Attempts:         attempts,
			LastError:        lastError,
			DeadLetterReason: sql.NullString{String: reason, Valid: reason != ""},
		}
	}
	for _, tc := range []struct {
		name   string
		repo   string
		number sql.NullInt64
		job    OpenJob
		want   string
	}{
		{"opened, with a repository", "acme/feedback", sql.NullInt64{Int64: 4, Valid: true}, OpenJob{}, `{"state":"opened","number":4,"url":"https://github.com/acme/feedback/issues/4","attempts":0}`},
		{"opened, no repository", "", sql.NullInt64{Int64: 4, Valid: true}, OpenJob{}, `{"state":"opened","number":4,"attempts":0}`},
		{"waiting", "", sql.NullInt64{}, failed("pending", 0, ""), `{"state":"pending","attempts":0}`},
		{"no job at all", "", sql.NullInt64{}, OpenJob{}, `{"state":"pending","attempts":0}`},
		{"retrying", "", sql.NullInt64{}, failed("pending", 2, ""), `{"state":"retrying","attempts":2,"lastError":"401 Bad credentials"}`},
		{"dead-lettered", "", sql.NullInt64{}, failed("dead_lettered", 8, "retries_exhausted"), `{"state":"dead_lettered","attempts":8,"lastError":"401 Bad credentials","deadLetterReason":"retries_exhausted"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := json.Marshal(IssueOf(tc.repo, tc.number, tc.job))
			if string(got) != tc.want {
				t.Errorf("IssueOf = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestRoleText_IsTheSentenceTheIssueCarries(t *testing.T) {
	if got := RoleText(true, sql.NullString{}); got != "Client" {
		t.Errorf("portal = %q, want Client", got)
	}
	if got := RoleText(false, sql.NullString{String: "owner,doula", Valid: true}); got != "owner, doula" {
		t.Errorf("staff = %q, want owner, doula", got)
	}
}
