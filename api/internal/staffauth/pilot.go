package staffauth

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/internalauth"
	"doula-cloud/api/internal/sessionmint"
)

// MsgPilotNotOpen is what signup answers an address the founder has not
// let into the pilot, while the pilot gate is closed (#1790). The signup
// screen switches on apierr.CodePilotNotOpen and shows its own screen,
// so this sentence is for a caller reading the API, and for the screen's
// fallback if it ever meets the refusal without the code.
const MsgPilotNotOpen = "DoulaCloud opens in January 2027. Until then, only a Practice in the pilot can sign up."

// pilotOperatorDoor is the transaction-local setting 00125's policies
// read. Only the two operator endpoints below set it, and only after the
// internal guard has admitted the caller.
const pilotOperatorDoor = `SELECT set_config('app.pilot_operator_trusted', 'true', true)`

// pilotNotOpen is signup's refusal of an address pilotAdmits says no to.
var pilotNotOpen = sessionmint.Refusal{Status: http.StatusForbidden, Code: apierr.CodePilotNotOpen, Message: MsgPilotNotOpen}

// pilotAdmits answers whether address may create a Practice: yes while
// the pilot gate is open, and otherwise only if the founder has added it
// to the allowlist. address is already in NormalizeAddress's spelling,
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

// PilotAllowlistRequest is the body of a POST to PilotAllowlistHandler:
// the address the founder is letting into the pilot, and who is doing it.
type PilotAllowlistRequest struct {
	Address string `json:"address"`
	AddedBy string `json:"addedBy"`
}

// PilotAllowlistEntry is one address on the pilot allowlist, as stored.
type PilotAllowlistEntry struct {
	Address string    `json:"address"`
	AddedBy string    `json:"addedBy"`
	AddedAt time.Time `json:"addedAt"`
}

// PilotAllowlistHandler adds an address to the pilot allowlist (#1790).
//
// An operator endpoint behind the internal guard, the same shape as the
// founding grant: the founder adds an address by hand after speaking
// with the doula, and adding it is the act that lets her into the pilot.
// "When was she let in, and by whom?" is what the row answers.
//
// Adding an address already on the list succeeds and changes nothing:
// the answer is the first record, so a second add can never rewrite who
// let her in or when.
func PilotAllowlistHandler(db *sql.DB, auth *internalauth.Guard) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !auth.Require(w, r) {
			return
		}

		var req PilotAllowlistRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		address := NormalizeAddress(req.Address)
		if !looksLikeAddress(address) {
			apierr.WriteError(w, "address must be an email address", http.StatusBadRequest)
			return
		}
		addedBy := strings.TrimSpace(req.AddedBy)
		if addedBy == "" {
			apierr.WriteError(w, "addedBy is required", http.StatusBadRequest)
			return
		}

		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			// coverage:ignore reason: DB connection failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		// A no-op once Commit below has run: every refusal on this
		// endpoint is answered before the transaction begins.
		defer func() { _ = tx.Rollback() }()

		// coverage:ignore reason: DB query failure, not exercised by unit tests
		if _, err := tx.ExecContext(r.Context(), pilotOperatorDoor); err != nil {
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO pilot_allowlist (address, added_by, added_at) VALUES ($1, $2, $3)
			 ON CONFLICT (address) DO NOTHING`,
			address, addedBy, clock.Now(r.Context()),
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		entry := PilotAllowlistEntry{Address: address}
		if err := tx.QueryRowContext(r.Context(),
			`SELECT added_by, added_at FROM pilot_allowlist WHERE address = $1`, address,
		).Scan(&entry.AddedBy, &entry.AddedAt); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, entry)
	})
}

// PilotGateOpenRequest is the body of a POST to PilotGateOpenHandler: who
// is opening the pilot gate.
type PilotGateOpenRequest struct {
	OpenedBy string `json:"openedBy"`
}

// PilotGate is the pilot gate once it is open: who opened it, and when.
type PilotGate struct {
	OpenedBy string    `json:"openedBy"`
	OpenedAt time.Time `json:"openedAt"`
}

// PilotGateOpenHandler opens the pilot gate (#1790): from then on signup
// does not ask the allowlist.
//
// It is the one deliberate act ADR-0043 asks for -- the gate opens in
// January because the founder opened it, not because a date passed --
// and the row records who did it and when. An environment variable would
// leave only the deploy history to answer that. Opening an open gate
// succeeds and keeps the first record, and nothing closes it again.
func PilotGateOpenHandler(db *sql.DB, auth *internalauth.Guard) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !auth.Require(w, r) {
			return
		}

		var req PilotGateOpenRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		openedBy := strings.TrimSpace(req.OpenedBy)
		if openedBy == "" {
			apierr.WriteError(w, "openedBy is required", http.StatusBadRequest)
			return
		}

		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			// coverage:ignore reason: DB connection failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		// A no-op once Commit below has run: every refusal on this
		// endpoint is answered before the transaction begins.
		defer func() { _ = tx.Rollback() }()

		// coverage:ignore reason: DB query failure, not exercised by unit tests
		if _, err := tx.ExecContext(r.Context(), pilotOperatorDoor); err != nil {
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if _, err := tx.ExecContext(r.Context(),
			`UPDATE pilot_gate SET opened_by = $1, opened_at = $2 WHERE opened_at IS NULL`,
			openedBy, clock.Now(r.Context()),
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		var gate PilotGate
		if err := tx.QueryRowContext(r.Context(),
			`SELECT opened_by, opened_at FROM pilot_gate`,
		).Scan(&gate.OpenedBy, &gate.OpenedAt); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, gate)
	})
}

// looksLikeAddress is GOV.UK's own email-address rule, no more: an "@"
// with something either side -- the rule clientauth's address change
// already applies. Anything stricter would refuse an address that really
// delivers.
func looksLikeAddress(address string) bool {
	at := strings.Index(address, "@")
	return at > 0 && at < len(address)-1
}
