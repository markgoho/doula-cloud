package staffauth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
)

// existingStaff reports what the verified identity already holds, for
// signup's three-way branch (#745). It returns ok=false when there is no
// staff row at all, and a non-zero HTTP status when signup must stop --
// either because the caller already belongs to a Practice, or because
// the lookup itself failed.
//
// Membership count, not the staff row alone, is what separates "resume
// this signup" from "you are already somewhere": a staff row with no
// Membership is exactly the person this ticket is about -- a signup that
// half-landed, or a Staff member whose last Membership was removed --
// and both of them are starting a Practice, not repeating one.
func existingStaff(ctx context.Context, tx *sql.Tx, identityUID string) (staffID, workState string, ok bool, status int, msg string) {
	err := tx.QueryRowContext(ctx,
		`SELECT id, work_state FROM staff WHERE identity_uid = $1`, identityUID,
	).Scan(&staffID, &workState)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", false, 0, ""
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", "", false, http.StatusInternalServerError, MsgInternalError
	}

	memberships, err := listMemberships(ctx, tx, staffID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", "", false, http.StatusInternalServerError, MsgInternalError
	}
	if len(memberships) > 0 {
		return "", "", false, http.StatusConflict, MsgAlreadyBelongsToPractice
	}
	return staffID, workState, true, 0, ""
}

// recordSignupPerson writes what this form said about the person: a
// first work-state assertion for a staff row the request created, and
// for one it resumed, the name and work state she has just given as
// current answers rather than a repeat of what the row already held.
// A first assertion and a correction are different facts about the same
// person -- see RecordWorkStateChange's own comment on why they are two
// functions -- and only the second has a previous value to move from.
//
// The resumed path moves the stored work state in the same breath as the
// event, the way UpdateWorkStateHandler does, so the column and the event
// log cannot disagree about where she says she works.
//
// Her email is the one field this deliberately does not take. It is the
// address she authenticated with, so it already matches the staff row,
// and moving a Staff email is #613's own flow -- it has to move in
// Identity Platform too, which this endpoint cannot do.
func recordSignupPerson(ctx context.Context, tx *sql.Tx, staffID string, req SignupRequest, previousWorkState string, resuming bool) error {
	if !resuming {
		return RecordFirstWorkStateAssertion(ctx, tx, staffID, req.WorkState, staffID)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE staff SET name = $1, work_state = $2, work_state_reported_at = now() WHERE id = $3`,
		req.StaffName, req.WorkState, staffID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("staffauth: move name and work state on resumed signup: %w", err)
	}
	return RecordWorkStateChange(ctx, tx, staffID, previousWorkState, req.WorkState, staffID)
}
