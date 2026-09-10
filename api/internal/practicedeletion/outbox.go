package practicedeletion

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/outbox"
)

// act names one row practice_deletion_outbox carries -- the Go side of
// the practice_deletion_act enum.
type act string

const (
	actReminder act = "reminder"
	actFinalize act = "finalize"
)

// enqueue writes one practice_deletion_outbox row, due at dueAt.
// InitiateHandler calls this twice, once per act, in the same
// transaction as the practices row it updates.
// enqueue upserts on the same (practice_id, act, status = 'pending')
// uniqueness the table already enforces, rather than inserting a fresh
// row: a restore-then-reinitiate must move the existing pending row's
// deadline forward, not collide with it or leave a stale one behind for
// the skip-at-send recheck to fire early against.
func enqueue(ctx context.Context, tx *sql.Tx, practiceID string, a act, dueAt time.Time) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO practice_deletion_outbox (practice_id, act, next_attempt_at)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (practice_id, act) WHERE status = 'pending'
		 DO UPDATE SET next_attempt_at = EXCLUDED.next_attempt_at, attempt_count = 0, last_error = NULL`,
		practiceID, a, dueAt,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("practicedeletion: enqueue %s: %w", a, err)
	}
	return nil
}

// reminderSubject and reminderText are the day-23 reminder's fixed copy,
// mailed to every current Owner. link is the pending-deletion screen an
// Owner restores from.
//
// reminderLeadDays is RestoreWindow minus ReminderLeadTime, in days --
// derived rather than typed twice, so a change to either constant in
// deletion.go cannot leave this copy naming the wrong number.
//
//nolint:gosec // deletion-reminder copy, not a credential
var reminderSubject = fmt.Sprintf(
	"Doula Cloud: your Practice will be deleted in %d days",
	int((RestoreWindow-ReminderLeadTime)/(24*time.Hour)),
)

func reminderText(link string, finalizeAt time.Time) string {
	return "Hello,\n\n" +
		"Your Practice is scheduled to be deleted on " + finalizeAt.Format("January 2, 2006") + ".\n\n" +
		"Once that happens, every Client on file will have their personal data erased, any unspent " +
		"Credit balance will be forfeited, and no Staff member or Client will be able to sign in.\n\n" +
		"To keep your Practice, restore it before then:\n" +
		link + "\n\n" +
		"If you have questions, reply to this email.\n"
}

// Worker performs both acts a practice_deletion_outbox row can carry:
// mailing the day-23 reminder to every current Owner, and running the
// day-30 finalization. Hand-written rather than outbox.MailWorker[R] for
// the same reason billing.Worker is: finalize sends no mail at all, and
// reminder's recipients are resolved live rather than carried on the
// row, the same "multi-recipient SendAll kind" shape low_credit_outbox
// already uses.
type Worker struct {
	outbox.Mailer
}

func (w Worker) inner() outbox.Worker {
	return w.Worker("practice_deletion_outbox")
}

type pendingRow struct {
	id           string
	practiceID   string
	act          act
	attemptCount int
}

const claimQuery = `SELECT id, practice_id, act::text, attempt_count
	 FROM practice_deletion_outbox
	 WHERE status = 'pending' AND next_attempt_at <= now()
	 ORDER BY next_attempt_at
	 LIMIT $1
	 FOR UPDATE SKIP LOCKED`

func scanRow(rows *sql.Rows) (pendingRow, error) {
	var r pendingRow
	var a string
	err := rows.Scan(&r.id, &r.practiceID, &a, &r.attemptCount)
	r.act = act(a)
	if err != nil {
		// coverage:ignore reason: DB scan failure, not exercised by unit tests
		return r, fmt.Errorf("practicedeletion: scan outbox row: %w", err)
	}
	return r, nil
}

// ProcessPending performs every due reminder or finalization within tx.
func (w Worker) ProcessPending(ctx context.Context, tx *sql.Tx) error {
	if err := outbox.ProcessPending(ctx, tx, w.inner(), claimQuery, scanRow, w.perform); err != nil {
		// coverage:ignore reason: only reached by a DB failure inside the outbox package, not exercised by unit tests
		return fmt.Errorf("practicedeletion: %w", err)
	}
	return nil
}

func (w Worker) perform(ctx context.Context, tx *sql.Tx, inner outbox.Worker, r pendingRow, now time.Time) error {
	switch r.act {
	case actReminder:
		return w.performReminder(ctx, tx, inner, r, now)
	case actFinalize:
		return w.performFinalize(ctx, tx, inner, r, now)
	}
	// coverage:ignore reason: act is constrained by the practice_deletion_act enum, so no other value can be scanned
	return markErr(fmt.Errorf("practicedeletion: unknown act %q", r.act))
}

// performReminder mails every current Owner, unless the Practice's own
// deletion_requested_at has gone null (restored) or deleted_at is
// already set (already finalized) by the time this row was claimed --
// the skip-at-send recheck the package doc describes. A skip marks sent
// with nothing mailed, the same "nothing left to do" shape SendAll
// already gives a Practice with zero current Owners.
func (w Worker) performReminder(ctx context.Context, tx *sql.Tx, inner outbox.Worker, r pendingRow, now time.Time) error {
	state, err := readDeletionState(ctx, tx, r.practiceID, false)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}
	if !state.requestedAt.Valid || state.deletedAt.Valid {
		return markErr(inner.MarkSent(ctx, tx, r.id, now))
	}

	emails, err := ownerEmails(ctx, tx, r.practiceID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}
	link := w.AppBaseURL + "/practices/" + r.practiceID + "/settings/delete"
	return markErr(inner.SendAll(ctx, tx, r.id, r.attemptCount, now, emails, reminderSubject, reminderText(link, state.finalizeAt.Time)))
}

// performFinalize runs the act itself, unless the same recheck finds the
// Practice was restored or a duplicate claim raced an earlier
// finalization. It locks the practices row FOR UPDATE for the length of
// the cascade, the same reason client.EraseHandler locks a Client's own
// row -- nothing else may observe a half-finalized Practice.
func (w Worker) performFinalize(ctx context.Context, tx *sql.Tx, inner outbox.Worker, r pendingRow, now time.Time) error {
	state, err := readDeletionState(ctx, tx, r.practiceID, true)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}
	if !state.requestedAt.Valid || state.deletedAt.Valid {
		return markErr(inner.MarkSent(ctx, tx, r.id, now))
	}

	if err := finalize(ctx, tx, r.practiceID, now); err != nil {
		// coverage:ignore reason: every step finalize takes fails only on a DB query failure, not exercised by unit tests
		return markErr(inner.MarkFailed(ctx, tx, r.id, r.attemptCount, err, now))
	}
	return markErr(inner.MarkSent(ctx, tx, r.id, now))
}

// finalize is the act itself: every Client still on file is erased
// through the same client.Erase ADR-0027 already runs for a single
// Owner-run erasure, any unspent Credit balance is forfeited as one
// credit_ledger row, and practices.deleted_at is stamped -- in that
// order, so the Activity row below can name real counts.
func finalize(ctx context.Context, tx *sql.Tx, practiceID string, now time.Time) error {
	// client.Erase writes through the same practice-scoped RLS policies
	// (clients_update and its neighbors) an Owner-initiated erasure
	// request runs behind via staffauth.Middleware -- this worker's own
	// transaction carries no such context otherwise, since its only door
	// so far is the notification-worker one ProcessHandler sets.
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("practicedeletion: set current practice id: %w", err)
	}

	clientIDs, err := unerasedClientIDs(ctx, tx, practiceID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}
	for _, clientID := range clientIDs {
		if _, err := client.Erase(ctx, tx, practiceID, clientID, activity.SystemActor(), now); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return fmt.Errorf("practicedeletion: erase client %s: %w", clientID, err)
		}
	}

	forfeited, err := forfeitBalance(ctx, tx, practiceID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE practices SET deleted_at = $1 WHERE id = $2`, now, practiceID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("practicedeletion: stamp deleted_at: %w", err)
	}

	diff, err := json.Marshal(finalizedScope{ClientsErased: len(clientIDs), CreditsForfeited: forfeited})
	if err != nil {
		// coverage:ignore reason: a struct of two ints always marshals cleanly, not exercised by unit tests
		return fmt.Errorf("practicedeletion: marshal finalized scope: %w", err)
	}
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectPractice,
		SubjectID:   practiceID,
		Action:      actionFinalized,
		Diff:        diff,
		Actor:       activity.SystemActor(),
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("practicedeletion: record finalization: %w", err)
	}
	return nil
}

