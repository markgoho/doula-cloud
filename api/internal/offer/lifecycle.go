package offer

import (
	"fmt"
	"slices"
	"time"
)

// State is an Offer's state: the offer_state enum (00030), as every
// reader sees it at an instant. The column stores all six, but what it
// stores is not on its own what the Offer is -- an 'offered' row whose
// expires_at has passed is expired whether or not anything has written
// that down yet. StateAt is the one answer.
type State string

// The six states.
const (
	StateOffered    State = "offered"
	StateAccepted   State = "accepted"
	StateDeclined   State = "declined"
	StateWithdrawn  State = "withdrawn"
	StateSuperseded State = "superseded"
	StateExpired    State = "expired"
)

// StateAt is the one definition of what an Offer is at an instant: its
// stored state, except that an 'offered' row whose expires_at is at or
// before at has lapsed. offer_state_at (00174) is the same rule in SQL,
// for the readers that ask in a query; lifecycle_schema_test.go proves
// the two agree at the edge. Neither depends on the lazy materialization
// (TransitionLapse) having run.
//
// at is clock.Now(ctx): expires_at is written from that clock, so the
// lapse is read on it too (#1992).
func StateAt(stored State, expiresAt, at time.Time) State {
	if stored == StateOffered && !expiresAt.After(at) {
		return StateExpired
	}
	return stored
}

// OpenAt reports whether an Offer is still open, and so still decidable,
// at the instant at. offer_open_at (00174) is the SQL form, and every
// guard on a write states it.
func OpenAt(stored State, expiresAt, at time.Time) bool {
	return StateAt(stored, expiresAt, at) == StateOffered
}

// terminal reports whether an Offer has stopped being decidable. #230
// hangs a rule on this: the Client's own fields stop being served the
// moment an Offer reaches any terminal state, so a stale link opens a
// closed Offer rather than a stranger's due date. It is asked of a
// state already derived by StateAt or offer_state_at.
func (s State) terminal() bool {
	return s != StateOffered
}

// DecidedBy is who closed an Offer: the offer_decider_kind enum (00174).
// decided_by names a Staff row, and three of the ways an Offer closes
// have none to name -- ADR-0008 forbids inventing one -- so the kind is
// what tells those three apart, and engagement_offers_lifecycle refuses
// a closed Offer that does not say which it was.
type DecidedBy string

// The four deciders.
const (
	// DecidedByStaff is a Staff member, named in decided_by.
	DecidedByStaff DecidedBy = "staff"
	// DecidedByInvitee is the holder of the emailed link and code,
	// declining before she has an account (DeclineByTokenHandler).
	DecidedByInvitee DecidedBy = "invitee"
	// DecidedByCompletion is the Engagement's completion, withdrawing
	// whatever was still open (CloseOnCompletion). Who completed it is
	// recorded on the Engagement.
	DecidedByCompletion DecidedBy = "completion"
	// DecidedByLapse is nobody: expires_at passed.
	DecidedByLapse DecidedBy = "lapse"
)

// Transition is one named move an Offer can make: the states it may
// start from, the state it ends in, and who the row records as having
// made it. Every write of engagement_offers.state goes through a
// Transition's update (lifecycle_test.go's source scan), which states
// the start state again in its own WHERE clause, so a row is never moved
// from a state the declaration does not allow even when two writers race
// -- and never moved by a reader that missed the lapse.
type Transition struct {
	// Act names the move as the person refused would do it, for Check's
	// refusal sentence.
	Act string
	// Requires is every State, as StateAt derives it, this move may start
	// from. Empty for TransitionOffer, which starts from no open Offer for
	// the same person on the same work.
	Requires []State
	// To is the state the move leaves the Offer in.
	To State
	// By is who the row records as having made the move. Empty for
	// TransitionOffer: an open Offer has no decider.
	By DecidedBy
}

// TransitionOffer sends an Offer: an INSERT, whose guard is create's
// hasOpenOffer read and 00030's two partial unique indexes.
var TransitionOffer = Transition{Act: "offer this work", To: StateOffered}

// TransitionAccept takes an open Offer. Its acceptance is what mints the
// granted Attachment.
var TransitionAccept = Transition{Act: "accept it", Requires: []State{StateOffered}, To: StateAccepted, By: DecidedByStaff}

