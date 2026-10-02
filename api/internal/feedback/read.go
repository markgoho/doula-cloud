package feedback

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"doula-cloud/api/internal/pagecursor"
)

// ReadPageSize is how many pieces one page of the founder's list
// carries (docs/api-design.md section 4). A pilot sends a few pieces a
// week, so fifty is months of them; the cursor is for the day it is not.
const ReadPageSize = 50

// The four states the issue of a piece can be in, as the founder's list
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
	// IssueDeadLettered: the open job used up its retries. Nothing tries
	// it again.
	IssueDeadLettered = "dead_lettered"
)

// IssueStatus is what the read page shows about a piece's GitHub issue.
type IssueStatus struct {
	State string `json:"state"`
	// Number and URL are set once the issue exists. URL is absent where
	// no repository is configured (GITHUB_FEEDBACK_REPO), which is every
	// environment but Deployed.
	Number *int    `json:"number,omitempty"`
	URL    *string `json:"url,omitempty"`
	// Attempts and LastError are the open job's own bookkeeping: how
	// many tries failed, and what the last one said. LastError is where a
	// lapsed GITHUB_FEEDBACK_TOKEN shows (#1500).
	Attempts  int    `json:"attempts"`
	LastError string `json:"lastError,omitempty"`
}

// Summary is one row of the founder's list. It carries no free text and
// nobody's name: the list is for finding a piece, and opening one is the
// act that is recorded.
type Summary struct {
	ID      string      `json:"id"`
	Kind    string      `json:"kind"`
	RouteID string      `json:"routeId"`
	SentAt  time.Time   `json:"sentAt"`
	Issue   IssueStatus `json:"issue"`
}

// SummaryPage is docs/api-design.md section 4's envelope.
type SummaryPage struct {
	Items      []Summary `json:"items"`
	NextCursor *string   `json:"nextCursor,omitempty"`
	HasMore    bool      `json:"hasMore"`
}

// The two kinds of sender a piece can have (feedback_sender, 00118).
const (
	SenderStaff  = "staff"
	SenderClient = "client"
)

// Sender is who sent a piece. Name is a Staff member's; a Portal sender
// has none, because a Portal Account is a login and its sign-in address
// is what names it.
type Sender struct {
	Kind  string `json:"kind"`
	Name  string `json:"name,omitempty"`
	Email string `json:"email"`
}

// Piece is every stored field of one piece of Feedback (#1526's own AC),
// as the founder read page shows it.
//
// One column is left out on purpose: client_id. #1526 names the fields
// and names a Portal sender by her Portal Account's sign-in address, not
// by her Client record. The id alone tells the founder nothing, and
// resolving it to her name would put more of a Practice's record in
// front of him than the read needs.
type Piece struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	Text        string    `json:"text"`
	PageURL     string    `json:"pageUrl"`
	RouteID     string    `json:"routeId"`
	AppBuild    string    `json:"appBuild"`
	ScreenWidth int       `json:"screenWidth"`
	Browser     string    `json:"browser"`
	SentAt      time.Time `json:"sentAt"`
	// Role is the sentence the piece's GitHub issue carries (roleText):
	// the role(s), "Client", or "Staff".
	Role string `json:"role"`
	// PracticeName is empty for a piece sent from a screen with no
	// Practice in it.
	PracticeName string      `json:"practiceName,omitempty"`
	Sender       Sender      `json:"sender"`
	Issue        IssueStatus `json:"issue"`
}

// OpenReader opens 00122's door for the rest of tx: the one SELECT
// policy on `feedback` a person's session can satisfy. staffID must be
// the Staff member tx already resolved (the policy checks it against
// current_staff_id()), and the caller must already have decided she is
// the founder -- staffauth.FounderOnly is the only caller.
func OpenReader(ctx context.Context, tx *sql.Tx, staffID string) error {
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.feedback_reader', $1, true)`, staffID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("feedback: open reader: %w", err)
	}
	return nil
}

// openJob is the lateral join both reads share: the newest open job of
// one piece. A piece has one in practice (both send handlers enqueue
// exactly once); newest-first is what keeps the read correct if a later
// ticket re-queues a dead-lettered one. The join starts from `feedback`
// because 00121 dropped the foreign key -- an outbox row can outlive its
// piece, and a row with no piece is nothing this page lists.
const openJob = `LEFT JOIN LATERAL (
	    SELECT status::text AS status, attempt_count, last_error
	      FROM feedback_issue_outbox
	     WHERE feedback_id = f.id AND act = 'open'
	     ORDER BY created_at DESC
	     LIMIT 1
	) o ON true`

// listQuery reads one page, newest first. $1/$2 are the cursor, both
// NULL for the first page. $3 narrows the list to the pieces whose issue
// did not open and whose job has failed: dead-lettered, or still pending
// after at least one failed attempt -- #1526's "dead-lettered or still
// pending after its retries", read as "every piece something has gone
// wrong for", so a lapsed token shows on the first failure rather than a
// day later when the last retry is spent.
const listQuery = `SELECT f.id, f.kind::text, f.route_id, f.sent_at, f.issue_number,
	        COALESCE(o.status, ''), COALESCE(o.attempt_count, 0), COALESCE(o.last_error, '')
	   FROM feedback f
	   ` + openJob + `
	  WHERE ($1::timestamptz IS NULL OR (f.sent_at, f.id) < ($1::timestamptz, $2::uuid))
	    AND (NOT $3::boolean OR (f.issue_number IS NULL
	         AND (o.status = 'dead_lettered' OR (o.status = 'pending' AND o.attempt_count > 0))))
	  ORDER BY f.sent_at DESC, f.id DESC
	  LIMIT $4`

