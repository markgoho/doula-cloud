package billing

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// FoundingGrantPerStaff is how many Credits a pilot Practice receives for
// each Staff member it has on the day it joins (#439). Three, the same
// number a signup bonus gives a whole Practice -- so a solo doula gets
// three and the fourteen-doula Rochester agency gets forty-two.
const FoundingGrantPerStaff = 3

var (
	// ErrAlreadyGranted is returned when a Practice already holds a
	// founding grant. #439 counted it once and never topped it up, so a
	// second request is a mistake rather than a bigger grant, and it is
	// refused rather than silently doubling the Practice's balance.
	ErrAlreadyGranted = errors.New("billing: practice already holds a founding grant")
	// ErrNoStaff is returned when a Practice has no Staff to size the
	// grant from. Three times nothing is nothing, and a zero-quantity
	// lot is refused by credit_ledger_lot_or_draw anyway -- saying so
	// here gives the operator the reason instead of a constraint name.
	ErrNoStaff = errors.New("billing: practice has no staff to size a founding grant from")
	// ErrNoGrantor is returned when the request does not say who is
	// issuing the grant. Required, because the audit record is the point
	// of the ticket that built this.
	ErrNoGrantor = errors.New("billing: grantedBy is required")
)

// FoundingGrantReceipt is what one issued grant did: how many Staff it
// was sized from, and how many Credits that came to.
type FoundingGrantReceipt struct {
	StaffCount int    `json:"staffCount"`
	Credits    int    `json:"credits"`
	GrantedBy  string `json:"grantedBy"`
}

// FoundingGrant gives a Practice its founding Credits, sized at
// FoundingGrantPerStaff for each Staff member on its roster right now.
//
// grantedBy is who is issuing it -- the platform operator, named in the
// request rather than derived from a session, because there is no session
// here and no Staff row for a person who is not a member of any Practice.
// It is the answer to "who gave this Practice its Credits?", and the
// constraint on credit_ledger is what keeps it from being skipped.
//
// The grant is a lot like any other: quantity positive, priced at $0.00
// with $0.00 of tax. Two consequences follow from that alone, and neither
// needs code here. openLots is FIFO, and a grant is always the Practice's
// oldest lot, so granted Credits are spent before purchased ones.
// refundableLots keeps only lots with a positive unit price and a
// PaymentIntent, so a granted Credit is never refundable -- which is what
// docs/copy/support-page.md already promises.
func FoundingGrant(ctx context.Context, tx *sql.Tx, practiceID, grantedBy string) (FoundingGrantReceipt, error) {
	grantedBy = strings.TrimSpace(grantedBy)
	if grantedBy == "" {
		return FoundingGrantReceipt{}, ErrNoGrantor
	}

	// Same lock ConsumeCredit and Refund take. Here it also serialises
	// two grants arriving together: the unique index below would refuse
	// the second either way, but the lock makes the refusal
	// ErrAlreadyGranted rather than a constraint violation.
	if _, err := tx.ExecContext(ctx, `SELECT id FROM practices WHERE id = $1 FOR UPDATE`, practiceID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return FoundingGrantReceipt{}, fmt.Errorf("billing: lock practice: %w", err)
	}

	var granted bool
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM credit_ledger WHERE practice_id = $1 AND origin = 'founding_grant')`,
		practiceID,
	).Scan(&granted); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return FoundingGrantReceipt{}, fmt.Errorf("billing: look up founding grant: %w", err)
	}
	if granted {
		return FoundingGrantReceipt{}, ErrAlreadyGranted
	}

	var staffCount int
	if err := tx.QueryRowContext(ctx,
		`SELECT count(*) FROM practice_memberships WHERE practice_id = $1`,
		practiceID,
	).Scan(&staffCount); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return FoundingGrantReceipt{}, fmt.Errorf("billing: count practice staff: %w", err)
	}
	if staffCount == 0 {
		return FoundingGrantReceipt{}, ErrNoStaff
	}

	credits := staffCount * FoundingGrantPerStaff
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO credit_ledger (practice_id, origin, quantity, granted_by)
		 VALUES ($1, 'founding_grant', $2, $3)`,
		practiceID, credits, grantedBy,
	); err != nil {
		// coverage:ignore reason: the unique index on (practice_id) WHERE origin = 'founding_grant' is the last word if two grants got past the lock above, and the lock makes that unreachable from a test -- a second caller waits on the practice row and then reads the committed grant, returning ErrAlreadyGranted before it reaches this insert
		return FoundingGrantReceipt{}, fmt.Errorf("billing: insert founding grant: %w", err)
	}

	return FoundingGrantReceipt{StaffCount: staffCount, Credits: credits, GrantedBy: grantedBy}, nil
}
