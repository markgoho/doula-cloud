package feedback

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"doula-cloud/api/internal/outbox"
)

// EnqueueIssueOutbox queues a pending feedback_issue_outbox row for
// feedbackID. Must run in the same transaction as the Feedback insert
// (#1524's own AC): a crash between the two would otherwise leave a
// piece of Feedback with no issue ever queued for it.
func EnqueueIssueOutbox(ctx context.Context, tx *sql.Tx, feedbackID string) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO feedback_issue_outbox (feedback_id) VALUES ($1)`,
		feedbackID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("feedback: enqueue issue outbox: %w", err)
	}
	return nil
}

// kindTitle is #1498 Q5's own three phrases, used to build an issue's
// title -- kind plus the route pattern, never free text.
var kindTitle = map[string]string{
	KindNotWorking:    "Something is not working",
	KindIdeaOrRequest: "An idea or a request",
	KindSomethingElse: "Something else",
}

// kindLabel is the GitHub label name for each kind, one of the three
// #1519's parent issue names. The fourth, erasedLabel, is below.
var kindLabel = map[string]string{
	KindNotWorking:    "not working",
	KindIdeaOrRequest: "idea or request",
	KindSomethingElse: "something else",
}

// erasedLabel is #1519's fourth label: what the issue of an erased
// piece carries once it is closed (#1501 Q3). The issue never held
// anything the Erasure had to remove, so it stays, closed and labeled.
const erasedLabel = "erased"

// The two acts a feedback_issue_outbox row carries -- the Go side of the
// feedback_issue_act enum (00121).
const (
	actOpen        = "open"
	actCloseErased = "close_erased"
)

// RetentionMonths is how long a piece of Feedback nothing destroys is
// kept, counted from when it was sent (#1501 Q4, CONTEXT.md's Feedback
// entry). Its GitHub issue is not touched when it goes.
const RetentionMonths = 24

// issueMarker is the hidden marker embedded in an issue's body, carrying
// a piece of Feedback's own id -- #1500's research: the fix for GitHub's
// create-issue endpoint having no idempotency key of its own. Checked
// before every create; a retry that finds it adopts the existing issue's
// number rather than opening a second one.
func issueMarker(feedbackID string) string {
	return "<!-- doula-cloud-feedback-id: " + feedbackID + " -->"
}

// issueTitle is made from kind and routeID alone, per #1524's own AC:
// never the free text.
func issueTitle(kind, routeID string) string {
	return kindTitle[kind] + ": " + routeID
}

// issueBody is exactly the field list #1501 Q1 settled crosses to
// GitHub: the route pattern, the app build, the screen width, the
// browser, the time sent, the role(s), and the link to the founder read
// page -- plus the hidden marker. It never holds the free text (not
// selected by issueClaimQuery in the first place), the full URL, the
// Practice name, or the sender.
func issueBody(r pendingIssueRow, appBaseURL string) string {
	var b strings.Builder
	b.WriteString("Route: `" + r.routeID + "`\n")
	b.WriteString("App build: `" + r.appBuild + "`\n")
	b.WriteString("Screen width: " + strconv.Itoa(r.screenWidth) + "px\n")
	b.WriteString("Browser: " + r.browser + "\n")
	b.WriteString("Sent: " + r.sentAt.UTC().Format(time.RFC3339) + "\n")
	b.WriteString("Role: " + feedbackRole(r) + "\n")
	b.WriteString("\n[Read the full item](" + appBaseURL + "/feedback/" + r.feedbackID + ")\n")
	b.WriteString("\n" + issueMarker(r.feedbackID) + "\n")
	return b.String()
}

// feedbackRole is #1524's own AC: the role(s), or "Client" for a Portal
// sender (#1501 Q2 -- a role with no Practice identifies nobody, and a
// Client's role is exactly that), or "Staff" for a Staff sender with no
// Practice in context (a piece sent from /account, where roles is NULL
// -- feedback_sender's own CHECK ties that to practice_id being NULL
// too). A Staff sender with a Practice but a zero-role membership
// (roles a valid, empty array -- staffauth.staffRolesAt's own "a
// membership can start with zero roles") reports whatever the join
// produces, which is the empty string: she holds no role a reader could
// name, and "Staff" would misstate that a Practice is in context.
func feedbackRole(r pendingIssueRow) string {
	return roleText(r.portalAccount.Valid, r.roles)
}

// roleText is feedbackRole's rule on the two facts it reads, so the
// founder read page (#1526) prints the same sentence the issue carries.
// roles is array_to_string(feedback.roles, ','): NULL where the piece
// has no Practice in context.
func roleText(portal bool, roles sql.NullString) string {
	if portal {
		return "Client"
	}
	if !roles.Valid {
		return "Staff"
	}
	return strings.ReplaceAll(roles.String, ",", ", ")
}

// IssueWorker performs every due feedback_issue_outbox row -- opening a
// private GitHub issue for a piece of Feedback, or closing the issue of
// a piece an Erasure destroyed (#1525) -- and, at the start of each run,
// deletes every piece past RetentionMonths. It is the
// Cloud-Scheduler-driven half of ADR-0010's outbox, in the
// client.ErasureWorker shape rather than outbox.MailWorker: this worker
// mails nobody.
type IssueWorker struct {
	Creator IssueCreator
	// AppBaseURL is prefixed onto /feedback/<id> for the link an issue's
	// body carries (#1524's own AC).
	AppBaseURL string
	Now        func() time.Time
}

func (w IssueWorker) inner() outbox.Worker {
	return outbox.Worker{Now: w.Now, Table: "feedback_issue_outbox"}
}

// pendingIssueRow is one due outbox row. For an open job it is joined to
// the piece of Feedback it is for -- every field an issue's title and
// body need, and nothing else: text is deliberately not selected, so an
// issue's body cannot hold it no matter what issueBody does with the
// row. pieceGone says that join found nothing: the piece was erased, or
// aged out, after the job was queued. A close job names no piece at all
// and carries issueNumber instead.
type pendingIssueRow struct {
	id            string
	attemptCount  int
	act           string
	issueNumber   int
	feedbackID    string
	queuedAt      time.Time
	pieceGone     bool
	kind          string
	routeID       string
	appBuild      string
	screenWidth   int
	browser       string
	sentAt        time.Time
	roles         sql.NullString
	portalAccount sql.NullString
}

// issueClaimQuery LEFT JOINs feedback, where #1524's version JOINed it:
// a close job has no piece, and an open job's piece can be gone. The
// COALESCEs exist only so one row shape scans both -- no value they
// supply is ever read, because perform branches on act and pieceGone
// before anything reads a piece's own field.
const issueClaimQuery = `SELECT o.id, o.attempt_count, o.act::text, COALESCE(o.issue_number, 0),
	        COALESCE(o.feedback_id::text, ''), o.created_at, f.id IS NULL,
	        COALESCE(f.kind::text, ''), COALESCE(f.route_id, ''), COALESCE(f.app_build, ''),
	        COALESCE(f.screen_width, 0), COALESCE(f.browser, ''), COALESCE(f.sent_at, o.created_at),
	        array_to_string(f.roles, ','), f.portal_account
	   FROM feedback_issue_outbox o
	   LEFT JOIN feedback f ON f.id = o.feedback_id
	  WHERE o.status = 'pending' AND o.next_attempt_at <= now()
	  ORDER BY o.next_attempt_at
	  LIMIT $1
	  FOR UPDATE OF o SKIP LOCKED`

func scanIssueRow(rows *sql.Rows) (pendingIssueRow, error) {
	var r pendingIssueRow
	err := rows.Scan(&r.id, &r.attemptCount, &r.act, &r.issueNumber, &r.feedbackID, &r.queuedAt, &r.pieceGone,
		&r.kind, &r.routeID, &r.appBuild, &r.screenWidth, &r.browser, &r.sentAt, &r.roles, &r.portalAccount)
	if err != nil {
		// coverage:ignore reason: DB scan failure, not exercised by unit tests
		return r, fmt.Errorf("feedback: scan issue outbox row: %w", err)
	}
	return r, nil
}

// ProcessPending deletes every piece of Feedback past its retention,
// then performs every due row, all within tx.
func (w IssueWorker) ProcessPending(ctx context.Context, tx *sql.Tx) error {
	if err := w.deleteExpired(ctx, tx); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}
	if err := w.inner().HandlePending(ctx, tx, issueClaimQuery, scanIssueRow, w.perform); err != nil {
		// coverage:ignore reason: only reached by a DB failure inside the outbox package, not exercised by unit tests
		return fmt.Errorf("feedback: %w", err)
	}
	return nil
}

// deleteExpired is #1501 Q4's retention: one indexed DELETE
// (feedback_sent_at, 00121) at the start of each run, the way
// authn.MintSession sweeps expired sessions in line. It rides this
// worker because the drain that runs it is the only scheduled tick there
// is (ADR-0010 as amended by #481) -- no new Scheduler job.
//
// The instant is w.Now, #773's seam for a struct that already carries a
// clock, so a simulation Run that moves the clock ages Feedback with it.
// It is compared against sent_at, which the send handlers also took from
// that seam. Nothing is queued for the piece's issue: it stays as it is.
//
// A piece whose open job is still pending is passed over, and goes on
// the first run after that job settles. Without that, the same run would
// then find the job's piece gone and take it for an erased one --
// performGone cannot tell an aged-out piece from an erased one -- and
// would close its issue as erased, which is exactly what retention must
// not do. A real job settles within a day (outbox.BackoffSchedule), so
// only a Run that jumps the clock across the whole 24 months reaches
// this. The NOT EXISTS reads feedback_issue_outbox_one_pending (00120).
func (w IssueWorker) deleteExpired(ctx context.Context, tx *sql.Tx) error {
	cutoff := w.Now().AddDate(0, -RetentionMonths, 0)
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM feedback f
		  WHERE f.sent_at < $1
		    AND NOT EXISTS (
		        SELECT 1 FROM feedback_issue_outbox o
		         WHERE o.feedback_id = f.id AND o.status = 'pending'
		    )`,
		cutoff,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("feedback: delete expired feedback: %w", err)
	}
	return nil
}

