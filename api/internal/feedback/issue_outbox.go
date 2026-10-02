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
// #1519's parent issue names (a fourth, "erased", is #1501 Q3's
// follow-up, not written here).
var kindLabel = map[string]string{
	KindNotWorking:    "not working",
	KindIdeaOrRequest: "idea or request",
	KindSomethingElse: "something else",
}

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
	if r.portalAccount.Valid {
		return "Client"
	}
	if !r.roles.Valid {
		return "Staff"
	}
	return strings.ReplaceAll(r.roles.String, ",", ", ")
}

// IssueWorker opens a private GitHub issue for every due
// feedback_issue_outbox row -- the Cloud-Scheduler-driven half of
// ADR-0010's outbox, in the client.ErasureWorker shape rather than
// outbox.MailWorker: this worker mails nobody.
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

// pendingIssueRow is one due outbox row joined to the piece of Feedback
// it is for -- every field an issue's title and body need, and nothing
// else: text is deliberately not selected, so an issue's body cannot
// hold it no matter what issueBody does with the row.
type pendingIssueRow struct {
	id            string
	attemptCount  int
	feedbackID    string
	kind          string
	routeID       string
	appBuild      string
	screenWidth   int
	browser       string
	sentAt        time.Time
	roles         sql.NullString
	portalAccount sql.NullString
}

const issueClaimQuery = `SELECT o.id, o.attempt_count, f.id, f.kind::text, f.route_id, f.app_build,
	        f.screen_width, f.browser, f.sent_at, array_to_string(f.roles, ','), f.portal_account
	   FROM feedback_issue_outbox o
	   JOIN feedback f ON f.id = o.feedback_id
	  WHERE o.status = 'pending' AND o.next_attempt_at <= now()
	  ORDER BY o.next_attempt_at
	  LIMIT $1
	  FOR UPDATE OF o SKIP LOCKED`

func scanIssueRow(rows *sql.Rows) (pendingIssueRow, error) {
	var r pendingIssueRow
	err := rows.Scan(&r.id, &r.attemptCount, &r.feedbackID, &r.kind, &r.routeID, &r.appBuild,
		&r.screenWidth, &r.browser, &r.sentAt, &r.roles, &r.portalAccount)
	if err != nil {
		// coverage:ignore reason: DB scan failure, not exercised by unit tests
		return r, fmt.Errorf("feedback: scan issue outbox row: %w", err)
	}
	return r, nil
}

// ProcessPending opens an issue for every due row within tx.
func (w IssueWorker) ProcessPending(ctx context.Context, tx *sql.Tx) error {
	if err := w.inner().HandlePending(ctx, tx, issueClaimQuery, scanIssueRow, w.perform); err != nil {
		// coverage:ignore reason: only reached by a DB failure inside the outbox package, not exercised by unit tests
		return fmt.Errorf("feedback: %w", err)
	}
	return nil
}

// perform resolves one pending row: adopts an already-open issue found
// by its marker, or opens a new one; labels it either way; then writes
// the issue number onto the feedback row in the same transaction that
// marks the outbox row sent (#1524's own AC).
func (w IssueWorker) perform(ctx context.Context, tx *sql.Tx, inner outbox.Worker, r pendingIssueRow, now time.Time) error {
	marker := issueMarker(r.feedbackID)

	existing, err := w.Creator.ListIssues(ctx, r.sentAt)
	if err != nil {
		return markIssueErr(inner.MarkFailed(ctx, tx, r.id, r.attemptCount, fmt.Errorf("feedback: list issues: %w", err), now))
	}

	number := 0
	for _, issue := range existing {
		if strings.Contains(issue.Body, marker) {
			number = issue.Number
			break
		}
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

	if _, err := tx.ExecContext(ctx, `UPDATE feedback SET issue_number = $1 WHERE id = $2`, number, r.feedbackID); err != nil {
		// coverage:ignore reason: DB update failure, not exercised by unit tests
		return fmt.Errorf("feedback: write issue number: %w", err)
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