// List reads one page of the founder's list. tx must be one
// staffauth.FounderOnly opened the reader door on. unopenedOnly narrows
// it to the pieces whose issue open has failed (see listQuery).
func List(ctx context.Context, tx *sql.Tx, repo string, unopenedOnly bool, after *pagecursor.Cursor) (SummaryPage, error) {
	var at, id any
	if after != nil {
		at, id = after.At, after.ID
	}
	rows, err := tx.QueryContext(ctx, listQuery, at, id, unopenedOnly, ReadPageSize+1)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return SummaryPage{}, fmt.Errorf("feedback: list: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := []Summary{}
	for rows.Next() {
		var s Summary
		var number sql.NullInt64
		var status, lastError string
		var attempts int
		if err := rows.Scan(&s.ID, &s.Kind, &s.RouteID, &s.SentAt, &number, &status, &attempts, &lastError); err != nil {
			// coverage:ignore reason: scan failure on a well-typed query, not exercised by unit tests
			return SummaryPage{}, fmt.Errorf("feedback: scan list row: %w", err)
		}
		s.Issue = issueOf(repo, number, status, attempts, lastError)
		items = append(items, s)
	}
	// coverage:ignore reason: row iteration failure, not exercised by unit tests
	if err := rows.Err(); err != nil {
		return SummaryPage{}, fmt.Errorf("feedback: iterate list: %w", err)
	}

	page := SummaryPage{Items: items, HasMore: len(items) > ReadPageSize}
	if page.HasMore {
		page.Items = items[:ReadPageSize]
		last := page.Items[ReadPageSize-1]
		next := pagecursor.Encode(last.SentAt, last.ID)
		page.NextCursor = &next
	}
	return page, nil
}

// issueOf turns a piece's issue_number and its open job's bookkeeping
// into the state the read page shows.
func issueOf(repo string, number sql.NullInt64, status string, attempts int, lastError string) IssueStatus {
	if number.Valid {
		n := int(number.Int64)
		issue := IssueStatus{State: IssueOpened, Number: &n}
		if repo != "" {
			url := "https://github.com/" + repo + "/issues/" + strconv.Itoa(n)
			issue.URL = &url
		}
		return issue
	}
	issue := IssueStatus{State: IssuePending, Attempts: attempts, LastError: lastError}
	switch {
	case status == "dead_lettered":
		issue.State = IssueDeadLettered
	case attempts > 0:
		issue.State = IssueRetrying
	}
	return issue
}

// getQuery reads one piece with everything the page prints.
// feedback_sender (00122) supplies the three facts that live on tables
// this transaction has no door onto.
const getQuery = `SELECT f.id, f.kind::text, f.text, f.page_url, f.route_id, f.app_build, f.screen_width,
	        f.browser, f.sent_at, array_to_string(f.roles, ','), f.portal_account IS NOT NULL, f.issue_number,
	        COALESCE(s.sender_name, ''), COALESCE(s.sender_email, ''), COALESCE(s.practice_name, ''),
	        COALESCE(o.status, ''), COALESCE(o.attempt_count, 0), COALESCE(o.last_error, '')
	   FROM feedback f
	   LEFT JOIN LATERAL feedback_sender(f.id) s ON true
	   ` + openJob + `
	  WHERE f.id = $1`

// Get reads one piece. found is false for an id that names no piece --
// one that never existed, one an Erasure destroyed, and one past its
// retention all read the same, because in each case the row is gone.
func Get(ctx context.Context, tx *sql.Tx, repo, id string) (piece Piece, found bool, err error) {
	var roles sql.NullString
	var portal bool
	var number sql.NullInt64
	var status, lastError string
	var attempts int
	err = tx.QueryRowContext(ctx, getQuery, id).Scan(
		&piece.ID, &piece.Kind, &piece.Text, &piece.PageURL, &piece.RouteID, &piece.AppBuild, &piece.ScreenWidth,
		&piece.Browser, &piece.SentAt, &roles, &portal, &number,
		&piece.Sender.Name, &piece.Sender.Email, &piece.PracticeName,
		&status, &attempts, &lastError,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Piece{}, false, nil
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return Piece{}, false, fmt.Errorf("feedback: get: %w", err)
	}
	piece.Sender.Kind = SenderStaff
	if portal {
		piece.Sender.Kind = SenderClient
	}
	piece.Role = roleText(portal, roles)
	piece.Issue = issueOf(repo, number, status, attempts, lastError)
	return piece, true, nil
}

// RecordRead writes the feedback_reads row for one open of a piece
// (#1526): who, and when. at is clock.Now(ctx) at the call site (#773).
func RecordRead(ctx context.Context, tx *sql.Tx, feedbackID, staffID string, at time.Time) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO feedback_reads (feedback_id, staff_id, read_at) VALUES ($1, $2, $3)`,
		feedbackID, staffID, at,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("feedback: record read: %w", err)
	}
	return nil
}
