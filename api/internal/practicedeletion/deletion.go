// Package practicedeletion is #871: a Practice deleting itself. It
// follows ADR-0027's own template -- redact in place, never a hard
// DELETE -- adapted for what a Practice carries that a Client does not:
// other people's access (Staff, Clients) and DoulaCloud's own business
// records.
//
// The act has a middle a Client's erasure does not: a 30-day restore
// window. Initiation locks the Practice (staffauth.Middleware and
// clientauth.Middleware both refuse every request against it except the
// two GET/DELETE routes this package mounts, Owner-only, and the plain
// GET .../session identity read staffauth.Middleware leaves open to
// every role) and enqueues a day-23 reminder and a day-30 finalization.
// Restoring, at any point before finalization, undoes the lock; the two
// enqueued outbox rows are never canceled (this package's own table
// carries no DELETE grant, the same design 00064_client_erasure.sql
// chose for client_erasure_outbox) -- each rechecks the Practice's live
// state at send time instead, the "skip-at-send recheck" shape
// offer/outbox.go already uses for a withdrawn Offer. A re-initiate
// after a restore upserts onto the same pending row rather than adding a
// second one (enqueue's own ON CONFLICT), so the recheck above never has
// a stale row from an earlier cycle to act on. Finalization cascades
// client.Erase across every Client still on file, forfeits any unspent
// Credit balance, and stamps practices.deleted_at -- the one fact that
// is never undone.
package practicedeletion

import (
	"context"
	"database/sql"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/invoicestatus"
	"doula-cloud/api/internal/staffauth"
)

// ReminderLeadTime is how long before ADR-0031's 30-day deadline the
// reminder outbox row fires -- the ticket's own settled default.
const ReminderLeadTime = 23 * 24 * time.Hour

// RestoreWindow is how long an Owner has to restore a Practice before
// its deletion finalizes -- ticket #871's decision 3.
const RestoreWindow = 30 * 24 * time.Hour

// Local Activity action vocabulary, the same shape export.go and
// staffauth/mfarequired.go each keep for their own SubjectPractice
// writes rather than centralizing in activity/actions.go (only Engagement
// and Client actions are centralized there, for the read-side filters
// that key off them; nothing reads a Practice-deletion action by name
// yet).
const (
	actionRequested = string(activity.ActionPracticeDeletionRequested)
	actionRestored  = string(activity.ActionPracticeDeletionRestored)
	actionFinalized = string(activity.ActionPracticeDeletionFinalized)
)

// requestedScope is the diff practice_deletion_requested's row carries:
// when the window closes, named plainly so a reader of the Practice's
// own Activity never has to compute it from created_at plus a constant.
type requestedScope struct {
	FinalizeAt time.Time `json:"finalizeAt"`
}

// finalizedScope is the diff practice_deletion_finalized's row carries --
// what the cascade actually covered, the same reasoning erasureScope
// gives in client/erase.go: it names counts, never a value that was
// destroyed, so the row stays readable regardless of what it describes.
type finalizedScope struct {
	ClientsErased    int `json:"clientsErased"`
	CreditsForfeited int `json:"creditsForfeited"`
}

// StatusResponse is what an Owner reads on the pending-deletion screen
// and, for a Practice with nothing pending, what the settings hub's
// precheck reads before ever offering the delete control -- the same
// role client.EraseEligibility plays ahead of #691's confirmation.
//
// Carries no Deleted/DeletedAt: once deleted_at is set,
// staffauth.Middleware refuses every request against the Practice,
// Owner included, before StatusHandler -- or any other handler -- ever
// runs again, so no caller of this endpoint can ever observe that state
// live.
type StatusResponse struct {
	Pending              bool       `json:"pending"`
	DeletionRequestedAt  *time.Time `json:"deletionRequestedAt,omitempty"`
	FinalizeAt           *time.Time `json:"finalizeAt,omitempty"`
	HasUnsettledInvoices bool       `json:"hasUnsettledInvoices"`
}

// InitiateResponse is what starting the 30-day window returns.
type InitiateResponse struct {
	DeletionRequestedAt time.Time `json:"deletionRequestedAt"`
	FinalizeAt          time.Time `json:"finalizeAt"`
}

