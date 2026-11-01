package staffauth

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/sessionmint"
)

// MsgPilotNotOpen is what signup answers an address the founder has not
// let into the pilot, while the pilot gate is closed (#1790). The signup
// screen switches on apierr.CodePilotNotOpen and shows its own screen,
// so this sentence is for a caller reading the API, and for the screen's
// fallback if it ever meets the refusal without the code.
const MsgPilotNotOpen = "DoulaCloud opens in January 2027. Until then, only a Practice in the pilot can sign up."

// pilotNotOpen is signup's refusal of an address pilotAdmits says no to.
func pilotNotOpen() *sessionmint.Refusal {
	return &sessionmint.Refusal{Status: http.StatusForbidden, Code: apierr.CodePilotNotOpen, Message: MsgPilotNotOpen}
}

// pilotAdmits answers whether address may create a Practice: yes while
// the pilot gate is open (opened in ops, #2485), and otherwise only if
// the founder has added it to the allowlist (in ops, #2484). address is already in NormalizeAddress's spelling,
// which is the spelling 00125 stores, so the match is case-insensitive.
//
// It reads through pilot_admits (00125), a yes-or-no question about one
// address, because signup runs for any verified identity and a policy it
// could satisfy would hand every caller the whole list.
func pilotAdmits(ctx context.Context, tx *sql.Tx, address string) (bool, error) {
	var admitted bool
	if err := tx.QueryRowContext(ctx, `SELECT pilot_admits($1)`, address).Scan(&admitted); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, fmt.Errorf("staffauth: ask pilot_admits: %w", err)
	}
	return admitted, nil
}

// LooksLikeAddress is GOV.UK's own email-address rule, no more: an "@"
// with something either side. Anything stricter refuses addresses that
// really deliver. One copy for both populations: clientauth's address
// change reads it, and so does ops's pilot allowlist add (#2484).
func LooksLikeAddress(address string) bool {
	at := strings.Index(address, "@")
	return at > 0 && at < len(address)-1
}
