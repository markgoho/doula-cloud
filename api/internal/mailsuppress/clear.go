package mailsuppress

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrNotClearable is what Clear returns for a complaint-caused row.
// ADR-0029 makes this a property of the suppression itself rather than
// of any one screen: a complaint means a recipient marked the mail on
// the one domain every Practice shares as spam, and a re-send risks a
// second complaint against all of them. Refusing it here rather than in
// the UI is what makes that hold for any caller.
var ErrNotClearable = errors.New("mailsuppress: a complaint-caused suppression is never cleared")

// ErrNotSuppressed is what Clear returns when the address has no active
// suppression -- never had one, or somebody else cleared it first.
var ErrNotSuppressed = errors.New("mailsuppress: address is not suppressed")

// BounceClearer is the vendor half of a clear: taking the address off
// Mailgun's own bounce list. Narrow on purpose -- mail.MailgunSender
// satisfies it in production and mail.FakeSender in tests, and nothing
// here needs the rest of a Sender.
type BounceClearer interface {
	DeleteBounce(ctx context.Context, address string) error
}

// Item is one suppressed address as Staff see it.
type Item struct {
	Address   string
	Cause     string
	CreatedAt time.Time
}

// Clear lifts a bounce-caused suppression on address, recording staffID
// as who did it.
//
// The two writes are deliberately ordered vendor-first. Mailgun keeps
// its own bounce list and refuses the send server-side regardless of
// what email_suppressions says, so a local row cleared while Mailgun's
// entry survives is a screen that reports the address as usable when it
// is not. Failing before the local UPDATE leaves both sides suppressed,
// which is wrong in the direction that is merely inconvenient: the
// address stays blocked and the Staff member tries again. The reverse
// order has no such safe failure.
func Clear(ctx context.Context, tx *sql.Tx, clearer BounceClearer, address, staffID string) error {
	normalized := Normalize(address)

	var cause string
	err := tx.QueryRowContext(ctx,
		`SELECT cause FROM email_suppressions WHERE address = $1 AND cleared_at IS NULL`,
		normalized,
	).Scan(&cause)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotSuppressed
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("mailsuppress: read suppression: %w", err)
	}
	if cause != CauseBounce {
		return ErrNotClearable
	}

	if err := clearer.DeleteBounce(ctx, normalized); err != nil {
		return fmt.Errorf("mailsuppress: clear Mailgun bounce: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE email_suppressions
		    SET cleared_at = now(), cleared_by = $2
		  WHERE address = $1 AND cleared_at IS NULL AND cause = 'bounce'`,
		normalized, staffID,
	); err != nil {
		// coverage:ignore reason: DB write failure, not exercised by unit tests
		return fmt.Errorf("mailsuppress: clear suppression: %w", err)
	}
	return nil
}

// List returns every active suppression on an address that belongs to
// practiceID, oldest first.
//
// The table itself is platform-level and address-keyed -- ADR-0029's
// whole point -- so "belongs to" is a question about the Practice's own
// records, not about the suppression. Three places hold an address a
// Practice is responsible for: a Client of theirs, a Staff member on
// their roster, and an outstanding Practice Invitation. An address on
// none of the three is another Practice's business and is not listed
// here, even though the suppression blocking it is the same row.
func List(ctx context.Context, q RowsQuerier, practiceID string) ([]Item, error) {
	rows, err := q.QueryContext(ctx, practiceAddressSQL, practiceID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("mailsuppress: list suppressions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := []Item{}
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.Address, &it.Cause, &it.CreatedAt); err != nil {
			// coverage:ignore reason: scan failure implies a schema/query mismatch, not a runtime path
			return nil, fmt.Errorf("mailsuppress: scan suppression: %w", err)
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("mailsuppress: iterate suppressions: %w", err)
	}
	return items, nil
}

// RowsQuerier is the multi-row read half of *sql.DB and *sql.Tx both,
// the same reason Querier above covers the single-row one.
type RowsQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// AttachedToPractice reports whether address is one practiceID is
// responsible for, by the same three-table definition List uses. It is
// the authorization check on a clear: email_suppressions carries no
// practice_id and no RLS policy, so this function is the only boundary
// that can stop one Practice acting on another's address.
func AttachedToPractice(ctx context.Context, q Querier, practiceID, address string) (bool, error) {
	var one int
	err := q.QueryRowContext(ctx,
		`SELECT 1 FROM (`+practiceAddressesSQL+`) a WHERE a.address = $2`,
		practiceID, Normalize(address),
	).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, fmt.Errorf("mailsuppress: read practice address: %w", err)
	}
	return true, nil
}

// practiceAddressesSQL is the one definition of "an address this
// Practice is responsible for", written once so List and
// AttachedToPractice cannot drift into disagreeing about who may see an
// address and who may clear it.
const practiceAddressesSQL = `
	SELECT lower(c.email) AS address
	  FROM clients c
	 WHERE c.practice_id = $1 AND c.email IS NOT NULL
	 UNION
	SELECT lower(s.email)
	  FROM staff s
	  JOIN practice_memberships pm ON pm.staff_id = s.id
	 WHERE pm.practice_id = $1
	 UNION
	SELECT lower(pi.address)
	  FROM practice_invitations pi
	 WHERE pi.practice_id = $1`

const practiceAddressSQL = `
	SELECT es.address, es.cause, es.created_at
	  FROM email_suppressions es
	  JOIN (` + practiceAddressesSQL + `) a ON a.address = es.address
	 WHERE es.cleared_at IS NULL
	 ORDER BY es.created_at`