// deletionState is the one row lookup every handler here starts from:
// the Practice's standing, and the two dates its window carries.
type deletionState struct {
	standing    Standing
	requestedAt sql.NullTime
	finalizeAt  sql.NullTime
}

func readDeletionState(ctx context.Context, tx *sql.Tx, practiceID string, forUpdate bool) (deletionState, error) {
	query := `SELECT standing::text, deletion_requested_at, deletion_finalize_at FROM practices WHERE id = $1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var s deletionState
	if err := tx.QueryRowContext(ctx, query, practiceID).Scan(&s.standing, &s.requestedAt, &s.finalizeAt); err != nil {
		// coverage:ignore reason: the practices row is guaranteed by staffauth.Middleware's own membership query, not exercised by unit tests
		return deletionState{}, fmt.Errorf("practicedeletion: read deletion state: %w", err)
	}
	return s, nil
}

// hasUnsettledInvoices reports whether any Client under practiceID
// carries a draft or open Invoice -- the one thing that refuses both an
// initiation and, transitively, the cascade a finalization would
// otherwise run into (client.Erase itself refuses the same condition per
// Client; this is the whole-Practice precheck so an Owner is refused
// with the reason named up front, the same shape
// client.unsettledInvoiceSummaries gives one Client's own eligibility
// read).
func hasUnsettledInvoices(ctx context.Context, tx *sql.Tx, practiceID string) (bool, error) {
	var blocked bool
	err := tx.QueryRowContext(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM invoices i
			JOIN contracts ct ON ct.id = i.contract_id
			JOIN engagements e ON e.id = ct.engagement_id
			WHERE e.practice_id = $1 AND `+invoicestatus.UnsettledExpr+`
		 )`,
		practiceID,
	).Scan(&blocked)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, fmt.Errorf("practicedeletion: check unsettled invoices: %w", err)
	}
	return blocked, nil
}

// StatusHandler answers what the pending-deletion screen and the
// settings hub's precheck both need: whether deletion is pending or
// final, when the window closes, and whether an initiation would be
// refused right now. Owner-only, the same seat as every other act here.
// Must be mounted behind staffauth.Middleware.
func StatusHandler() http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		tx, practiceID := c.Tx, c.PracticeID

		state, err := readDeletionState(r.Context(), tx, practiceID, false)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		blocked, err := hasUnsettledInvoices(r.Context(), tx, practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		out := StatusResponse{HasUnsettledInvoices: blocked, Pending: state.standing == StandingPendingDeletion}
		if state.requestedAt.Valid {
			t := state.requestedAt.Time
			out.DeletionRequestedAt = &t
		}
		if state.finalizeAt.Valid {
			t := state.finalizeAt.Time
			out.FinalizeAt = &t
		}
		apierr.WriteJSON(w, http.StatusOK, out)
	})
}

