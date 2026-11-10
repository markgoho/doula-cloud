// Package staffinvitation declares a Staff Invitation's lifecycle once:
// the statuses practice_invitations stores, the one derivation that turns
// a stored status and an expiry into what the Invitation is, the named
// moves between them, and the audit row each move writes. It is the
// Invitation's counterpart of contracts/lifecycle.go, in a package of its
// own because both staffauth (which writes the row) and staffinvite (whose
// outbox reads it, and which staffauth imports) need the same derivation.
package staffinvitation

import (
	"context"
	"database/sql"
	"encoding/json/v2"
	"fmt"
	"time"

	"doula-cloud/api/internal/activity"
)

// Status is what practice_invitations.status stores. There are three:
// 00030's invitation_status enum also lists 'expired', and 00170's
// practice_invitations_lifecycle CHECK refuses it on every write, because
// expiry is a fact about the clock and is derived, never stored (see
// StateOf).
type Status string

// The three stored statuses.
const (
	StatusPending  Status = "pending"
	StatusAccepted Status = "accepted"
	StatusRevoked  Status = "revoked"
)

// State is what an Invitation is, as every reader sees it: a stored
// Status, plus StateExpired, which nothing stores.
type State string

// The four states. StateExpired is a pending row whose expires_at has
// passed.
const (
	StatePending  State = "pending"
	StateExpired  State = "expired"
	StateAccepted State = "accepted"
	StateRevoked  State = "revoked"
)

// StateOf is the one definition of an expired Invitation: still pending,
// and expires_at at or before now. Every reader asks it -- the accept
// screen's lookup, accept itself, the Staff screen's roster, and the
// outbox deciding whether to mail -- so "expired" means one thing
// whichever of them looks. Before 00170 there were two: the accept path
// flipped the column to 'expired' when someone opened a stale link, while
// the Staff screen judged pending rows by the clock, so a flipped
// Invitation left the screen and an unflipped one stayed on it marked
// expired. A lapsed Invitation stays 'pending' in the column, so it keeps
// its slot in practice_invitations_one_pending until it is withdrawn or
// sent again, and the Owner sees it to do either.
func StateOf(status Status, expiresAt, now time.Time) State {
	if status == StatusPending && !expiresAt.After(now) {
		return StateExpired
	}
	return State(status)
}

// Transition is one named move an Invitation can make, the states it may
// start from, and the activity action that records it. Every write to
// practice_invitations in staffauth names the Transition it performs
// (lifecycle_test.go's source scan), and states its Requires again in
// its own WHERE clause, so the row is never moved from a state the
// declaration does not allow even when two writers race.
type Transition struct {
	// Act names the move as the person refused would do it, for Check's
	// refusal sentence.
	Act string
	// Requires is every State this move may start from. Empty for
	// TransitionInvite, which starts from no open Invitation at all.
	Requires []State
	// Action is the activity row the move writes through Record. Empty
	// for TransitionAccept only: accepting writes the Membership's own
	// 'joined' row (staffauth.RecordMembershipEvent), and the Invitation
	// row itself keeps accepted_staff_id and accepted_at, so a second
	// row saying the same thing would print the same event twice on the
	// Practice's feed.
	Action activity.PracticeAction
}

// TransitionInvite creates an Invitation for an address with none open
// at this Practice. The SQL guard is practice_invitations_one_pending: an
// insert meeting an open row becomes TransitionReinvite instead.
var TransitionInvite = Transition{Act: "invite this address", Action: activity.ActionStaffInvited}

// TransitionReinvite rotates an open Invitation -- pending or lapsed -- to
// a fresh token, a fresh expiry, and the roles and employment type typed
// this time. Both states are status = 'pending' in the column, which is
// exactly the predicate of the ON CONFLICT target that performs it. The
// row keeps its original invited_by and created_at; who sent it again,
// and when, is this move's activity row.
var TransitionReinvite = Transition{
	Act:      "send it again",
	Requires: []State{StatePending, StateExpired},
	Action:   activity.ActionStaffInvitationResent,
}

// TransitionAccept turns a pending, unexpired Invitation into a
// Membership.
var TransitionAccept = Transition{Act: "accept it", Requires: []State{StatePending}}

// TransitionRevoke withdraws an open Invitation. A lapsed one may be
// withdrawn too: it still holds its address's slot, and withdrawing is
// one of the two ways to free it.
var TransitionRevoke = Transition{
	Act:      "revoke it",
	Requires: []State{StatePending, StateExpired},
	Action:   activity.ActionStaffInvitationRevoked,
}

// Transitions is every declared move, walked by lifecycle_test.go.
var Transitions = []Transition{TransitionInvite, TransitionReinvite, TransitionAccept, TransitionRevoke}

// Check reports whether current is one of the states t may start from
// (ok=true, no refusal), or, if not, the sentence a person reads saying
// what the Invitation is and what that stops ("This invitation has been
// accepted, so you cannot revoke it.").
func (t Transition) Check(current State) (ok bool, refusal string) {
	for _, s := range t.Requires {
		if s == current {
			return true, ""
		}
	}
	return false, fmt.Sprintf("%s, so you cannot %s.", stateClause(current), t.Act)
}

func stateClause(current State) string {
	switch current {
	case StatePending:
		return "This invitation is still waiting for an answer"
	case StateExpired:
		return "This invitation has expired"
	case StateAccepted:
		return "This invitation has been accepted"
	case StateRevoked:
		return "This invitation has been revoked"
	}
	return fmt.Sprintf("This invitation is %s", current)
}

// Diff is an Invitation move's activity diff: which Invitation, and --
// for an invite or a re-invite -- what it was sent with. It carries no
// address: the Invitation row holds that, and an address written into the
// ledger could not be taken back out of it.
type Diff struct {
	InvitationID   string    `json:"invitationId"`
	Roles          []string  `json:"roles,omitzero"`
	EmploymentType string    `json:"employmentType,omitzero"`
	ExpiresAt      time.Time `json:"expiresAt,omitzero"`
}

// Record writes t's activity row against the Practice, in tx, at the
// instant the move itself wrote (the caller's clock.Now(ctx)), so "who
// invited this person, who sent it again, who withdrew it, and when" has
// an answer on the Practice's feed. Must run with app.current_practice_id
// set to practiceID.
func Record(ctx context.Context, tx *sql.Tx, t Transition, practiceID, actorStaffID string, at time.Time, d Diff) error {
	if t.Action == "" {
		return fmt.Errorf("staffinvitation: %q records no activity row of its own", t.Act)
	}
	diff, err := json.Marshal(d)
	if err != nil {
		// coverage:ignore reason: a struct of strings and a time always marshals cleanly, not exercised by unit tests
		return fmt.Errorf("staffinvitation: marshal diff: %w", err)
	}
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectPractice,
		SubjectID:   practiceID,
		Action:      string(t.Action),
		Diff:        diff,
		Actor:       activity.StaffActor(actorStaffID),
		CreatedAt:   at,
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("staffinvitation: record %s: %w", t.Action, err)
	}
	return nil
}
