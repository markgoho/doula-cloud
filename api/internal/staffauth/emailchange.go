package staffauth

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"

	firebaseauth "firebase.google.com/go/v4/auth"

	"doula-cloud/api/internal/authmail"
	"doula-cloud/api/internal/authn"
)

// ChangeEmailRequest is the body of a Staff email change: the address to
// change to.
type ChangeEmailRequest struct {
	NewEmail string `json:"newEmail"`
}

// ChangeEmailHandler lets a signed-in Staff member change her account
// address. The write goes through the Admin SDK (UpdateUser(Email(...))),
// which also clears emailVerified (authn.AccountManager.SetEmail's own
// contract) -- a changed address goes back through verification the same
// way self-signup does. staff.email is then kept in step with it in the
// same transaction, closing the drift #614 found, and the *old* address
// is notified through the outbox (authmail.QueueEmailChangeNotice),
// never the new one: the old address's owner is who needs to know if she
// did not make this change.
//
// The Admin SDK write runs before any Postgres write, so a rejected
// change (a duplicate address, or the Admin SDK being unreachable) rolls
// back the whole request -- there is nothing here for a later DB failure
// to leave inconsistent.
//
// Mounted outside the Practice-scoped middleware, like
// UpdateWorkStateHandler and RequestVerificationHandler: an email
// address is a fact about the person, not about a Membership.
func ChangeEmailHandler(accounts authn.AccountManager, db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, uid, ok := authn.Begin(w, r, db)
		if !ok {
			return
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		var req ChangeEmailRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		newAddress := NormalizeAddress(req.NewEmail)
		if newAddress == "" {
			http.Error(w, "newEmail is required", http.StatusBadRequest)
			return
		}

		status, msg := changeEmail(r.Context(), tx, accounts, uid, newAddress)
		if status != http.StatusNoContent {
			http.Error(w, msg, status)
			return
		}

		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed = true

		w.WriteHeader(http.StatusNoContent)
	})
}

// changeEmail resolves the caller's current address, changes it via the
// Admin SDK, updates staff.email to match, and queues the old-address
// notice -- in that order, so a rejected Admin SDK write leaves neither
// Postgres row touched.
func changeEmail(ctx context.Context, tx *sql.Tx, accounts authn.AccountManager, uid, newAddress string) (int, string) {
	current, err := accounts.GetAccount(ctx, uid)
	if err != nil {
		return http.StatusInternalServerError, MsgInternalError
	}
	oldAddress := NormalizeAddress(current.Email)

	if err := accounts.SetEmail(ctx, uid, newAddress); err != nil {
		// coverage:ignore reason: firebaseauth.IsEmailAlreadyExists matches only a *internal.FirebaseError the Admin SDK constructs, which cannot be built from outside firebase.google.com/go/v4's own internal package -- requires a real Identity Platform project
		if firebaseauth.IsEmailAlreadyExists(err) {
			// coverage:ignore reason: see above -- unreachable without a real Admin SDK error
			return http.StatusConflict, "that email address is already in use"
		}
		return http.StatusInternalServerError, MsgInternalError
	}

	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_identity_uid', $1, true)`, uid); err != nil {
		return http.StatusInternalServerError, MsgInternalError
	}

	// staff_self_update (00044) is the same self-only, pre-Practice
	// window UpdateWorkStateHandler writes through.
	res, err := tx.ExecContext(ctx, `UPDATE staff SET email = $1 WHERE identity_uid = $2`, newAddress, uid)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return http.StatusInternalServerError, MsgInternalError
	}
	rows, err := res.RowsAffected()
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return http.StatusInternalServerError, MsgInternalError
	}
	if rows == 0 {
		return http.StatusNotFound, MsgNoMatchingStaffAccount
	}

	if err := authmail.QueueEmailChangeNotice(ctx, tx, uid, oldAddress); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return http.StatusInternalServerError, MsgInternalError
	}

	return http.StatusNoContent, ""
}
