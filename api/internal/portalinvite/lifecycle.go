package portalinvite

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"time"
)

// State is a Client's portal access: where her client_portal_users row
// stands, named once (00168_portal_access_state.sql). Before it, the
// stage was read off four nullable columns in whichever order a reader
// asked, and the answer fell through to the outbox's own word -- so an
// invite past its expiry, or one a merge or an erasure had revoked,
// still read "Invite sent".
type State string

// The stored states, matching the portal_access_state enum, plus the
// one derived state. Expired is never written: it is an invited row past
// invite_token_expires_at, and Derive is the only place that says so.
const (
	StateInvited  State = "invited"
	StateRefused  State = "refused"
	StateAccepted State = "accepted"
	StateRevoked  State = "revoked"
	StateUnlinked State = "unlinked"
	StateExpired  State = "expired"
)

// Transition is one declared move of a client_portal_users row. Every
// writer of the row names the Transition it makes and passes From() as
// its own WHERE guard, so "which rows may this move leave" is answered
// here once rather than restated as `identity_uid IS NULL` in each
// statement. lifecycle_test.go's source scan fails a file that writes
// the row without naming one.
type Transition struct {
	// Act names the move for the reader of this declaration.
	Act string
	// From is the stored states the move may leave. Empty means the move
	// creates a new row.
	From []State
	// To is the stored state the move leaves the row in.
	To State
}

// open is a live invitation: one that carries a token and can still be
// accepted, refused again, rotated or revoked.
var open = []State{StateInvited, StateRefused}

// TransitionInvite is invite()'s first invitation, and every later one
// once the last has been revoked or unlinked: a new row, so the old one
// keeps its record of what happened to it.
var TransitionInvite = Transition{Act: "invite her", To: StateInvited}

// TransitionReinvite rotates a live invitation's token and expiry. A
// refused invitation is a question asked again (#2522, 00140).
var TransitionReinvite = Transition{Act: "send a new invite", From: open, To: StateInvited}

// TransitionAccept is acceptInvite's claim of the row. A refused
// invitation may still be accepted once the refusal's cause is gone,
// and the accept clears the refusal rather than standing beside it.
var TransitionAccept = Transition{Act: "accept it", From: open, To: StateAccepted}

// TransitionRefuse is ADR-0015's refusal of an accept (#2279), written
// by mark_portal_invite_refused (00168) with the same From.
var TransitionRefuse = Transition{Act: "refuse it", From: open, To: StateRefused}

// TransitionRevoke stops a live invitation, recording who, when and why
// (Revoke). Terminal.
var TransitionRevoke = Transition{Act: "withdraw it", From: open, To: StateRevoked}

// TransitionUnlink takes an accepted login off an erased Client. The
// database writes it (00168's client_portal_users_unlink trigger),
// because one of its two causes is the Portal Account FK's own ON DELETE
// SET NULL, which no Go code runs; erasure's own `SET identity_uid =
// NULL` goes through the same trigger, so there is one writer.
var TransitionUnlink = Transition{Act: "unlink it", From: []State{StateAccepted}, To: StateUnlinked}

// Transitions is every declared move, walked by lifecycle_test.go.
var Transitions = []Transition{TransitionInvite, TransitionReinvite, TransitionAccept, TransitionRefuse, TransitionRevoke, TransitionUnlink}

// FromStates is From as the text[] a statement binds against
// `state = ANY($n::portal_access_state[])`.
func (t Transition) FromStates() []string {
	out := make([]string, len(t.From))
	for i, s := range t.From {
		out[i] = string(s)
	}
	return out
}

// Allows reports whether a row in stored state s may make this move.
func (t Transition) Allows(s State) bool {
	return slices.Contains(t.From, s)
}

// Access is what a Staff-facing read says about one Client's portal
// access: the State, and -- only while she holds a live invitation --
// what the latest send of it did. The two are separate vocabularies of
// two tables (client_portal_users and portal_invite_outbox), so they
// are separate fields rather than one string that falls through from
// one to the other.
type Access struct {
	State State
	// Delivery is the latest portal_invite_outbox row's status ("pending",
	// "sent", "bounced", "dead_lettered", "complained"), set only when
	// State is StateInvited; "" otherwise, or when nothing was queued.
	Delivery string
}

// Derive is the one derivation of Access, fed by HerAccessColumns. nil
// means she has never been invited. now comes from the clock seam, so a
// simulated clock moves expiry with everything else.
func Derive(stored sql.NullString, expiresAt sql.NullTime, delivery sql.NullString, now time.Time) *Access {
	if !stored.Valid {
		return nil
	}
	state := State(stored.String)
	if state == StateInvited && TokenExpired(expiresAt, now) {
		state = StateExpired
	}
	a := &Access{State: state}
	if state == StateInvited && delivery.Valid {
		a.Delivery = delivery.String
	}
	return a
}

