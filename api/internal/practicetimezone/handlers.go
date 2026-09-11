package practicetimezone

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/ianazone"
	"doula-cloud/api/internal/staffauth"
)

// actionTimezoneChanged records a change to a Practice's timezone --
// Practice-scoped, plain-string, the same shape
// actionPaymentTermsChanged uses for the sibling Practice-level setting.
const actionTimezoneChanged = "practice_timezone_changed"

// fieldTimezone is PutRequest's own json tag, and so the Details key its
// refusal is written under -- docs/api-design.md section 7 rule 4 keys
// Details by the DTO's field name, so a client maps it onto a control
// with no translation table.
const fieldTimezone = "timezone"

// Response is what both handlers return: the zone the Practice is
// currently reading its calendar days in.
type Response struct {
	Timezone string `json:"timezone"`
}

// PutRequest is PutHandler's body. PUT semantics, replacing the one zone
// a Practice has.
type PutRequest struct {
	Timezone string `json:"timezone"`
}

// timezoneDiff is PutHandler's activity diff. Both sides are always
// present: the column is NOT NULL, so a Practice always had some zone
// before, even if it was the one signup wrote.
type timezoneDiff struct {
	TimezoneBefore string `json:"timezoneBefore"`
	TimezoneAfter  string `json:"timezoneAfter"`
}

// GetHandler reads the Practice's own timezone. Owner and Admin only,
// declared at the mount, which departs from the AnyStaff read that
// payments' payment terms and billing mode carry: a Doula never meets
// the zone name on a screen, she meets the Visit type derived from it,
// and #1166 scopes the whole setting to the pair that may state it. A
// later ticket that wants to show a Doula which zone her list was typed
// in widens this, with its own reason.
//
// Must be mounted behind staffauth.Middleware.
func GetHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		name, err := readName(r.Context(), tx, practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, Response{Timezone: name})
	})
}

// PutHandler states a Practice's timezone. Owner and Admin only,
// declared at the mount (#990's form), the same authority that already
// writes the rate card and the payment terms: ADR-0036 puts a Practice's
// zone at that level, a fact about the business rather than a reader's
// preference.
//
// Idempotent by construction, following practicerate.PutRateHandler: a
// retry with the same body reads the same stored value, writes nothing,
// and records nothing new.
//
// Changing the zone retypes the Practice's existing Visits at once, and
// that is the intended behavior rather than a side effect to guard
// against: CONTEXT.md's Visit entry computes a Visit's type on every
// read, so there is nothing stored to migrate and no instant that moves.
// The screen says so before the press; a Practice that needs its old
// Visits typed in the old zone needs the per-Visit time model parked on
// #330, not a second column here.
//
// Must be mounted behind staffauth.Middleware.
func PutHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}

		var req PutRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		if _, err := ianazone.Parse(req.Timezone); err != nil {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument,
				fmt.Sprintf("timezone %q is not an IANA zone name", req.Timezone),
				map[string]string{fieldTimezone: ianazone.MsgNotRecognized})
			return
		}

		after := req.Timezone
		before, err := readName(r.Context(), tx, practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if before != after {
			if err := write(r.Context(), tx, practiceID, before, after); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
		}

		apierr.WriteJSON(w, http.StatusOK, Response{Timezone: after})
	})
}

// write persists the zone and records who changed it and when -- one
// write path, so the row and its Activity entry can never disagree about
// what happened.
func write(ctx context.Context, tx *sql.Tx, practiceID, before, after string) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE practices SET timezone = $1 WHERE id = $2`, after, practiceID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("practicetimezone: write practice timezone: %w", err)
	}

	diffJSON, err := json.Marshal(timezoneDiff{TimezoneBefore: before, TimezoneAfter: after})
	if err != nil {
		// coverage:ignore reason: marshal of a fixed, always-serializable struct never fails
		return fmt.Errorf("practicetimezone: marshal timezone diff: %w", err)
	}

	actorStaffID, _ := staffauth.StaffID(ctx)
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectPractice,
		SubjectID:   practiceID,
		Action:      actionTimezoneChanged,
		Diff:        diffJSON,
		Actor:       activity.StaffActor(actorStaffID),
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("practicetimezone: record timezone change: %w", err)
	}
	return nil
}
