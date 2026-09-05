// Package mailsuppress is ADR-0029's address-keyed email suppression:
// the one fact that says "this address must receive no more mail from
// Doula Cloud", shared by all eleven mail kinds because ADR-0011 puts
// them on one Mailgun domain and one reputation.
//
// It is deliberately not a notification_preferences channel (#303).
// Preferences are keyed on identity_uid + engagement_id -- a Client's
// own per-Engagement choice -- and a suppression is neither: it is
// Mailgun's reaction to what happened, and it has to exist before any
// account does, since the Client portal invite and the Staff invitation
// both send pre-account.
//
// The guard is a mail.Sender decorator rather than a NOT EXISTS clause
// in each outbox's claim query. A row excluded at claim time would stay
// 'pending' forever and record nothing; refused at send time it
// dead-letters with a reason, which is the visibility ADR-0029 asks for.
// One decorator also covers every kind at once -- eleven hand-written
// claim guards is eleven chances to miss one.
package mailsuppress

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"doula-cloud/api/internal/mail"
)

// Cause is why an address was suppressed. Only CauseBounce is ever
// clearable (ADR-0029): a complaint means someone marked the shared
// domain's mail as spam, and a re-send risks a second complaint against
// every Practice's reputation, not just that one address's.
const (
	CauseBounce    = "bounce"
	CauseComplaint = "complaint"
)

// Normalize is how an address is stored and compared. Mailgun reports
// the recipient as the sender wrote it, so the local part's case must
// not decide whether a suppression is found.
func Normalize(address string) string {
	return strings.ToLower(strings.TrimSpace(address))
}

// Querier is the read half of *sql.DB and *sql.Tx both, so a send-time
// check on the pool and a webhook's check inside its own transaction use
// the same function.
type Querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Execer is the write half, for the same reason.
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Active reports whether address is suppressed right now -- a row that
// exists and has not been cleared.
func Active(ctx context.Context, q Querier, address string) (bool, error) {
	var one int
	err := q.QueryRowContext(ctx,
		`SELECT 1 FROM email_suppressions WHERE address = $1 AND cleared_at IS NULL`,
		Normalize(address),
	).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, fmt.Errorf("mailsuppress: read suppression: %w", err)
	}
	return true, nil
}

// Record suppresses address. A re-suppression of an address a Staff
// member had cleared re-arms it -- cleared_at goes back to NULL and the
// new cause and event id replace the old ones, because the reason it is
// suppressed now is the new event, not the one somebody already dealt
// with.
func Record(ctx context.Context, e Execer, address, cause, mailgunEventID string) error {
	if _, err := e.ExecContext(ctx,
		`INSERT INTO email_suppressions (address, cause, mailgun_event_id)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (address) DO UPDATE SET
		     cause = EXCLUDED.cause,
		     mailgun_event_id = EXCLUDED.mailgun_event_id,
		     created_at = now(),
		     cleared_at = NULL,
		     cleared_by = NULL`,
		Normalize(address), cause, sql.NullString{String: mailgunEventID, Valid: mailgunEventID != ""},
	); err != nil {
		// coverage:ignore reason: DB write failure, not exercised by unit tests
		return fmt.Errorf("mailsuppress: record suppression: %w", err)
	}
	return nil
}

// Sender wraps the real mail.Sender and refuses a suppressed address
// with mail.ErrSuppressed instead of handing it to Mailgun -- which
// would refuse it server-side anyway, but without telling Doula Cloud's
// own outbox rows anything about why.
type Sender struct {
	Inner mail.Sender
	DB    Querier
}

// Send delivers msg unless msg.To is suppressed.
func (s Sender) Send(ctx context.Context, msg mail.Message) error {
	suppressed, err := Active(ctx, s.DB, msg.To)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}
	if suppressed {
		return fmt.Errorf("%w: %s", mail.ErrSuppressed, Normalize(msg.To))
	}
	if err := s.Inner.Send(ctx, msg); err != nil {
		return fmt.Errorf("mailsuppress: %w", err)
	}
	return nil
}
