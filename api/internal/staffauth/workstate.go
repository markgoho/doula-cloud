package staffauth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/authn"
)

// MsgWorkStateRequired is what a caller who omitted a work state, or sent
// something that is not a US state, gets back. One literal, shared by
// signup and invitation acceptance, because a person meets whichever of
// the two her Practice sends her to and should read the same sentence.
const MsgWorkStateRequired = "workState is required, and must be a two-letter US state abbreviation"

// usStates is the 50 states and the District of Columbia as USPS
// two-letter abbreviations -- the same set 00043's staff_work_state_usps
// CHECK constraint holds. Duplicated deliberately: the constraint is what
// makes a bad value impossible, and this is what makes a bad value a 400
// rather than a 500.
var usStates = map[string]bool{
	"AL": true, "AK": true, "AZ": true, "AR": true, "CA": true, "CO": true,
	"CT": true, "DE": true, "DC": true, "FL": true, "GA": true, "HI": true,
	"ID": true, "IL": true, "IN": true, "IA": true, "KS": true, "KY": true,
	"LA": true, "ME": true, "MD": true, "MA": true, "MI": true, "MN": true,
	"MS": true, "MO": true, "MT": true, "NE": true, "NV": true, "NH": true,
	"NJ": true, "NM": true, "NY": true, "NC": true, "ND": true, "OH": true,
	"OK": true, "OR": true, "PA": true, "RI": true, "SC": true, "SD": true,
	"TN": true, "TX": true, "UT": true, "VT": true, "VA": true, "WA": true,
	"WV": true, "WI": true, "WY": true,
}

// NormalizeWorkState trims and upper-cases what the caller sent and
// reports whether it names a US state. It is the server-side half of the
// requirement: the form offers a fixed <select>, but a form is not an
// enforcement boundary, so nothing reaches the database on the strength
// of the browser having behaved.
func NormalizeWorkState(raw string) (string, bool) {
	s := strings.ToUpper(strings.TrimSpace(raw))
	return s, usStates[s]
}

// RecordFirstWorkStateAssertion appends the onboarding row to the audit
// trail: what this person said her work state was, and when. It leaves
// previous_work_state NULL, which is what a first assertion means -- there
// was no earlier value.
//
// A change to an existing value is a different row with both sides
// filled in: RecordWorkStateChange, below.
//
// actorStaffID is passed rather than assumed equal to staffID: only the
// person herself may assert where she works, so the two are always equal
// today, and a row where they differ is a signal rather than an
// impossibility.
func RecordFirstWorkStateAssertion(ctx context.Context, tx *sql.Tx, staffID, workState, actorStaffID string) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO staff_work_state_events (staff_id, work_state, actor_staff_id)
		 VALUES ($1, $2, $3)`,
		staffID, workState, actorStaffID,
	)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return fmt.Errorf("staffauth: record first work state assertion: %w", err)
	}
	return nil
}

// RecordWorkStateChange appends the row that says a work state moved:
// what it was, what it became, who said so, and when. The sibling of
// RecordFirstWorkStateAssertion, and deliberately a second function
// rather than a nullable argument on the first -- a first assertion and
// a correction are different facts about the same person, and a caller
// that has no previous value should not be able to express one.
//
// previousWorkState is never empty on this path: a staff row cannot exist
// without a work state (00043 made the column NOT NULL), so a change
// always has something to change from.
//
// The two sides may be equal. Saving the same state again is a
// re-assertion rather than a no-op: work_state_reported_at is the only
// staleness signal the design has, nothing prompts anyone to refresh it,
// and a Practice substantiating an ST-100 three years later wants "yes,
// still New York, as of today" on the record rather than an unexplained
// gap. See UpdateWorkStateHandler, which does not short-circuit.
func RecordWorkStateChange(ctx context.Context, tx *sql.Tx, staffID, previousWorkState, workState, actorStaffID string) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO staff_work_state_events (staff_id, previous_work_state, work_state, actor_staff_id)
		 VALUES ($1, $2, $3, $4)`,
		staffID, previousWorkState, workState, actorStaffID,
	)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return fmt.Errorf("staffauth: record work state change: %w", err)
	}
	return nil
}

