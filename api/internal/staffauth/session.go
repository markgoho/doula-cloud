package staffauth

import (
	"context"
	"database/sql"
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
	// SoleOwner is whether this person is the only Owner of at least one
	// Practice (#615's saved-recovery-code population, read through
	// staff_is_sole_owner). It rides on this response for the same reason
	// the work state does (#437): the account screen has to know whether
	// to offer her saved recovery codes at all, and the only endpoint that
	// could otherwise tell her -- the rotate -- answers by destroying the
	// set she holds. A second round trip to learn a fact this one can
	// carry is a round trip nobody needs.
	//
	// Drawing only, never a gate: POST /api/staff/mfa-recovery/saved-codes/
	// rotate re-derives the same predicate and 403s on its own (ADR-0006).
	SoleOwner bool `json:"soleOwner"`
}

// SessionHandler resolves the verified caller to a Staff row and reports
// their Practice memberships. It runs before any Practice is chosen, so
// -- like SignupHandler -- it only ever sets app.current_identity_uid,
// never app.current_practice_id.
func SessionHandler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, uid, secondFactor, ok := authn.Begin(w, r, db, authn.TierStaff)
		if !ok {
			return
		}
		defer func() { _ = tx.Rollback() }()

		// #1182 settled that this handler belongs behind the same seam
		// as the rest of the family rather than beside it. It reads more
		// of the row than the others do, which is why the seam returns
		// the row and not only its id -- the columns this response is
		// built from are the ones requireSelf already fetched, so
		// joining the family costs it no second query.
		self, ok := requireSelf(w, r, tx, uid)
		if !ok {
			return
		}

		resp, err := resolveSession(r, tx, self, secondFactor)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, resp)
	})
}

func resolveSession(r *http.Request, tx *sql.Tx, self selfStaff, secondFactor bool) (SessionResponse, error) {
	ctx := r.Context()
	staffID := self.ID

	memberships, err := listMemberships(ctx, tx, staffID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SessionResponse{}, err
	}

	soleOwner, err := isSoleOwnerAnywhere(ctx, tx, staffID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SessionResponse{}, err
	}

	resp := SessionResponse{
		StaffID:             staffID,
		Name:                self.Name,
		Email:               self.Email,
		WorkState:           self.WorkState,
		WorkStateReportedAt: self.WorkStateReportedAt,
		Memberships:         memberships,
		SecondFactor:        secondFactor,
		SoleOwner:           soleOwner,
	}
	if self.LastPracticeID.Valid {
		resp.LastPracticeID = &self.LastPracticeID.String
	}
	return resp, nil
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
