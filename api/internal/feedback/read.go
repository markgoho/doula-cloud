package feedback

import (
	"database/sql"
	"strconv"
)

// The four states the issue of a piece can be in, as ops's Feedback list
// reports them. A piece's own row says only whether the issue exists
// (issue_number); the rest comes from its open job in
// feedback_issue_outbox.
const (
	// IssueOpened: the issue exists and the piece carries its number.
	IssueOpened = "opened"
	// IssuePending: the open job is queued and has not failed.
	IssuePending = "pending"
	// IssueRetrying: the open job has failed at least once and will be
	// tried again (outbox.BackoffSchedule).
	IssueRetrying = "retrying"
	// IssueDeadLettered: the open job stopped. Nothing tries it again.
	IssueDeadLettered = "dead_lettered"
)

// IssueStatus is what ops shows about a piece's GitHub issue.
type IssueStatus struct {
	State string `json:"state"`
	// Number and URL are set once the issue exists. URL is absent where
	// no repository is configured (GITHUB_FEEDBACK_REPO).
	Number *int    `json:"number,omitempty"`
	URL    *string `json:"url,omitempty"`
	// Attempts and LastError are the open job's own bookkeeping. LastError
	// is where a lapsed GITHUB_FEEDBACK_TOKEN shows (#1500).
	Attempts  int    `json:"attempts"`
	LastError string `json:"lastError,omitempty"`
	// DeadLetterReason is why a dead-lettered job stopped (#2474).
	DeadLetterReason string `json:"deadLetterReason,omitempty"`
}

// OpenJob is a piece's newest open job, as the ops reads return it.
type OpenJob struct {
	Status           sql.NullString
	Attempts         int
	LastError        sql.NullString
	DeadLetterReason sql.NullString
}

// IssueOf turns a piece's issue_number and its open job into the state
// ops shows.
func IssueOf(repo string, number sql.NullInt64, job OpenJob) IssueStatus {
	if number.Valid {
		n := int(number.Int64)
		issue := IssueStatus{State: IssueOpened, Number: &n}
		if repo != "" {
			url := "https://github.com/" + repo + "/issues/" + strconv.Itoa(n)
			issue.URL = &url
		}
		return issue
	}
	issue := IssueStatus{State: IssuePending, Attempts: job.Attempts, LastError: job.LastError.String}
	switch {
	case job.Status.String == "dead_lettered":
		issue.State = IssueDeadLettered
		issue.DeadLetterReason = job.DeadLetterReason.String
	case job.Attempts > 0:
		issue.State = IssueRetrying
	}
	return issue
}

// RoleText is the sentence a piece's GitHub issue carries for its sender
// (feedbackRole), so ops prints the same one.
func RoleText(portal bool, roles sql.NullString) string {
	return roleText(portal, roles)
}