// InitiateHandler starts the 30-day window: locks the Practice against
// every Staff and Client request but the Owner's own reach into this
// package's routes, and enqueues the reminder and finalization. Refused,
// with 409 and the reason named, if deletion is already pending or
// already final, or if any Client under the Practice carries an
// unsettled Invoice -- the same three-way refusal shape EraseHandler
// gives one Client, widened to a whole Practice.
//
// staffauth.RequireConfirmed backstops #691's ConfirmDialog the same way
// EndSessionsHandler's own X-Confirmed check does: the frontend never
// sends this request until its confirm button is pressed, and the
// header is what proves it tried.
//
// Must be mounted behind staffauth.Middleware.
//
// Owner-only is declared at the mount, not checked here (#1028,
// following #970, #990 and #1016): this handler no longer calls
// staffauth.RequireOwner, because an Admin or a Doula is refused by the
// gate before it runs. Widening or narrowing this route means editing
// its role list in mount.go.
func InitiateHandler() http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		tx, practiceID, staffID := c.Tx, c.PracticeID, c.StaffID

		// coverage:ignore reason: unreachable through the real mount -- staffauth.Middleware refuses a deleted Practice to everyone and a pending one to every route but GET/DELETE, so a second initiate is refused only by racing a first one onto this row's FOR UPDATE lock; TestTransition_CheckSpeaksToThePerson proves the refusal it writes
		if !standingAllows(w, r, tx, practiceID, TransitionRequest, apierr.CodeConflict) {
			return
		}

		blocked, err := hasUnsettledInvoices(r.Context(), tx, practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if blocked {
			apierr.Write(w, http.StatusConflict, apierr.CodeFailedPrecondition,
				"cannot delete a practice with unsettled invoices: settle or void them first", nil)
			return
		}

		now := clock.Now(r.Context()).UTC()
		finalizeAt := now.Add(RestoreWindow)
		reminderAt := now.Add(ReminderLeadTime)

		ok, err := moved(tx.ExecContext(r.Context(),
			`UPDATE practices SET deletion_requested_at = $1, deletion_requested_by = $2, deletion_finalize_at = $3
			 WHERE id = $4 AND standing = $5`,
			now, staffID, finalizeAt, practiceID, TransitionRequest.Requires,
		))
		// coverage:ignore reason: a DB failure, or a row that left TransitionRequest.Requires under this transaction's own FOR UPDATE lock, which nothing else can write through
		if err != nil || !ok {
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := enqueue(r.Context(), tx, practiceID, actReminder, reminderAt); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := enqueue(r.Context(), tx, practiceID, actFinalize, finalizeAt); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		diff, err := json.Marshal(requestedScope{FinalizeAt: finalizeAt})
		if err != nil {
			// coverage:ignore reason: a struct of one time always marshals cleanly, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := recordPracticeAct(r.Context(), tx, practiceID, actionRequested, diff, activity.StaffActor(staffID)); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, InitiateResponse{DeletionRequestedAt: now, FinalizeAt: finalizeAt})
	}, staffauth.Confirmed())
}

// RestoreHandler undoes a pending deletion at any point before
// finalization -- any current Owner, not only the one who initiated it
// (ticket #871's decision 3: every other Owner-only gate in the product
// is seat-based, not person-specific). Refused with 409 if there is
// nothing pending to restore. The two outbox rows enqueued at initiation
// are left as they are; PracticeDeletionWorker's own recheck of live
// state at send time is what makes this restore effective (see the
// package doc comment).
//
// Must be mounted behind staffauth.Middleware.
//
// Owner-only is declared at the mount, not checked here (#1028,
// following #970, #990 and #1016): this handler no longer calls
// staffauth.RequireOwner, because an Admin or a Doula is refused by the
// gate before it runs. Widening or narrowing this route means editing
// its role list in mount.go.
func RestoreHandler() http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		tx, practiceID, staffID := c.Tx, c.PracticeID, c.StaffID

		if !standingAllows(w, r, tx, practiceID, TransitionRestore, apierr.CodeFailedPrecondition) {
			return
		}

		ok, err := moved(tx.ExecContext(r.Context(),
			`UPDATE practices SET deletion_requested_at = NULL, deletion_requested_by = NULL, deletion_finalize_at = NULL
			 WHERE id = $1 AND standing = $2`,
			practiceID, TransitionRestore.Requires,
		))
		// coverage:ignore reason: a DB failure, or a row that left TransitionRestore.Requires under this transaction's own FOR UPDATE lock, which nothing else can write through
		if err != nil || !ok {
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := recordPracticeAct(r.Context(), tx, practiceID, actionRestored, nil, activity.StaffActor(staffID)); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

// standingAllows reads practiceID's deletion state under its row lock
// and asks t whether the Practice may make that move. When it may not,
// or the read fails, it writes the response and answers false.
func standingAllows(w http.ResponseWriter, r *http.Request, tx *sql.Tx, practiceID string, t Transition, refused apierr.Code) bool {
	state, err := readDeletionState(r.Context(), tx, practiceID, true)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return false
	}
	if ok, refusal := t.Check(state.standing); !ok {
		apierr.Write(w, http.StatusConflict, refused, refusal, nil)
		return false
	}
	return true
}

// recordPracticeAct records action, by actor, on the Practice itself.
func recordPracticeAct(ctx context.Context, tx *sql.Tx, practiceID, action string, diff jsontext.Value, actor activity.Actor) error {
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectPractice,
		SubjectID:   practiceID,
		Action:      action,
		Diff:        diff,
		Actor:       actor,
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("practicedeletion: record %s: %w", action, err)
	}
	return nil
}