// TransitionDecline turns an open Offer down from her own inbox.
var TransitionDecline = Transition{Act: "decline it", Requires: []State{StateOffered}, To: StateDeclined, By: DecidedByStaff}

// TransitionDeclineByToken turns an open Offer down through the emailed
// link and code, before she has an account.
var TransitionDeclineByToken = Transition{Act: "decline it", Requires: []State{StateOffered}, To: StateDeclined, By: DecidedByInvitee}

// TransitionWithdraw is an Owner or an Admin taking an open Offer back.
var TransitionWithdraw = Transition{Act: "withdraw it", Requires: []State{StateOffered}, To: StateWithdrawn, By: DecidedByStaff}

// TransitionSupersede closes every other open Offer on the work when one
// is accepted, recording the accepter as its cause (ADR-0008).
var TransitionSupersede = Transition{Act: "give it to someone else", Requires: []State{StateOffered}, To: StateSuperseded, By: DecidedByStaff}

// TransitionCloseOnCompletion withdraws every Offer still open on an
// Engagement that has completed.
var TransitionCloseOnCompletion = Transition{Act: "withdraw it", Requires: []State{StateOffered}, To: StateWithdrawn, By: DecidedByCompletion}

// TransitionLapse writes down a lapse the clock has already decided: an
// Offer StateAt calls expired that the column still holds as 'offered'.
// It records decided_at as expires_at, the instant it lapsed, not the
// instant somebody looked, so a row materialized late reads the same as
// one materialized on time. No reader needs it to have run; it frees the
// row's slot in 00030's one-open-per-target indexes before a fresh Offer
// is sent, and it runs before every other move on the same work, so a
// lapsed Offer ends as 'expired' and never as 'withdrawn' or
// 'superseded'.
var TransitionLapse = Transition{Act: "let it lapse", Requires: []State{StateExpired}, To: StateExpired, By: DecidedByLapse}

// Transitions is every declared move, walked by lifecycle_test.go.
var Transitions = []Transition{
	TransitionOffer, TransitionAccept, TransitionDecline, TransitionDeclineByToken,
	TransitionWithdraw, TransitionSupersede, TransitionCloseOnCompletion, TransitionLapse,
}

// Check reports whether current is one of the states t may start from
// (ok=true, no refusal), or, if not, the sentence a person reads saying
// what the Offer is and what that stops ("This Offer has expired, so you
// cannot accept it.").
func (t Transition) Check(current State) (ok bool, refusal string) {
	if slices.Contains(t.Requires, current) {
		return true, ""
	}
	return false, t.refusal(current)
}

func (t Transition) refusal(current State) string {
	return fmt.Sprintf("%s, so you cannot %s.", stateClause(current), t.Act)
}

func stateClause(current State) string {
	switch current {
	case StateOffered:
		return "This Offer is still open"
	case StateAccepted:
		return "This Offer has been accepted"
	case StateDeclined:
		return "This Offer has been declined"
	case StateWithdrawn:
		return "This Offer has been withdrawn"
	case StateSuperseded:
		return "This work has gone to someone else"
	case StateExpired:
		return "This Offer has expired"
	}
	return fmt.Sprintf("This Offer is %s", current)
}

// update is the one UPDATE that moves Offers along t. $1 is the instant
// (clock.Now(ctx)), $2 the deciding Staff member -- NULL for every decider
// but DecidedByStaff, which engagement_offers_lifecycle holds it to --
// and filter selects which Offers, with its own parameters from $3 on.
// returning, when not empty, is the RETURNING list.
//
// The guard is t's Requires in SQL: offer_open_at for a decision, and for
// TransitionLapse a row still stored 'offered' that offer_open_at says
// is not open. Both read the instant passed, never now().
func (t Transition) update(filter, returning string) string {
	guard := `offer_open_at(state, expires_at, $1::timestamptz)`
	decidedAt := `$1::timestamptz`
	if t.By == DecidedByLapse {
		guard = `state = 'offered' AND NOT offer_open_at(state, expires_at, $1::timestamptz)`
		decidedAt = `expires_at`
	}
	q := `UPDATE engagement_offers
	    SET state = '` + string(t.To) + `', decided_at = ` + decidedAt + `,
	        decided_by = $2::uuid, decided_by_kind = '` + string(t.By) + `'
	  WHERE ` + guard + ` AND (` + filter + `)`
	if returning != "" {
		q += ` RETURNING ` + returning
	}
	return q
}
