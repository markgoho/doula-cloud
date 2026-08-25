// Package offer is ADR-0008's Offer flow (#317): the event that grants a
// Doula's attachment to one Engagement through her own agreement. An
// Owner or Admin offers work to an existing doula Membership or to a bare
// email address, fan-out is uncapped and the first acceptance wins,
// acceptance mints a granted engagement_attachments row with the fee and
// terms copied onto it, and completion of the Engagement closes whatever
// is still open.
//
// An Offer is a copy, never a view: the four decidable facts it carries
// (Client first initial, general area, exact due date, fee) plus its
// free-text terms are typed in at send time and never refreshed, which
// is what lets someone who is not Staff at all read enough to decide
// before she has an account (#230). A changed fact means withdraw and
// re-offer.
package offer

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"doula-cloud/api/internal/staffauth"
)

// Lifetime is how long an Offer stays open after it is sent -- 7 days,
// the default #229 chose and ADR-0008 records, the same as an
// Invitation's.
const Lifetime = 7 * 24 * time.Hour

// maxAccessCodeAttempts bounds guessing at the pre-account read's
// six-digit code. The endpoint sits outside staffauth, so the code is the
// whole second factor in front of it; ten guesses against a 10^6 space is
// a 1-in-100,000 chance of a hit, and a burned Offer stays burned because
// the counter lives on the row (00041) rather than in a process.
const maxAccessCodeAttempts = 10

// newAccessCode returns the six-digit code the pre-account Offer read
// asks for, from crypto/rand rather than math/rand: it is a credential,
// short enough that a predictable one would be trivially reproducible.
// Formatted with leading zeros so every code is six characters -- a
// five-digit one would leak that the number is small.
func newAccessCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		// coverage:ignore reason: crypto/rand failure, not exercised by unit tests
		return "", fmt.Errorf("offer: generate access code: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// expireOpen flips every Offer matching the given engagement/staff/id
// filter that has run past its own expires_at to 'expired', on the way
// past. There is no sweep job: acceptInvite (staffauth) set the
// precedent, and for the same reason -- the person looking at the row is
// the one who found out it is stale, and every read and decision path in
// this package calls this first so no path can act on an Offer that has
// quietly outlived itself.
//
// column is one of the three this package filters Offers by, never
// caller input -- see the named constants below.
func expireOpen(ctx context.Context, tx *sql.Tx, column, value string) error {
	if _, err := tx.ExecContext(ctx,
		//nolint:gosec // column is one of three package-local constants, never request input
		`UPDATE engagement_offers SET state = 'expired'
		  WHERE `+column+` = $1 AND state = 'offered' AND expires_at <= now()`,
		value,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("offer: expire open offers: %w", err)
	}
	return nil
}

// The three columns expireOpen filters by, named so no call site can pass
// a string that came from a request.
const (
	byID           = "id"
	byEngagementID = "engagement_id"
	byStaffID      = "staff_id"
)

// The offer_state values (00030) this package compares against by name.
// The database is the authority on the set; these exist so a typo is a
// compile error rather than a condition that quietly never matches.
const (
	stateOffered  = "offered"
	stateDeclined = "declined"
)

// writeJSON encodes a 200 response body, the one shape every handler in
// this package shares.
func writeJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	// coverage:ignore reason: response encoding failure, not exercised by unit tests
	if err := json.NewEncoder(w).Encode(body); err != nil {
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
	}
}
