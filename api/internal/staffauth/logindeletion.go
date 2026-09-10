package staffauth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
)

// DeletedStaffName is what a deleted login's staff.name becomes. The
// column is NOT NULL and always has been, so the redaction replaces it
// rather than nulling it, and the replacement says plainly what happened
// rather than inventing a person -- the same choice client.ErasedGivenName
// makes for a Client.
const DeletedStaffName = "Deleted Staff Member"

// DeletedStaffEmail is what a deleted login's staff.email becomes. Also
// NOT NULL, and unlike identity_uid not unique, so one shared constant
// serves every deleted row. The domain is RFC 2606's reserved .invalid,
// which no mail can ever be delivered to -- if some future send path
// reaches a redacted row despite the recheck every outbox worker now
// does, it fails loudly at the address rather than mailing a stranger.
const DeletedStaffEmail = "deleted@deleted.invalid"

// deletedIdentityUID is the sentinel staff.identity_uid takes on
// deletion. NOT NULL and UNIQUE, so it can be neither nulled nor set to
// a constant shared with the next person to delete her login; deriving
// it from the row's own id makes it unique for free. It cannot collide
// with a real Identity Platform uid, which is 28 alphanumeric characters
// with no colon in it.
//
// 00101's staff_self_login_deletion policy spells this same expression
// in SQL. The duplication is deliberate: the policy is the boundary that
// can actually enforce the shape, and this is what makes a wrong shape a
// caught mistake here rather than a silent zero-rows UPDATE there.
func deletedIdentityUID(staffID string) string {
	return "deleted:" + staffID
}

// soleOwnerAt is one Practice a deletion is refused over: she holds
// 'owner' there, nobody else does, and the Practice has not been
// finalized.
type soleOwnerAt struct {
	practiceID string
	name       string
}

// ownMembership is one of her Memberships, read before anything is
// destroyed -- the roles and employment type the 'removed' event has to
// name are exactly what the DELETE takes away.
type ownMembership struct {
	practiceID     string
	roles          string
	employmentType string
}

