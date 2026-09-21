package oncall

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"doula-cloud/api/internal/activity"
)

// gapFacts is a gap's stored shape, as written and as recorded in a
// diff. The reason is deliberately absent from the diff: it is free text
// about a person's evening, and the ledger answers who changed the gap
// and when without repeating it.
type gapFacts struct {
	StaffID         string    `json:"staffId"`
	StartsAt        time.Time `json:"startsAt"`
	EndsAt          time.Time `json:"endsAt"`
	CoveringStaffID *string   `json:"coveringStaffId"`
	reason          *string
}

// diffBefore and diffAfter are the two keys every on-call diff in this
// package is written under, so a ledger reader meets one convention.
const (
	diffBefore = "before"
	diffAfter  = "after"
)

// gapDiff is the Activity diff every gap action records. Before is
// absent on a create and After on a clear.
type gapDiff struct {
	GapID  string    `json:"gapId"`
	Before *gapFacts `json:"before,omitempty"`
	After  *gapFacts `json:"after,omitempty"`
}

// errGapNotFound is a gap that is not on this Engagement, or is already
// cleared.
var errGapNotFound = errors.New("oncall: coverage gap not found")

// lockLiveGap reads a live gap on engagementID for update, so two edits
// to one gap serialize rather than both reading the same before.
func lockLiveGap(ctx context.Context, tx *sql.Tx, engagementID, gapID string) (gapFacts, error) {
	var f gapFacts
	var covering, reason sql.NullString
	err := tx.QueryRowContext(ctx,
		`SELECT staff_id, starts_at, ends_at, covering_staff_id, reason
		   FROM engagement_coverage_gaps
		  WHERE id = $1 AND engagement_id = $2 AND cleared_at IS NULL
		  FOR UPDATE`,
		gapID, engagementID,
	).Scan(&f.StaffID, &f.StartsAt, &f.EndsAt, &covering, &reason)
	if errors.Is(err, sql.ErrNoRows) {
		return gapFacts{}, errGapNotFound
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return gapFacts{}, fmt.Errorf("oncall: lock coverage gap: %w", err)
	}
	f.CoveringStaffID = nullString(covering)
	f.reason = nullString(reason)
	return f, nil
}

// gapColumns is the projection every Gap is read through, in the order
// scanGap takes them. One list, so the roster's read and the write
// handlers' read-back cannot shape a Gap two different ways.
const gapColumns = `g.id, g.engagement_id, g.staff_id, s.name, g.starts_at, g.ends_at,
	       g.reason, g.covering_staff_id, cs.name`

// gapJoins resolves both Staff names a Gap carries. LEFT, because a
// colleague whose Membership has ended is a row this Practice can no
// longer see, and the reader gets the words for that rather than a bare
// id (displayName).
const gapJoins = `FROM engagement_coverage_gaps g
	   LEFT JOIN staff s ON s.id = g.staff_id
	   LEFT JOIN staff cs ON cs.id = g.covering_staff_id`

// scanner is what a *sql.Row and a *sql.Rows both are, so scanGap serves
// the one-row read and the list read without knowing which it is in.
type scanner interface {
	Scan(dest ...any) error
}

// scanGap reads one gapColumns row into the shape a response carries.
func scanGap(row scanner) (Gap, error) {
	var g Gap
	var staffName, reason, coveringID, coveringName sql.NullString
	if err := row.Scan(&g.ID, &g.EngagementID, &g.StaffID, &staffName, &g.StartsAt, &g.EndsAt,
		&reason, &coveringID, &coveringName); err != nil {
		// coverage:ignore reason: DB scan failure, not exercised by unit tests
		return Gap{}, fmt.Errorf("oncall: scan coverage gap: %w", err)
	}
	g.StaffName = displayName(staffName)
	g.Reason = nullString(reason)
	g.CoveringStaffID = nullString(coveringID)
	if g.CoveringStaffID != nil {
		name := displayName(coveringName)
		g.CoveringStaffName = &name
	}
	return g, nil
}

// readGap reads one gap as the response shows it.
func readGap(ctx context.Context, tx *sql.Tx, gapID string) (Gap, error) {
	return scanGap(tx.QueryRowContext(ctx, `SELECT `+gapColumns+` `+gapJoins+` WHERE g.id = $1`, gapID))
}

// recordGap writes one gap action to the Engagement's Activity ledger.
func recordGap(ctx context.Context, tx *sql.Tx, practiceID, engagementID, actorStaffID string, action activity.EngagementAction, diff gapDiff) error {
	return record(ctx, tx, activity.SubjectEngagement, practiceID, engagementID, actorStaffID, string(action), diff)
}

// record is the one path every on-call write takes to the Activity
// ledger: marshal the diff, name the acting Staff member, and say which
// action failed if the write does. The three write sites -- a gap, an
// Engagement's rule or narrowing, and the Practice's own rule -- differ
// only in the subject they name and the diff they carry.
func record(ctx context.Context, tx *sql.Tx, subjectKind, practiceID, subjectID, actorStaffID, action string, diff any) error {
	raw, err := json.Marshal(diff)
	if err != nil {
		// coverage:ignore reason: marshal of a fixed, always-serializable value never fails
		return fmt.Errorf("oncall: marshal %s diff: %w", action, err)
	}
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: subjectKind,
		SubjectID:   subjectID,
		Action:      action,
		Diff:        raw,
		Actor:       activity.StaffActor(actorStaffID),
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("oncall: record %s: %w", action, err)
	}
	return nil
}

// queueGapNotice queues the "a birth needs on-call cover" Notification
// for gapID. At most one row per gap is ever pending (00114's partial
// unique index), so a second save of the same uncovered gap before the
// first mail goes is one mail, not two. Reports whether a row was
// queued, which is when the handler nudges the worker.
func queueGapNotice(ctx context.Context, tx *sql.Tx, practiceID, gapID, actorStaffID string) (bool, error) {
	res, err := tx.ExecContext(ctx,
		`INSERT INTO coverage_gap_outbox (practice_id, gap_id, requested_by_staff_id)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (gap_id) WHERE status = 'pending' DO NOTHING`,
		practiceID, gapID, actorStaffID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, fmt.Errorf("oncall: queue coverage gap notice: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		// coverage:ignore reason: the pgx driver always reports rows affected for an INSERT
		return false, fmt.Errorf("oncall: count queued coverage gap notice: %w", err)
	}
	return n > 0, nil
}