// Fields is a as the two wire fields every Staff-facing DTO carries
// (portalInviteStatus, portalInviteDelivery): the state, and the
// delivery only while it means something. Both nil when a is.
func (a *Access) Fields() (state *State, delivery *string) {
	if a == nil {
		return nil, nil
	}
	s := a.State
	if a.Delivery == "" {
		return &s, nil
	}
	d := a.Delivery
	return &s, &d
}

// TokenExpired is the one test of a live invitation's expiry: its
// invite_token_expires_at is at or before now. Derive reads an invited
// row through it, and acceptInvite refuses any live token through it --
// a refused invitation included, which Derive still names "refused"
// because that is what a Practice has to act on first.
func TokenExpired(expiresAt sql.NullTime, now time.Time) bool {
	return !expiresAt.Valid || !expiresAt.Time.After(now)
}

// CanReachPortal answers the question a Contract send asks (#255): can
// she open what is sent to her now? She can if she signs in already, or
// holds an invitation she can still accept that was not refused
// delivery. A complaint is not a failed delivery -- the mail arrived.
func (a *Access) CanReachPortal() bool {
	if a == nil {
		return false
	}
	switch a.State {
	case StateAccepted:
		return true
	case StateInvited:
		return a.Delivery != "bounced" && a.Delivery != "dead_lettered"
	case StateRefused, StateRevoked, StateUnlinked, StateExpired:
		// No live invitation she can accept, and no login: she cannot.
	}
	return false
}

// herRowOrder is which client_portal_users row is hers when a Client
// holds more than one (#813: a merge can leave her reached by two
// logins, or holding a revoked invitation beside a newer one). An
// accepted row first -- "she can sign in" is the more informative answer
// and the one that keeps a second invitation refused rather than sent --
// then a live invitation, then the most recent of whatever history is
// left. Deterministic, so the same request never answers two ways.
// Read by HerAccessJoin and by invite(), which decides from the same row.
const herRowOrder = `ORDER BY CASE state WHEN 'accepted' THEN 0 WHEN 'invited' THEN 1 WHEN 'refused' THEN 1 ELSE 2 END,
	          created_at DESC, id`

// HerAccessJoin is the one SQL projection of a Client's portal access,
// joined against a query whose Client is aliased c: her row (pu) and its
// latest send (latest). Select HerAccessColumns and pass them, in order,
// to Derive.
const HerAccessJoin = `
		 LEFT JOIN LATERAL (
		     SELECT id, state, invite_token_expires_at FROM client_portal_users
		      WHERE client_id = c.id
		      ` + herRowOrder + `
		      LIMIT 1
		 ) pu ON true
		 LEFT JOIN LATERAL (
		     SELECT o.status FROM portal_invite_outbox o
		     WHERE o.client_portal_user_id = pu.id
		     ORDER BY o.created_at DESC LIMIT 1
		 ) latest ON true`

// HerAccessColumns is HerAccessJoin's three outputs, in Derive's order.
const HerAccessColumns = `pu.state, pu.invite_token_expires_at, latest.status`

// RevokedClientErased is an erasure stopping a live invitation: the
// Client it named no longer exists to be reached.
const RevokedClientErased RevokeReason = "revoked: client erased"

// storedRevokeReason is RevokeReason as portal_access_revoke_reason
// spells it on the row.
func storedRevokeReason(reason RevokeReason) string {
	switch reason {
	case RevokedEmailChanged:
		return "email_changed"
	case RevokedRecordMerged:
		return "record_merged"
	case RevokedClientErased:
		return "client_erased"
	}
	// coverage:ignore reason: every RevokeReason is one of the three constants above
	return string(reason)
}

// Revoke makes TransitionRevoke on clientID's live invitation, if she
// holds one: the row records who (staffID, "" when DoulaCloud acted with
// nobody asking), when and why, its token stops being redeemable, and
// any send still queued for it is stopped (RevokePending). Returns how
// many rows it revoked -- zero when there was no live invitation, which
// is not an error.
func Revoke(ctx context.Context, tx *sql.Tx, clientID, staffID string, reason RevokeReason, at time.Time) (int, error) {
	res, err := tx.ExecContext(ctx,
		`UPDATE client_portal_users
		    SET state = $2, invite_token = NULL, invite_token_expires_at = NULL, access_refused_at = NULL,
		        revoked_at = $3, revoked_by_staff_id = NULLIF($4, '')::uuid, revoke_reason = $5
		  WHERE client_id = $1 AND state = ANY($6::portal_access_state[])`,
		clientID, string(TransitionRevoke.To), at, staffID, storedRevokeReason(reason), TransitionRevoke.FromStates(),
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return 0, fmt.Errorf("portalinvite: revoke invitation: %w", err)
	}
	revoked, err := res.RowsAffected()
	if err != nil {
		// coverage:ignore reason: pgx always reports a row count for an UPDATE
		return 0, fmt.Errorf("portalinvite: count revoked invitations: %w", err)
	}
	if err := RevokePending(ctx, tx, clientID, reason); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return 0, err
	}
	return int(revoked), nil
}