// DeleteLoginHandler is #892: a Staff person deleting her own login.
// Immediate and irreversible -- there is no restore window, ADR-0033's
// decision 3. Every Membership she holds ends, at every Practice; her
// sessions are deleted; her staff row is redacted in place; her Identity
// Platform account is destroyed. Every record she authored keeps
// resolving, to a row that no longer names her.
//
// Self-only, and enforced by shape rather than by a check, the same way
// UpdateWorkStateHandler is: the route carries no staff id and no
// {practiceId}, and the row acted on is the one the session cookie's
// identity resolves to. There is no Owner-triggered variant -- an
// Owner's tool for ending someone's access is RemoveMembershipHandler,
// which is correctly scoped to the one Practice she owns, and no Owner
// has reach across the other Practices a contractor doula works at.
//
// Refused with 409 while she is sole Owner of any Practice that has not
// been finalized -- a Practice already pending deletion included, since
// letting her go during that window would foreclose the restore ADR-0031
// promises. She hands ownership over, or deletes the Practice first.
// Refused with 409, too, on a login that is already deleted.
//
// RequireConfirmed backstops the frontend's confirmation the same way
// EndSessionsHandler and practicedeletion.InitiateHandler do: a hard
// block with a deliberate override, per CLAUDE.md, not a dismissible
// warning.
//
// Mounted outside Middleware on purpose, beside DELETE /api/staff/mfa: a
// login belongs to no Practice, so app.current_practice_id starts unset
// -- the window 00044's and 00101's policies are both scoped to. The
// per-Practice writes below set and unset it around themselves.
func DeleteLoginHandler(accounts authn.AccountManager, db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, uid, _, ok := authn.Begin(w, r, db, authn.TierStaff)
		if !ok {
			return
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		if !RequireConfirmed(w, r) {
			return
		}

		// Two session variables, for two different reads.
		//
		// app.current_identity_uid is what 00044's and 00101's policies
		// key on, and authn.Begin does not set it (#151 moved session
		// ownership off Identity Platform entirely) -- UpdateWorkStateHandler
		// sets it for its own UPDATE the same way, and this is the same
		// pre-Practice self-edit window.
		//
		// app.notification_worker_trusted is the same reuse of 00033's
		// trust flag RotateSavedCodesHandler makes, and for the same
		// reason: this act reads across every Practice she belongs to,
		// which no per-Practice policy can admit by construction.
		//
		// It is load-bearing a second time, in a way that is not visible
		// from that sentence and would not survive a cleanup that trusted
		// it. Postgres checks this table's SELECT policies against the
		// *new* row of an UPDATE, and the redaction's new row carries the
		// sentinel identity_uid, which staff_self_visibility (00006) does
		// not match. staff_notification_worker (00033) is the only SELECT
		// policy left that admits it. Drop this flag as "only needed for
		// the reads" and every deletion 500s at the redaction.
		// rls_test.go's own TestRLS_LoginDeletionNeedsTheTrustedFlagToSeeItsOwnNewRow
		// fails first, which is the point of it.
		if _, err := tx.ExecContext(r.Context(),
			`SELECT set_config('app.current_identity_uid', $1, true),
			        set_config('app.notification_worker_trusted', 'true', true)`, uid); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		staffID, ok := lockOwnStaffRow(w, r, tx, uid)
		if !ok {
			return
		}

		memberships, err := listOwnMemberships(r.Context(), tx, staffID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		blocking, err := soleOwnerPractices(r.Context(), tx, staffID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if len(blocking) > 0 {
			names := make([]string, len(blocking))
			for i, p := range blocking {
				names[i] = p.name
			}
			apierr.Write(w, http.StatusConflict, apierr.CodeFailedPrecondition,
				"cannot delete your login while you are the only Owner of "+strings.Join(names, ", ")+
					": hand ownership to someone else, or delete the practice first", nil)
			return
		}

		if err := endEveryMembership(r.Context(), tx, staffID, memberships); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := redactStaffRow(r.Context(), tx, staffID, uid, time.Now().UTC()); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		// Last, and inside the transaction: a failure here rolls
		// everything above back, so a person is never left with a redacted
		// row and a live Identity Platform account. The reverse order --
		// account gone, row intact -- would lock her out of a login she
		// asked to delete and could not then finish deleting. An
		// already-absent account is a success (authn.AccountManager's own
		// contract), which is what lets a retry finish.
		if err := accounts.DeleteAccount(r.Context(), uid); err != nil {
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: commit failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed = true

		// Her session rows are gone, so the cookie in this browser already
		// names nothing. Clearing it anyway is what stops the frontend
		// sending a dead credential on the next request and reading the
		// 401 as a fault -- the same clearing session.EndHandler does on
		// ordinary sign-out.
		http.SetCookie(w, &http.Cookie{
			Name:     authn.SessionCookieName,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})
		w.WriteHeader(http.StatusNoContent)
	})
}

// lockOwnStaffRow resolves uid to her staff row and locks it, writing the
// response itself on every refusal -- the ok-bool idiom RequireOwner and
// client.lookupErasedAt both use.
//
// The lock is what makes the already-deleted check a gate rather than a
// guess: two concurrent deletions serialize on this row, and the second
// reads the first's deleted_at.
func lockOwnStaffRow(w http.ResponseWriter, r *http.Request, tx *sql.Tx, uid string) (staffID string, ok bool) {
	var deletedAt sql.NullTime
	err := tx.QueryRowContext(r.Context(),
		`SELECT id, deleted_at FROM staff WHERE identity_uid = $1 FOR UPDATE`, uid,
	).Scan(&staffID, &deletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		apierr.WriteError(w, MsgNoMatchingStaffAccount, http.StatusNotFound)
		return "", false
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return "", false
	}
	if deletedAt.Valid {
		// coverage:ignore reason: reachable only by two genuinely concurrent deletions racing this row's FOR UPDATE lock -- a sequential second call finds the sentinel identity_uid instead and 404s above, and holds no live session to reach here with anyway
		apierr.Write(w, http.StatusConflict, apierr.CodeConflict,
			"this login has already been deleted", nil)
		return "", false
	}
	return staffID, true
}

// listOwnMemberships reads every Membership she holds, across every
// Practice. Admitted by 00033's trusted-worker policy, which the handler
// set before calling: no per-Practice policy can answer a question whose
// scope is a person.
func listOwnMemberships(ctx context.Context, tx *sql.Tx, staffID string) ([]ownMembership, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT practice_id, array_to_string(roles, ','), employment_type::text
		   FROM practice_memberships
		  WHERE staff_id = $1
		  ORDER BY practice_id`,
		staffID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("staffauth: list own memberships: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []ownMembership
	for rows.Next() {
		var m ownMembership
		if err := rows.Scan(&m.practiceID, &m.roles, &m.employmentType); err != nil {
			// coverage:ignore reason: DB scan failure, not exercised by unit tests
			return nil, fmt.Errorf("staffauth: scan own membership: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("staffauth: list own memberships: %w", err)
	}
	return out, nil
}

// soleOwnerPractices names every Practice where she holds 'owner', no
// other Member does, and the Practice has not been finalized.
//
// It is isSoleOwnerAnywhere's question widened from a yes/no to a list,
// with one predicate the other does not have. The two deliberately do not
// agree about a finalized Practice: #615's saved-recovery-code
// eligibility counts her as its sole Owner (she is), and this refusal
// does not (nobody can reach it). Spelled out because it is the kind of
// divergence that reads as a copy someone forgot to keep in step --
// a person refused this act deserves to be told which Practices are in
// the way, not merely that some are. The deleted_at filter is what makes
// a finalized Practice stop blocking: nobody can reach it any more, so an
// Owner it has no longer means anything. A Practice merely *pending*
// deletion still blocks, deliberately: ADR-0031 promises a restore during
// that window, and a restore into a Practice with no Owner is a Practice
// nobody can invite to, edit a Membership at, or buy Credits for.
func soleOwnerPractices(ctx context.Context, tx *sql.Tx, staffID string) ([]soleOwnerAt, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT p.id, p.name
		   FROM practice_memberships pm
		   JOIN practices p ON p.id = pm.practice_id
		  WHERE pm.staff_id = $1
		    AND 'owner' = ANY(pm.roles)
		    AND p.deleted_at IS NULL
		    AND NOT EXISTS (
		        SELECT 1 FROM practice_memberships other
		         WHERE other.practice_id = pm.practice_id
		           AND other.staff_id <> pm.staff_id
		           AND 'owner' = ANY(other.roles)
		    )
		  ORDER BY p.name`,
		staffID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("staffauth: list sole-owner practices: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []soleOwnerAt
	for rows.Next() {
		var p soleOwnerAt
		if err := rows.Scan(&p.practiceID, &p.name); err != nil {
			// coverage:ignore reason: DB scan failure, not exercised by unit tests
			return nil, fmt.Errorf("staffauth: scan sole-owner practice: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("staffauth: list sole-owner practices: %w", err)
	}
	return out, nil
}

// endEveryMembership ends each Membership in memberships, one Practice at
// a time, recording the same 'removed' event RemoveMembershipHandler
// records -- with her as her own actor.
//
// The per-Practice context set here is the whole reason this is a loop
// rather than one statement. activity's INSERT policy and
// practice_memberships' own FOR ALL policy are both plain comparisons
// against app.current_practice_id, so a Practice-scoped write is only
// possible inside a Practice's own context, and this act spans several.
// The context is unset again at the end: 00101's redaction policy, and
// 00044's before it, are scoped to the pre-Practice window, and leaving a
// stale Practice id set would make the UPDATE that follows match zero
// rows silently.
func endEveryMembership(ctx context.Context, tx *sql.Tx, staffID string, memberships []ownMembership) error {
	for _, m := range memberships {
		if err := setPracticeContext(ctx, tx, m.practiceID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return err
		}
		// The event goes first: it names the roles and employment type the
		// next statement destroys.
		if err := RecordMembershipEvent(ctx, tx, MembershipEvent{
			PracticeID: m.practiceID, StaffID: staffID, Type: "removed",
			PreviousRoles: "{" + m.roles + "}", PreviousEmploymentType: m.employmentType,
			ActorStaffID: staffID,
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM practice_memberships WHERE practice_id = $1 AND staff_id = $2`,
			m.practiceID, staffID,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return fmt.Errorf("staffauth: delete own membership: %w", err)
		}
		// #615's rule, the same call RemoveMembershipHandler makes after
		// its own DELETE: losing her can leave a remaining co-Owner newly
		// sole, and so newly entitled to saved recovery codes.
		if err := reconcileOwnersAtPractice(ctx, tx, m.practiceID, staffID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return err
		}
	}
	return setPracticeContext(ctx, tx, "")
}

// setPracticeContext sets app.current_practice_id for the rest of the
// transaction, or clears it when practiceID is empty.
func setPracticeContext(ctx context.Context, tx *sql.Tx, practiceID string) error {
	if _, err := tx.ExecContext(ctx,
		`SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("staffauth: set practice context: %w", err)
	}
	return nil
}

// redactStaffRow performs the redaction itself: her sessions first, then
// the row.
//
// The order is load-bearing and is the one thing here that cannot be
// reordered for readability. sessions is keyed on identity_uid, not on
// staff_id, so deleting her session rows after the sentinel is written
// would match nothing at all and leave her signed in on a live cookie at
// every browser she ever used.
func redactStaffRow(ctx context.Context, tx *sql.Tx, staffID, uid string, now time.Time) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE identity_uid = $1`, uid); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("staffauth: delete own sessions: %w", err)
	}

	if err := resolveQueuedMail(ctx, tx, uid, now); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}

	res, err := tx.ExecContext(ctx,
		`UPDATE staff
		    SET name = $1, email = $2, identity_uid = $3, deleted_at = $4
		  WHERE id = $5 AND deleted_at IS NULL`,
		DeletedStaffName, DeletedStaffEmail, deletedIdentityUID(staffID), now, staffID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("staffauth: redact staff row: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		// coverage:ignore reason: driver failure, not exercised by unit tests
		return fmt.Errorf("staffauth: redact staff row: %w", err)
	}
	if affected != 1 {
		// coverage:ignore reason: unreachable -- lockOwnStaffRow holds this row's FOR UPDATE lock and has already refused a deleted_at, so nothing can make this UPDATE miss. Named rather than ignored because a silent zero-rows UPDATE is exactly what an RLS policy that stops admitting this shape would look like.
		return fmt.Errorf("staffauth: redact staff row: %d rows affected, want 1", affected)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO staff_auth_events (staff_id, actor_staff_id, reason)
		 VALUES ($1, $1, 'login_deleted')`,
		staffID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("staffauth: record login deletion: %w", err)
	}
	return nil
}

// pendingMailAddressedToHer is one outbox table's "every pending row for this uid"
// statement. Written out one per table rather than built from a table
// name and a column name: the interpolation that would take would be a
// SQL string this file assembles at runtime, which is the shape gosec
// refuses on sight and which no reader can check by eye. Four literals
// are longer and are exactly what they appear to be.
var pendingMailAddressedToHer = []string{
	`UPDATE staff_token_mail_outbox   SET status = 'sent', sent_at = $1 WHERE identity_uid = $2 AND status = 'pending'`,
	`UPDATE session_notice_outbox     SET status = 'sent', sent_at = $1 WHERE identity_uid = $2 AND status = 'pending'`,
	`UPDATE staff_mfa_recovery_outbox SET status = 'sent', sent_at = $1 WHERE recipient_identity_uid = $2 AND status = 'pending'`,
}

// staff_email_change_outbox is deliberately not in that list, and this is
// the one omission worth naming rather than leaving to be noticed. Its
// notice is composed from the old_email captured on the row, reads
// nothing about her at send time, and is still true afterwards: somebody
// changed the address on an account, and the person who used to own that
// mailbox is exactly who needs to hear it, whatever became of the account
// since. Suppressing it would hand anyone who reached her session a way
// to move her address and then silence the notice by deleting the login.

// resolveQueuedMail marks every pending outbox row addressed to her sent,
// having sent nothing -- ADR-0033's skip-at-send recheck, moved to the
// source. It runs before the sentinel is written, for the same reason the
// session delete does: every table here is keyed on identity_uid, so a
// row addressed to the old uid becomes unfindable the moment it changes.
//
// Each worker also rechecks her live state at send time, and that recheck
// is not redundant with this. This resolves what is queued at the moment
// she deletes her login; the recheck covers a row queued by a request
// already in flight, which no statement in this transaction can see.
// Between them, a row naming her can neither be mailed nor dead-lettered.
//
// It also closes the one case the send-time recheck cannot: a queued
// password reset. That row's identity was resolved through Identity
// Platform, which holds Client Portal accounts too, so a worker seeing no
// staff row cannot tell a deleted Staff person from a Client and must not
// guess. Here there is no guess to make -- this uid is hers, and she just
// asked for it to stop existing.
func resolveQueuedMail(ctx context.Context, tx *sql.Tx, uid string, now time.Time) error {
	for _, statement := range pendingMailAddressedToHer {
		if _, err := tx.ExecContext(ctx, statement, now, uid); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return fmt.Errorf("staffauth: resolve queued mail: %w", err)
		}
	}
	return nil
}