// perform resolves one pending row by its act.
func (w IssueWorker) perform(ctx context.Context, tx *sql.Tx, inner outbox.Worker, r pendingIssueRow, now time.Time) error {
	switch {
	case r.act == actCloseErased:
		return w.finishClose(ctx, tx, inner, r, r.issueNumber, now)
	case r.pieceGone:
		return w.performGone(ctx, tx, inner, r, now)
	default:
		return w.performOpen(ctx, tx, inner, r, now)
	}
}

// findIssueByMarker returns the number of the issue whose body carries
// feedbackID's marker, or 0 when there is none.
func (w IssueWorker) findIssueByMarker(ctx context.Context, feedbackID string, since time.Time) (int, error) {
	existing, err := w.Creator.ListIssues(ctx, since)
	if err != nil {
		return 0, fmt.Errorf("feedback: list issues: %w", err)
	}
	marker := issueMarker(feedbackID)
	for _, issue := range existing {
		if strings.Contains(issue.Body, marker) {
			return issue.Number, nil
		}
	}
	return 0, nil
}

// finishClose closes issue number as erased and marks the row done. The
// label goes on first and on every attempt: both calls are safe to
// repeat, so a retry after either one failed finishes the job rather
// than leaving a closed issue with no label saying why.
func (w IssueWorker) finishClose(ctx context.Context, tx *sql.Tx, inner outbox.Worker, r pendingIssueRow, number int, now time.Time) error {
	if err := w.Creator.AddLabels(ctx, number, []string{erasedLabel}); err != nil {
		return markIssueErr(inner.MarkFailed(ctx, tx, r.id, r.attemptCount, fmt.Errorf("feedback: add erased label: %w", err), now))
	}
	if err := w.Creator.CloseIssue(ctx, number); err != nil {
		return markIssueErr(inner.MarkFailed(ctx, tx, r.id, r.attemptCount, fmt.Errorf("feedback: close issue: %w", err), now))
	}
	return markIssueErr(inner.MarkSent(ctx, tx, r.id, now))
}

