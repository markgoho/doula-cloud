// Package cardpayments owns the one definition of whether Clients can pay
// a Practice by card (#270, #2295): its stored card payments capability,
// practices.stripe_connect_card_payments_status, is active.
//
// It is a leaf of its own because payments imports staffauth, and both
// read the fact: the Practice session in the same query as the Practice's
// name, and payments' Invoice gate, Invoice list and portal payment.
package cardpayments

import (
	"context"
	"database/sql"
	"fmt"
)

// Status is the status Stripe reports for one capability on a v2
// Account's merchant configuration -- four-valued, where v1's equivalent
// was a boolean. Only Active means the capability actually works; Pending
// is Stripe reviewing something the Owner has already supplied, and
// Restricted means something is still outstanding.
type Status string

// The four values Stripe's capability `status` field takes.
const (
	Active      Status = "active"
	Pending     Status = "pending"
	Restricted  Status = "restricted"
	Unsupported Status = "unsupported"
)

// ClientsCanPayExpr is the predicate as a boolean SQL expression over a
// practices row, for a caller that reads it alongside other columns in a
// query of its own.
const ClientsCanPayExpr = "stripe_connect_card_payments_status = '" + string(Active) + "'"

// ClientsCanPay reads the predicate for practiceID on its own. It reads
// the column the Connect webhook keeps fresh, never Stripe: every reader
// needs an instant answer on a request that is not itself about Connect.
func ClientsCanPay(ctx context.Context, tx *sql.Tx, practiceID string) (bool, error) {
	var canPay bool
	if err := tx.QueryRowContext(ctx,
		`SELECT `+ClientsCanPayExpr+` FROM practices WHERE id = $1`, practiceID,
	).Scan(&canPay); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, fmt.Errorf("cardpayments: read clients can pay: %w", err)
	}
	return canPay, nil
}
