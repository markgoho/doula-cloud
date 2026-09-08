package engagement

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// ownerRole is the practice_role enum member (00002_practice_staff_tenancy.sql)
// this file cares about (helpers_test.go names it a second time, for
// the external test package): correcting a frozen birth outcome is the one
// act ADR-0015 gives to an Owner alone: not an Admin, not a Doula, because
// it is a hatch rather than an editing surface.
const ownerRole = "owner"

// The three birth_outcome enum members (00092_engagement_birth_outcome.sql),
// exported so a caller elsewhere in the BFF -- and #294's living-baby
// derivation in particular -- names the same values this file validates
// against rather than hand-copying string literals.
const (
	OutcomeLiveBirth = "live_birth"
	OutcomeLoss      = "loss"
	OutcomeUnknown   = "unknown"
)

// dateLayout is the only shape pregnancyEndedOn is accepted in: a plain
// calendar date, the same `date`-column text form every other date the
// BFF hands out (Detail.DueDate) already uses.
const dateLayout = "2006-01-02"

// birthOutcomes is ADR-0015's fixed three-value vocabulary, checked in
// the handler so a caller gets a clean 400 rather than the database's
// own enum-cast error.
var birthOutcomes = map[string]bool{
	OutcomeLiveBirth: true,
	OutcomeLoss:      true,
	OutcomeUnknown:   true,
}

// BirthOutcomeRequest records what happened to the pregnancy, and when.
// Correction is the caller's explicit acknowledgment that an already
// recorded outcome is being overwritten: a plain record onto a frozen
// row is refused rather than silently applied, so the one act that can
// rewrite a woman's record is always deliberate.
type BirthOutcomeRequest struct {
	// BirthOutcome is nullable on the way in as well as on the row: a
	// null outcome with correction: true un-records one, which is the
	// state ADR-0015's own motivating typo needs. A loss entered on the
	// wrong Engagement leaves that Engagement never having had an
	// outcome, and 'unknown' would not say so -- 'unknown' means the
	// Practice looked and never learned.
	BirthOutcome     *string `json:"birthOutcome"`
	PregnancyEndedOn *string `json:"pregnancyEndedOn,omitempty"`
	Correction       bool    `json:"correction,omitempty"`
}

// BirthOutcomeResponse confirms the two facts the Engagement now holds.
type BirthOutcomeResponse struct {
	EngagementID     string  `json:"engagementId"`
	BirthOutcome     *string `json:"birthOutcome"`
	PregnancyEndedOn *string `json:"pregnancyEndedOn,omitempty"`
}

// birthOutcomeRow is the frozen side of the Engagement this handler
// reads before it writes: what the row holds now, which decides whether
// this request is a first recording, a no-op, or a correction.
type birthOutcomeRow struct {
	outcome *string
	endedOn *string
}

// RecordBirthOutcomeHandler records ADR-0015's birth outcome and the
// date the pregnancy ended (#293). Staff-only and never Client-facing:
// nothing in this file reaches a portal session, and neither column is
// read by package portal.
//
// Recorded whenever the fact becomes known, not only at completion -- a
// postpartum-only Engagement records live_birth at intake, because the
// baby was born before the Practice was hired. Any role that may move a
// status may record it (ADR-0006 as ADR-0015's role table applies it);
// once recorded, the pair is frozen by 00092's BEFORE UPDATE trigger and
// only an Owner may correct it, by sending correction: true. The
// database cannot know roles, so the trigger's door
// (app.allow_outcome_correction) is opened here and only here, after the
// Owner check has already passed.
//
// Re-sending the values the Engagement already holds is a no-op: no
// write, no audit row, 200 -- which is what lets this endpoint be PUT
// and carry no Idempotency-Key. Must be mounted behind
// staffauth.Middleware.
func RecordBirthOutcomeHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		reader, has := staffauth.ReaderFrom(r.Context())
		if !has {
			// coverage:ignore reason: staffauth.Middleware always places a Reader on context before this handler runs
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		engagementID := r.PathValue("engagementId")
		if !staffauth.ParseUUID(w, "engagement", engagementID) {
			return
		}
		if refuseFactWrite(w, reader, "a contractor Doula cannot record an Engagement's birth outcome") {
			return
		}

		var req BirthOutcomeRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		endedOn, valid := validateOutcome(w, req)
		if !valid {
			return
		}

		var current birthOutcomeRow
		err := tx.QueryRowContext(r.Context(),
			`SELECT birth_outcome::text, pregnancy_ended_on::text
			   FROM engagements WHERE id = $1 AND practice_id = $2`,
			engagementID, practiceID,
		).Scan(&current.outcome, &current.endedOn)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "engagement not found", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if unchanged(current, req.BirthOutcome, endedOn) {
			apierr.WriteJSON(w, http.StatusOK, BirthOutcomeResponse{
				EngagementID: engagementID, BirthOutcome: req.BirthOutcome, PregnancyEndedOn: endedOn,
			})
			return
		}
		if refuseFrozenWrite(w, reader, current.outcome != nil, req.Correction) {
			return
		}
		if req.Correction {
			// The trigger's door, opened for this transaction only, after
			// the Owner check above -- 00092's own comment names this
			// handler as its single caller.
			if _, err := tx.ExecContext(r.Context(),
				`SELECT set_config('app.allow_outcome_correction', 'on', true)`); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
		}

		if _, err := tx.ExecContext(r.Context(),
			`UPDATE engagements SET birth_outcome = $1, pregnancy_ended_on = $2::date WHERE id = $3`,
			req.BirthOutcome, endedOn, engagementID,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		actorStaffID, _ := staffauth.StaffID(r.Context())
		if err := recordOutcomeEvent(r.Context(), tx, outcomeEvent{
			practiceID:               practiceID,
			engagementID:             engagementID,
			previousBirthOutcome:     current.outcome,
			birthOutcome:             req.BirthOutcome,
			previousPregnancyEndedOn: current.endedOn,
			pregnancyEndedOn:         endedOn,
			actorStaffID:             &actorStaffID,
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, BirthOutcomeResponse{
			EngagementID: engagementID, BirthOutcome: req.BirthOutcome, PregnancyEndedOn: endedOn,
		})
	})
}