// unerasedClientIDs lists every Client under practiceID not already
// erased -- an Owner may have erased one by hand before initiating
// deletion (the only time a live request can reach client.EraseHandler
// at all; the lockout refuses it for the whole window after), and
// finalize must not double-erase her.
func unerasedClientIDs(ctx context.Context, tx *sql.Tx, practiceID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx,
		// merged_into IS NULL excludes the tombstone a merge leaves behind
		// (#813). client.Erase reaches an absorbed record through the
		// survivor's own erasure and through the one SECURITY DEFINER door
		// that can write to a tombstone at all -- calling Erase on the
		// tombstone directly would run redactRecord's plain UPDATE, which
		// clients_update's own USING clause (merged_into IS NULL, 00080)
		// matches zero rows for, and the key would then be destroyed with
		// her name still standing in the row.
		`SELECT id FROM clients WHERE practice_id = $1 AND erased_at IS NULL AND merged_into IS NULL FOR UPDATE`,
		practiceID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("practicedeletion: list unerased clients: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			// coverage:ignore reason: DB scan failure, not exercised by unit tests
			return nil, fmt.Errorf("practicedeletion: scan client id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("practicedeletion: iterate clients: %w", err)
	}
	return ids, nil
}

// forfeitBalance writes one 'forfeit' credit_ledger row zeroing out
// practiceID's current balance, and returns how many Credits it
// forfeited. A zero or negative balance writes nothing: there is
// nothing to forfeit, and an append-only ledger should not carry a
// zero-quantity row that answers no question.
func forfeitBalance(ctx context.Context, tx *sql.Tx, practiceID string) (int, error) {
	balance, err := billing.Balance(ctx, tx, practiceID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return 0, fmt.Errorf("practicedeletion: read balance: %w", err)
	}
	if balance <= 0 {
		return 0, nil
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO credit_ledger (practice_id, origin, quantity) VALUES ($1, 'forfeit', $2)`,
		practiceID, -balance,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return 0, fmt.Errorf("practicedeletion: forfeit balance: %w", err)
	}
	return balance, nil
}

// ownerEmails returns the email of every Staff member holding the owner
// role at practiceID -- the same query billing.ownerEmails and
// payments.ownerEmails each already hold as their own copy, one per
// outbox kind rather than shared, the established pattern for this
// three-line query.
func ownerEmails(ctx context.Context, tx *sql.Tx, practiceID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT s.email FROM staff s
		 JOIN practice_memberships pm ON pm.staff_id = s.id
		 WHERE pm.practice_id = $1 AND 'owner' = ANY(pm.roles)`,
		practiceID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("practicedeletion: resolve owner emails: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var emails []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			// coverage:ignore reason: DB scan failure, not exercised by unit tests
			return nil, fmt.Errorf("practicedeletion: scan owner email: %w", err)
		}
		emails = append(emails, email)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("practicedeletion: iterate owner emails: %w", err)
	}
	return emails, nil
}

// markErr gives an error from the outbox package -- a sibling package,
// so wrapcheck treats its errors as external -- this package's prefix.
func markErr(err error) error {
	if err == nil {
		return nil
	}
	// coverage:ignore reason: only reached by a DB failure inside the outbox package, not exercised by unit tests
	return fmt.Errorf("practicedeletion: %w", err)
}
