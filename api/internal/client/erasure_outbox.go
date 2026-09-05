package client

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"doula-cloud/api/internal/outbox"
)

// StripeEraser is the slice of payments.Client this worker needs: the
// two Stripe acts erasure defers. It is declared here, structurally,
// rather than importing payments.Client -- payments already imports this
// package (invoice.go reads a Client's name), so naming the type would
// be an import cycle. main.go passing the real payments.Client in is
// what proves the two shapes still agree.
type StripeEraser interface {
	DeleteCustomer(ctx context.Context, accountID, customerID string) error
	CreateRedactionJob(ctx context.Context, accountID, customerID string) (string, error)
}

// ErasureWorker performs the two outside-world acts an erasure enqueued:
// deleting a Stripe Customer and running its Redaction Job once Stripe's
// 90-day floor has passed (ADR-0027). It no longer touches Identity
// Platform: #617 retired that half once a Client stopped having an
// account there to delete (ADR-0026).
//
// It is the only outbox worker in this product that sends no mail. It
// still rides outbox.ProcessPending because the machinery is what it
// needs -- claim with SKIP LOCKED, retry on the shared backoff schedule,
// dead-letter when retrying stops being worth it -- and none of that is
// about email.
type ErasureWorker struct {
	Stripe StripeEraser
	Now    func() time.Time
}

func (w ErasureWorker) inner() outbox.Worker {
	return outbox.Worker{Now: w.Now, Table: "client_erasure_outbox"}
}

type erasurePendingRow struct {
	id           string
	practiceID   string
	act          erasureAct
	target       string
	attemptCount int
}

// erasureClaimQuery claims due rows the same way every other outbox does.
// next_attempt_at is what defers a redaction job to its eligibility date,
// so this needs no special case for it: a job that is not yet allowed is
// simply not yet due.
const erasureClaimQuery = `SELECT id, practice_id, act::text, target, attempt_count
	 FROM client_erasure_outbox
	 WHERE status = 'pending' AND next_attempt_at <= now()
	 ORDER BY next_attempt_at
	 LIMIT $1
	 FOR UPDATE SKIP LOCKED`

func scanErasureRow(rows *sql.Rows) (erasurePendingRow, error) {
	var r erasurePendingRow
	var act string
	err := rows.Scan(&r.id, &r.practiceID, &act, &r.target, &r.attemptCount)
	r.act = erasureAct(act)
	if err != nil {
		// coverage:ignore reason: DB scan failure, not exercised by unit tests
		return r, fmt.Errorf("client: scan erasure outbox row: %w", err)
	}
	return r, nil
}

// ProcessPending performs every due erasure act within tx.
func (w ErasureWorker) ProcessPending(ctx context.Context, tx *sql.Tx) error {
	if err := outbox.ProcessPending(ctx, tx, w.inner(), erasureClaimQuery, scanErasureRow, w.perform); err != nil {
		// coverage:ignore reason: only reached by a DB failure inside the outbox package, not exercised by unit tests
		return fmt.Errorf("client: %w", err)
	}
	return nil
}

// errNoConnectAccount is what a Stripe act reports when the Practice has
// no connected account any more. It is terminal, not retryable: there is
// no account to reach, and no later attempt will find one.
var errNoConnectAccount = errors.New("client: practice has no stripe connected account")

func (w ErasureWorker) perform(ctx context.Context, tx *sql.Tx, inner outbox.Worker, r erasurePendingRow, now time.Time) error {
	err := w.act(ctx, tx, r)
	if errors.Is(err, errNoConnectAccount) {
		// Nothing to do and nothing to wait for. Dead-lettered rather
		// than marked sent, so the row still reads as "this act did not
		// happen" when someone asks what an erasure actually covered.
		return markErr(inner.MarkDeadLetteredNow(ctx, tx, r.id, err.Error()))
	}
	if err != nil {
		return markErr(inner.MarkFailed(ctx, tx, r.id, r.attemptCount, err, now))
	}
	return markErr(inner.MarkSent(ctx, tx, r.id, now))
}

// act does the one thing r names. Each branch is idempotent on its own
// side -- a deleted Customer deletes again as a no-op -- so a retry after
// a partial failure never double-acts.
func (w ErasureWorker) act(ctx context.Context, tx *sql.Tx, r erasurePendingRow) error {
	switch r.act {
	case actStripeCustomerDelete, actStripeRedactionJob:
		accountID, err := connectAccount(ctx, tx, r.practiceID)
		if err != nil {
			return err
		}
		if r.act == actStripeCustomerDelete {
			if err := w.Stripe.DeleteCustomer(ctx, accountID, r.target); err != nil {
				return fmt.Errorf("client: delete stripe customer: %w", err)
			}
			return nil
		}
		if _, err := w.Stripe.CreateRedactionJob(ctx, accountID, r.target); err != nil {
			return fmt.Errorf("client: create stripe redaction job: %w", err)
		}
		return nil
	}
	// coverage:ignore reason: act is constrained by the client_erasure_act enum, so no other value can be scanned
	return fmt.Errorf("client: unknown erasure act %q", r.act)
}

// connectAccount reads the Practice's Stripe connected account id. The
// worker runs with no Practice session context, so this reads the column
// directly rather than through a practice-scoped helper; practices has no
// RLS of its own to satisfy here.
func connectAccount(ctx context.Context, tx *sql.Tx, practiceID string) (string, error) {
	var accountID sql.NullString
	if err := tx.QueryRowContext(ctx,
		`SELECT stripe_connect_account_id FROM practices WHERE id = $1`, practiceID,
	).Scan(&accountID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests -- the row is guaranteed by the outbox row's foreign key
		return "", fmt.Errorf("client: read connect account: %w", err)
	}
	if !accountID.Valid || accountID.String == "" {
		return "", errNoConnectAccount
	}
	return accountID.String, nil
}

// markErr gives an error from the outbox package -- a sibling package,
// so wrapcheck treats its errors as external -- this package's prefix.
func markErr(err error) error {
	if err == nil {
		return nil
	}
	// coverage:ignore reason: only reached by a DB failure inside the outbox package, not exercised by unit tests
	return fmt.Errorf("client: %w", err)
}