// validateOutcome checks req against ADR-0015's vocabulary and its
// engagements_outcome_is_dated rule before the database does, so a
// caller gets a named 400 rather than a constraint violation. It reports
// the normalized date to write, which is nil for an un-recording, and for
// an 'unknown' outcome the Practice has no date for.
func validateOutcome(w http.ResponseWriter, req BirthOutcomeRequest) (endedOn *string, valid bool) {
	if req.BirthOutcome == nil {
		if !req.Correction {
			apierr.WriteError(w,
				"birthOutcome may only be cleared as a correction", http.StatusBadRequest)
			return nil, false
		}
		if req.PregnancyEndedOn != nil && *req.PregnancyEndedOn != "" {
			apierr.WriteError(w,
				"an Engagement with no birth outcome carries no date either", http.StatusBadRequest)
			return nil, false
		}
		return nil, true
	}
	if !birthOutcomes[*req.BirthOutcome] {
		apierr.WriteError(w, "birthOutcome must be 'live_birth', 'loss' or 'unknown'", http.StatusBadRequest)
		return nil, false
	}
	if req.PregnancyEndedOn == nil || *req.PregnancyEndedOn == "" {
		if *req.BirthOutcome != OutcomeUnknown {
			apierr.WriteError(w,
				"pregnancyEndedOn is required unless the birth outcome is 'unknown'", http.StatusBadRequest)
			return nil, false
		}
		return nil, true
	}
	if _, err := time.Parse(dateLayout, *req.PregnancyEndedOn); err != nil {
		apierr.WriteError(w, "pregnancyEndedOn must be a date, as YYYY-MM-DD", http.StatusBadRequest)
		return nil, false
	}
	return req.PregnancyEndedOn, true
}

// unchanged reports whether the request asks for exactly what the row
// already holds -- the no-op that makes this endpoint safe to retry.
func unchanged(current birthOutcomeRow, outcome, endedOn *string) bool {
	return sameString(current.outcome, outcome) && sameString(current.endedOn, endedOn)
}

// sameString compares two nullable strings, treating null as a value of
// its own -- "not recorded" is a state this endpoint both reads and
// writes, so it has to compare equal to itself.
func sameString(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// refuseFrozenWrite guards ADR-0015's correction hatch, writing the
// refusal itself and reporting whether the request was refused -- the
// same polarity refuseFactWrite uses, so both gates read the same way at
// their call sites. A frozen outcome refuses a plain record with a 409
// whoever is asking -- an
// Owner included, because block-over-warn means the overwrite is always
// a deliberate act -- and refuses a correction from anyone but an Owner
// with a 403. A correction offered where nothing is recorded is refused
// too: there is nothing to correct, and accepting it would let the
// deliberate act be the caller's default.
func refuseFrozenWrite(w http.ResponseWriter, reader staffauth.Reader, frozen, correction bool) bool {
	switch {
	case frozen && !correction:
		apierr.Write(w, http.StatusConflict, apierr.CodeBirthOutcomeFrozen,
			"this Engagement already has a birth outcome; only a Practice Owner can correct it", nil)
		return true
	case frozen && !reader.Has(ownerRole):
		apierr.WriteError(w, "only a Practice Owner can correct a recorded birth outcome", http.StatusForbidden)
		return true
	case !frozen && correction:
		apierr.WriteError(w, "this Engagement has no birth outcome to correct", http.StatusConflict)
		return true
	}
	return false
}
