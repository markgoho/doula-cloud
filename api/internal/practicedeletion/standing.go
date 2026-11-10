package practicedeletion

import (
	"database/sql"
	"fmt"
)

// Standing is a Practice's deletion standing: the practice_standing enum
// that practices.standing is generated from the four deletion facts
// (00172_practice_standing.sql). The column is the one derivation; every
// reader, Go or SQL, reads it rather than asking the facts again.
type Standing string

// The three stored standings, matching practice_standing.
const (
	// StandingActive is a Practice nobody has asked to delete, or one an
	// Owner restored.
	StandingActive Standing = "active"
	// StandingPendingDeletion is a Practice inside ADR-0031's 30-day
	// window: locked to everything but its Owner's restore, and due to
	// finalize at deletion_finalize_at.
	StandingPendingDeletion Standing = "pending_deletion"
	// StandingDeleted is a finalized Practice. Nothing leaves it.
	StandingDeleted Standing = "deleted"
)

// StandingFinalizeOverdue is never stored: it is StandingPendingDeletion
// read at an instant past deletion_finalize_at, the state a finalization
// that keeps failing (or a dead-lettered finalize row) leaves a Practice
// in. It depends on the instant, which a generated column cannot read, so
// its one derivation is the SQL function practice_standing_at, which ops'
// Practice list reads. Still restorable: to every gate it is pending.
const StandingFinalizeOverdue Standing = "finalize_overdue"

// Transition is one named move between standings, the shape
// contracts.Transition gives a Contract: the standing the Practice must
// hold beforehand, and the one the move leaves it in. Every write to the
// deletion facts goes through one of these, twice: Check on the standing
// read under the row's FOR UPDATE lock, so a refusal names the standing
// the Practice is in, and Requires as the guard on the UPDATE itself, so
// the write cannot land on a row that is not in it. standing_test.go's
// source scan fails a writer that drops either.
type Transition struct {
	// Act names the move as the reader would do it, for the refusal
	// sentence Check writes.
	Act string
	// Requires is the one standing the move may start from.
	Requires Standing
	// Becomes is the standing the move leaves the Practice in.
	Becomes Standing
}

// TransitionRequest is InitiateHandler's move: an active Practice opens
// its 30-day window.
var TransitionRequest = Transition{Act: "delete it", Requires: StandingActive, Becomes: StandingPendingDeletion}

// TransitionRestore is RestoreHandler's move: a pending Practice goes
// back to active, at any point before finalization -- an overdue one
// included.
var TransitionRestore = Transition{Act: "restore it", Requires: StandingPendingDeletion, Becomes: StandingActive}

// TransitionFinalize is the finalize worker's move: a pending Practice
// is deleted, for good.
var TransitionFinalize = Transition{Act: "finalize its deletion", Requires: StandingPendingDeletion, Becomes: StandingDeleted}

// Transitions is every declared move, walked by standing_test.go.
var Transitions = []Transition{TransitionRequest, TransitionRestore, TransitionFinalize}

// Check reports whether current satisfies t, or, if not, the 409 message
// a caller writes: one sentence naming the standing the Practice is in.
func (t Transition) Check(current Standing) (ok bool, refusal string) {
	if current == t.Requires {
		return true, ""
	}
	return false, fmt.Sprintf("%s, so you cannot %s.", standingClause(current), t.Act)
}

// standingClause says what standing a Practice is in, as the opening
// clause of a refusal a person reads.
func standingClause(current Standing) string {
	switch current {
	case StandingActive:
		return "This Practice is not set to be deleted"
	case StandingPendingDeletion, StandingFinalizeOverdue:
		return "This Practice is set to be deleted"
	case StandingDeleted:
		return "This Practice has been deleted"
	}
	return fmt.Sprintf("This Practice is %s", current)
}

// moved reports whether a write guarded on a Transition's Requires
// landed on its row: false is the row no longer holding that standing.
func moved(res sql.Result, err error) (bool, error) {
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, fmt.Errorf("practicedeletion: write standing: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		// coverage:ignore reason: driver failure, not exercised by unit tests
		return false, fmt.Errorf("practicedeletion: rows affected: %w", err)
	}
	return n == 1, nil
}
