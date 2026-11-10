// Package attachmentstate names an Attachment's states -- the Doulas on
// an Engagement, engagement_attachments -- and is the one writer of its
// lifecycle: every accrual, grant and ending goes through a Transition
// declared here, with the instant from the clock seam.
//
// The state is derived once, by the engagement_attachments.state
// generated column (00173_engagement_attachment_state.sql). Nothing here
// derives it: Go names the three values the column holds, and a reader
// names the states it admits in its own SQL -- `ea.state = 'granted'` for
// "a Doula on the Engagement now", `ea.state <> 'ended'` for "open, either
// origin" -- rather than restating origin and ended_at, so a difference
// between two readers is a visible choice. "The Doula who held this has
// left" is the SQL function engagement_holder_left (00173), which both
// the Staff reads and the Client portal's door call. guardrail_test.go
// fails the build on a reader that goes back to ended_at, or a writer
// that is not this package.
//
// It imports nothing of this repo's own but the clock, for the reason
// invoicestatus and clientstanding give: staffauth, client, visit, offer
// and oncall all read the names, and staffauth's seam writes through it,
// so it must import none of them.
package attachmentstate

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"doula-cloud/api/internal/clock"
)

// State is the engagement_attachments.state enum (00173).
type State string

// The three states, matching attachment_state's labels.
const (
	// Accrued is open and opened by her own work (staffauth.AttachingWrite):
	// a record of work, never a key.
	Accrued State = "accrued"
	// Granted is open and someone decided she is on it. It is the only
	// state that reaches, and "who is working this birth" is every row in
	// it.
	Granted State = "granted"
	// Ended is closed, never deleted. The row's origin still says which of
	// the other two it ended from.
	Ended State = "ended"
)

// States is every value attachment_state holds, in its declared order.
// schema_test.go holds it to the database's own enum_range.
var States = []State{Accrued, Granted, Ended}

// Ending is why an Attachment ended, the attachment_ending enum (00173):
// the four triggers ADR-0008 names.
type Ending string

// The four endings.
const (
	// DroppedOut is her own decision to stop. No route writes it yet: a
	// Doula has no way to take herself off an Engagement that is not the
	// Owner's or the Admin's own Take off.
	DroppedOut Ending = "dropped_out"
	// TakenOff is the Practice taking her off (#2596).
	TakenOff Ending = "taken_off"
	// EngagementCompleted is the Engagement completing (#317).
	EngagementCompleted Ending = "engagement_completed"
	// MembershipEnded is her Membership at the Practice ending: an Owner
	// removing her, or her deleting her own login.
	MembershipEnded Ending = "membership_ended"
)

// Endings is every value attachment_ending holds, in its declared order.
var Endings = []Ending{DroppedOut, TakenOff, EngagementCompleted, MembershipEnded}

// Transition is one declared move of an engagement_attachments row.
type Transition struct {
	// Act names the move for the reader of this declaration.
	Act string
	// From is the states the move may leave. Empty means the move opens a
	// new row.
	From []State
	// To is the state the move leaves the row in.
	To State
	// Ending is why, for a move to Ended.
	Ending Ending
}

// The declared moves. accrued -> granted is one-way and ended is
// terminal; 00173's engagement_attachments_one_way trigger refuses
// anything else, from any writer.
//
// Reopening a completed Engagement (engagement/transition.go) makes no
// move here: the Attachments completion ended stay ended, and the
// Engagement comes back with no Doula on it until someone is put on it
// again. That is today's behavior, kept as it is; whether a reopen
// should put back the Doulas completion took off is a question for the
// founder, not a decision this declaration makes.
var (
	// TransitionAccrue opens an accrued Attachment for the acting Doula,
	// or leaves the open one she already holds alone (Accrue).
	TransitionAccrue = Transition{Act: "accrue", To: Accrued}
	// TransitionGrant opens a granted Attachment, or upgrades her open
	// accrued one in place (Grant).
	TransitionGrant = Transition{Act: "grant", From: []State{Accrued}, To: Granted}
	// TransitionTakeOff ends her granted Attachment (TakeOff). An
	// accrued one is not on the Doula row an Owner or an Admin acts on,
	// so it is not taken off.
	TransitionTakeOff = Transition{Act: "take her off", From: []State{Granted}, To: Ended, Ending: TakenOff}
	// TransitionComplete ends every open Attachment on an Engagement that
	// completed (EndForCompletion).
	TransitionComplete = Transition{Act: "complete the Engagement", From: []State{Accrued, Granted}, To: Ended, Ending: EngagementCompleted}
	// TransitionMembershipEnds ends every open Attachment she holds at a
	// Practice whose Membership ended (EndForMembership).
	TransitionMembershipEnds = Transition{Act: "end her Membership", From: []State{Accrued, Granted}, To: Ended, Ending: MembershipEnded}
)

// Transitions is every declared move, walked by attachmentstate_test.go.
var Transitions = []Transition{TransitionAccrue, TransitionGrant, TransitionTakeOff, TransitionComplete, TransitionMembershipEnds}