// UpdateWorkStateRequest is the whole body of a work state correction:
// the state she now works from, as a USPS two-letter abbreviation. There
// is deliberately no staff id anywhere in this request or in the route
// that carries it -- see UpdateWorkStateHandler.
type UpdateWorkStateRequest struct {
	WorkState string `json:"workState"`
}

// WorkStateResponse is what a correction returns: the stored value and
// the moment it was asserted. The date is echoed rather than left for the
// caller to guess at, because the screen prints it back as "self-reported
// <date>" and a clock read in the browser is not the one the audit row
// carries.
type WorkStateResponse struct {
	WorkState           string    `json:"workState"`
	WorkStateReportedAt time.Time `json:"workStateReportedAt"`
}

// UpdateWorkStateHandler lets a Staff member correct where she works
// (#437). Before it there was no way at all: #415 required the fact at
// both doors into staff and then left it frozen, so a doula who moved
// from New York to New Jersey had no way to say so and her Practice's
// sales tax went quietly wrong from that day -- nothing errored, the
// apportionment simply used a stale fact.
//
// Self-edit only, and enforced by shape rather than by a check: the
// route carries no staff id, and the row written is the one the session
// cookie's identity resolves to. An Owner or an Admin reading the value
// on the roster has no way to name anyone else's row, so there is no
// authorization branch here to get wrong. staff_self_update (00044) is
// the same rule again at the boundary that can actually enforce it.
//
// Mounted outside the Practice-scoped middleware on purpose. A work
// state is a fact about a person and not about a Membership (00043), so
// it belongs to no Practice; app.current_practice_id stays unset, which
// is the window both of 00044's policies are scoped to.
func UpdateWorkStateHandler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, uid, ok := authn.Begin(w, r, db)
		if !ok {
			return
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		var req UpdateWorkStateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		// The same normalizer and the same sentence the two onboarding
		// paths use: a person who met the question at signup and meets it
		// again here should not be told off in different words.
		normalized, valid := NormalizeWorkState(req.WorkState)
		if !valid {
			http.Error(w, MsgWorkStateRequired, http.StatusBadRequest)
			return
		}

		resp, status, msg := updateWorkState(r.Context(), tx, uid, normalized)
		if status != http.StatusOK {
			http.Error(w, msg, status)
			return
		}

		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed = true

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// updateWorkState reads the outgoing value, writes the new one, and
// appends the audit row -- one transaction, so a correction that fails
// halfway leaves neither a changed value with no event nor an event
// describing a change that did not happen.
//
// The read of the previous value is what makes the event row a change
// rather than an assertion, and it must happen before the UPDATE for the
// obvious reason.
//
// No short-circuit when the value is unchanged: see RecordWorkStateChange
// for why a re-assertion is a real act.
func updateWorkState(ctx context.Context, tx *sql.Tx, identityUID, workState string) (WorkStateResponse, int, string) {
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_identity_uid', $1, true)`, identityUID); err != nil {
		return WorkStateResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	var staffID, previous string
	err := tx.QueryRowContext(ctx,
		`SELECT id, work_state FROM staff WHERE identity_uid = $1`, identityUID,
	).Scan(&staffID, &previous)
	if errors.Is(err, sql.ErrNoRows) {
		return WorkStateResponse{}, http.StatusNotFound, "no matching staff account"
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return WorkStateResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	// work_state_reported_at moves in the same statement as the value, so
	// the two can never disagree about when the current answer was given.
	var reportedAt time.Time
	if err := tx.QueryRowContext(ctx,
		`UPDATE staff SET work_state = $1, work_state_reported_at = now()
		  WHERE id = $2
		  RETURNING work_state_reported_at`,
		workState, staffID,
	).Scan(&reportedAt); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return WorkStateResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	if err := RecordWorkStateChange(ctx, tx, staffID, previous, workState, staffID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return WorkStateResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	return WorkStateResponse{WorkState: workState, WorkStateReportedAt: reportedAt}, http.StatusOK, ""
}
