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

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
)

// Membership is one Practice a Staff member belongs to, and the roles
// they hold there.
type Membership struct {
	PracticeID   string   `json:"practiceId"`
	PracticeName string   `json:"practiceName"`
	Roles        []string `json:"roles"`
}

// SessionResponse is what the frontend needs to decide where to land a
// Staff member after login: their memberships (to auto-redirect when
// there's exactly one, or offer a picker when there's more than one) and
// their last-used Practice (to skip the picker on a returning visit).
// It is also the whole read behind /account (#437), the per-person
// screen where a Staff member corrects her work state. That screen is
// not scoped to a Practice -- a work state is a fact about the person
// (00043) -- so this endpoint, which already answers "who am I" before
// any Practice is chosen, is where the fact belongs rather than a second
// route saying the same thing. Name rides along for the same reason:
// the accept-invite screen shows a returning contractor what her
// existing account already holds instead of asking for it and throwing
// the answer away.
type SessionResponse struct {
	StaffID string `json:"staffId"`
	Name    string `json:"name"`
	// Email is here for the shell's avatar menu (#452), which shows the
	// person the account they are signed in as. A Staff member can hold
	// Memberships at several Practices under one account, so "which
	// account is this" is a real question at the top right of every
	// screen, and the answer is her email rather than her name.
	Email string `json:"email"`
	// WorkState is the US state she works from, and WorkStateReportedAt
	// is when she last asserted it -- the pair the roster prints as "New
	// York -- self-reported 28 Aug 2026", shown to her here so she can
	// see the value she is being taxed on and how old it is.
	WorkState           string       `json:"workState"`
	WorkStateReportedAt time.Time    `json:"workStateReportedAt"`
	LastPracticeID      *string      `json:"lastPracticeId,omitempty"`
	Memberships         []Membership `json:"memberships"`
	// SecondFactor is this session's own second-factor fact (#606,
	// authn.Begin), not a query of her current Identity Platform
	// enrolment -- decision 3's session-carried claim, read straight
	// through. It is what the account screen offers "Enrol" or "Remove"
	// against: a person enrolled on another device still sees "Enrol"
	// here until she signs in again on this one, at which point the
	// TOTP challenge fires and the next session carries it.
	SecondFactor bool `json:"secondFactor"`
}

// SessionHandler resolves the verified caller to a Staff row and reports
// their Practice memberships. It runs before any Practice is chosen, so
// -- like SignupHandler -- it only ever sets app.current_identity_uid,
// never app.current_practice_id.
func SessionHandler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, uid, secondFactor, ok := authn.Begin(w, r, db)
		if !ok {
			return
		}
		defer func() { _ = tx.Rollback() }()

		resp, status, msg := resolveSession(r, tx, uid, secondFactor)
		if status != http.StatusOK {
			apierr.WriteError(w, msg, status)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
		}
	})
}

func resolveSession(r *http.Request, tx *sql.Tx, identityUID string, secondFactor bool) (SessionResponse, int, string) {
	ctx := r.Context()

	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_identity_uid', $1, true)`, identityUID); err != nil {
		return SessionResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	var staffID, name, email, workState string
	var workStateReportedAt time.Time
	var lastPracticeID sql.NullString
	err := tx.QueryRowContext(ctx,
		`SELECT id, name, email, work_state, work_state_reported_at, last_practice_id
		   FROM staff WHERE identity_uid = $1`, identityUID,
	).Scan(&staffID, &name, &email, &workState, &workStateReportedAt, &lastPracticeID)
	if errors.Is(err, sql.ErrNoRows) {
		return SessionResponse{}, http.StatusNotFound, MsgNoMatchingStaffAccount
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SessionResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	memberships, err := listMemberships(ctx, tx, staffID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SessionResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	resp := SessionResponse{
		StaffID:             staffID,
		Name:                name,
		Email:               email,
		WorkState:           workState,
		WorkStateReportedAt: workStateReportedAt,
		Memberships:         memberships,
		SecondFactor:        secondFactor,
	}
	if lastPracticeID.Valid {
		resp.LastPracticeID = &lastPracticeID.String
	}
	return resp, http.StatusOK, ""
}

func listMemberships(ctx context.Context, tx *sql.Tx, staffID string) ([]Membership, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT pm.practice_id, p.name, array_to_string(pm.roles, ',')
		 FROM practice_memberships pm
		 JOIN practices p ON p.id = pm.practice_id
		 WHERE pm.staff_id = $1
		 ORDER BY p.name`,
		staffID,
	)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return nil, fmt.Errorf("staffauth: list memberships: %w", err)
	}
	defer func() { _ = rows.Close() }()

	memberships := []Membership{}
	for rows.Next() {
		var m Membership
		var roles string
		if err := rows.Scan(&m.PracticeID, &m.PracticeName, &roles); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("staffauth: scan membership row: %w", err)
		}
		if roles != "" {
			m.Roles = strings.Split(roles, ",")
		}
		memberships = append(memberships, m)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("staffauth: iterate membership rows: %w", err)
	}
	return memberships, nil
}