// Accrue opens an accrued Attachment for staffID on engagementID: she
// did something on it herself, so attached_by is her own id. ON CONFLICT
// DO NOTHING leaves an Attachment already open for the pair -- accrued
// from an earlier write, or granted -- exactly as it is, so origin only
// ever travels accrued -> granted.
//
// attached_at reads the clock seam, like every ending below.
func Accrue(ctx context.Context, tx *sql.Tx, engagementID, staffID string) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by, attached_at)
		 VALUES ($1, $2, 'accrued', $2, $3)
		 ON CONFLICT (engagement_id, staff_id) WHERE state <> 'ended' DO NOTHING`,
		engagementID, staffID, clock.Now(ctx),
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("attachmentstate: accrue: %w", err)
	}
	return nil
}

// Grant opens a granted Attachment for staffID on engagementID, or
// upgrades her open accrued one in place. Granted is the only state that
// reaches, so it is written only by a decision: an Offer's acceptance,
// and attachment.Grant's writers (a Request's approval, "Put me on this
// Engagement", an Owner or an Admin on the Doula row, a Visit that names
// her). Never by the AttachingWrite seam.
//
// feeAmountCents and feeTerms are the Offer's own, copied at acceptance
// so nothing can later rewrite what she agreed to; both are nil
// everywhere else. On an upgrade they are COALESCEd rather than
// assigned: an acceptance over an accrued row still copies its fee on,
// while a later grant carrying no fee cannot blank one an Offer copied.
// An already-granted row is left alone, attached_by included.
//
// at is the instant a new row is attached, passed in rather than read
// here: offer/list.go finds the Attachment an Offer's acceptance opened by
// attached_at <= decided_at, so the acceptance hands both writes the one
// instant it read from the clock. An upgraded row keeps the attached_at
// it accrued with.
func Grant(ctx context.Context, tx *sql.Tx, engagementID, staffID, attachedBy string, feeAmountCents *int64, feeTerms *string, at time.Time) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO engagement_attachments
		     (engagement_id, staff_id, origin, attached_by, fee_amount_cents, fee_terms, attached_at)
		 VALUES ($1, $2, 'granted', $3, $4, $5, $6)
		 ON CONFLICT (engagement_id, staff_id) WHERE state <> 'ended'
		 DO UPDATE SET origin = 'granted', attached_by = $3,
		               fee_amount_cents = COALESCE($4, engagement_attachments.fee_amount_cents),
		               fee_terms = COALESCE($5, engagement_attachments.fee_terms)
		 WHERE engagement_attachments.state = 'accrued'`,
		engagementID, staffID, attachedBy, feeAmountCents, feeTerms, at,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("attachmentstate: grant: %w", err)
	}
	return nil
}

// TakeOff ends staffID's granted Attachment on engagementID, recording
// who ended it, when and why. It reports whether there was one to end,
// so a second press writes and reports nothing.
func TakeOff(ctx context.Context, tx *sql.Tx, engagementID, staffID, endedBy string) (bool, error) {
	res, err := tx.ExecContext(ctx,
		`UPDATE engagement_attachments
		    SET ended_at = $4, ended_by = $3, ended_reason = $5
		  WHERE engagement_id = $1 AND staff_id = $2 AND state = 'granted'`,
		engagementID, staffID, endedBy, clock.Now(ctx), string(TransitionTakeOff.Ending),
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, fmt.Errorf("attachmentstate: take off: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		// coverage:ignore reason: the pgx driver always reports rows affected
		return false, fmt.Errorf("attachmentstate: take off: %w", err)
	}
	return n > 0, nil
}

// EndForCompletion ends every open Attachment on engagementID, of either
// origin, as the Engagement completes; endedBy is who completed it.
// Running it again on a completed Engagement ends nothing more.
func EndForCompletion(ctx context.Context, tx *sql.Tx, engagementID, endedBy string) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE engagement_attachments
		    SET ended_at = $3, ended_by = $2, ended_reason = $4
		  WHERE engagement_id = $1 AND state <> 'ended'`,
		engagementID, endedBy, clock.Now(ctx), string(TransitionComplete.Ending),
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("attachmentstate: end for completion: %w", err)
	}
	return nil
}

// EndForMembership ends every open Attachment staffID holds on an
// Engagement at practiceID, of either origin, as her Membership there
// ends; endedBy is who ended it (the Owner who removed her, or herself
// deleting her login). The caller runs it inside the Practice's own
// context, as the Membership's own DELETE needs.
func EndForMembership(ctx context.Context, tx *sql.Tx, practiceID, staffID, endedBy string) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE engagement_attachments ea
		    SET ended_at = $4, ended_by = $3, ended_reason = $5
		   FROM engagements e
		  WHERE e.id = ea.engagement_id AND e.practice_id = $1
		    AND ea.staff_id = $2 AND ea.state <> 'ended'`,
		practiceID, staffID, endedBy, clock.Now(ctx), string(TransitionMembershipEnds.Ending),
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("attachmentstate: end for membership: %w", err)
	}
	return nil
}