// performGone resolves an open job whose piece no longer exists
// (#1525): it opens no issue and marks the job done.
//
// It looks for the marker first, because "no issue was opened" is not
// something the missing row can say. An earlier attempt may have opened
// one and failed before the number reached the feedback row (#1500's
// crash window); the Erasure then found no number to close, and this
// retry is the only act left that can close that issue. The lookup
// starts from when the job was queued -- the same transaction as the
// send -- since the piece's own sent_at went with the row.
func (w IssueWorker) performGone(ctx context.Context, tx *sql.Tx, inner outbox.Worker, r pendingIssueRow, now time.Time) error {
	number, err := w.findIssueByMarker(ctx, r.feedbackID, r.queuedAt)
	if err != nil {
		return markIssueErr(inner.MarkFailed(ctx, tx, r.id, r.attemptCount, err, now))
	}
	if number == 0 {
		return markIssueErr(inner.MarkSent(ctx, tx, r.id, now))
	}
	return w.finishClose(ctx, tx, inner, r, number, now)
}

// enqueueCloseErased queues a "close as erased" job for issueNumber. The
// Erasure's own close jobs are written by erase_client_feedback and
// erase_portal_account_feedback (00121), in the statement that deletes
// the piece; this is the same row, written by the worker for the one
// piece those functions could not see a number on.
func enqueueCloseErased(ctx context.Context, tx *sql.Tx, issueNumber int) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO feedback_issue_outbox (act, issue_number) VALUES ('close_erased', $1)
		 ON CONFLICT (issue_number) WHERE status = 'pending' AND act = 'close_erased' DO NOTHING`,
		issueNumber,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("feedback: enqueue close as erased: %w", err)
	}
	return nil
}

// performOpen resolves one open job: adopts an already-open issue found
// by its marker, or opens a new one; labels it either way; then writes
// the issue number onto the feedback row in the same transaction that
// marks the outbox row sent (#1524's own AC).
func (w IssueWorker) performOpen(ctx context.Context, tx *sql.Tx, inner outbox.Worker, r pendingIssueRow, now time.Time) error {
	number, err := w.findIssueByMarker(ctx, r.feedbackID, r.sentAt)
	if err != nil {
		return markIssueErr(inner.MarkFailed(ctx, tx, r.id, r.attemptCount, err, now))
	}

	if number == 0 {
		number, err = w.Creator.CreateIssue(ctx, issueTitle(r.kind, r.routeID), issueBody(r, w.AppBaseURL))
		if err != nil {
			return markIssueErr(inner.MarkFailed(ctx, tx, r.id, r.attemptCount, fmt.Errorf("feedback: create issue: %w", err), now))
		}
	}

	// Outside the create branch on purpose (#1587): a retry that adopts
	// an issue cannot tell whether the attempt that opened it got as far
	// as labeling it, and GitHub's add-labels endpoint does not add a
	// label an issue already has, so every attempt makes the call.
	if err := w.Creator.AddLabels(ctx, number, []string{kindLabel[r.kind]}); err != nil {
		return markIssueErr(inner.MarkFailed(ctx, tx, r.id, r.attemptCount, fmt.Errorf("feedback: add labels: %w", err), now))
	}

	res, err := tx.ExecContext(ctx, `UPDATE feedback SET issue_number = $1 WHERE id = $2`, number, r.feedbackID)
	if err != nil {
		// coverage:ignore reason: DB update failure, not exercised by unit tests
		return fmt.Errorf("feedback: write issue number: %w", err)
	}
	written, err := res.RowsAffected()
	if err != nil {
		// coverage:ignore reason: pgx always reports a row count for an UPDATE, not exercised by unit tests
		return fmt.Errorf("feedback: count issue number writes: %w", err)
	}
	if written == 0 {
		// The piece was erased while this job was talking to GitHub: the
		// claim read it, and it is gone now. The Erasure deleted a row
		// with no issue number on it, so it queued no close, and the issue
		// just opened would stay open forever. This UPDATE is where the
		// two meet -- it waits on the Erasure's row lock if the delete is
		// still in flight, and the Erasure's DELETE waits on this one and
		// reads the number if this got there first -- so exactly one of
		// them queues the close.
		if err := enqueueCloseErased(ctx, tx, number); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return err
		}
	}

	return markIssueErr(inner.MarkSent(ctx, tx, r.id, now))
}

// markIssueErr gives an error from the outbox package (a sibling
// package, so wrapcheck treats its errors as external) this package's
// own prefix.
func markIssueErr(err error) error {
	if err == nil {
		return nil
	}
	// coverage:ignore reason: only reached by a DB failure inside the outbox package, not exercised by unit tests
	return fmt.Errorf("feedback: %w", err)
}
