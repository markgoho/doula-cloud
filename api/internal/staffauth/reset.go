package staffauth

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authmail"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authtoken"
)

// minPasswordLength mirrors Identity Platform's own minimum -- rejecting
// a too-short password here means SpendResetHandler never has to
// distinguish "the Admin SDK rejected this" from any other failure of
// SetPassword; only length is checked server-side, and only here.
const minPasswordLength = 6

// RequestResetRequest is the body of a password-reset request: the
// address to send a reset link to.
type RequestResetRequest struct {
	Email string `json:"email"`
}

// RequestResetHandler starts a Staff password reset. Public and
// unauthenticated -- a person who forgot her password holds no
// credential to present -- and answers identically whether or not the
// address names an account: #168 named account-enumeration oracles as a
// class to refuse, and this is the same class. Rate limiting (#602) is
// applied by the route table, keyed on this request's own email field
// (ratelimit.JSONFieldRule) since there is no Bearer token or session to
// key on this early.
func RequestResetHandler(accounts authn.AccountManager, db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req RequestResetRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierr.WriteError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		address := NormalizeAddress(req.Email)
		if address == "" {
			apierr.WriteError(w, "email is required", http.StatusBadRequest)
			return
		}

		uid, err := accounts.GetAccountByEmail(r.Context(), address)
		if errors.Is(err, authn.ErrAccountNotFound) {
			// Identical response to the address-exists branch below --
			// nothing here may tell a caller whether an account exists.
			w.WriteHeader(http.StatusAccepted)
			return
		}
		if err != nil {
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			// coverage:ignore reason: DB connection failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed := false
		defer func() {
			// coverage:ignore reason: every step below this point that can fail without committing is itself a DB failure already marked coverage:ignore, so this defer's rollback body is reached only alongside one of those, never standalone
			if !committed {
				_ = tx.Rollback()
			}
		}()

		token, err := authtoken.Mint(r.Context(), tx, uid, authtoken.PurposeStaffPasswordReset, authmail.ResetLinkLifetime, time.Now())
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := authmail.QueueTokenMail(r.Context(), tx, uid, authmail.KindPasswordReset, token); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed = true

		w.WriteHeader(http.StatusAccepted)
	})
}

// SpendResetRequest is the body of a password-reset spend: the token
// from the emailed link, and the new password.
type SpendResetRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
}

// SpendResetHandler turns a reset link into a new password. Public,
// pre-account, and reads no Bearer token: the person spending it has, by
// definition, forgotten her credential, so the link's own token is the
// whole proof, the same shape SpendVerificationHandler and
// offer.ReadHandler's pre-account read use.
//
// This deliberately does not mint a session -- Identity Platform's own
// reset does not sign you in either, and one that did would walk
// straight past #167's enforced MFA. It does end every existing session
// for the identity (endAllSessionsAndNotify, shared with
// EndSessionsHandler), because a password only a compromised session
// away from being reset is not meaningfully reset at all.
func SpendResetHandler(accounts authn.AccountManager, db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req SpendResetRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierr.WriteError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		req.Token = strings.TrimSpace(req.Token)
		if req.Token == "" {
			apierr.WriteError(w, "token is required", http.StatusBadRequest)
			return
		}
		if len(req.NewPassword) < minPasswordLength {
			apierr.WriteError(w, fmt.Sprintf("newPassword must be at least %d characters", minPasswordLength), http.StatusBadRequest)
			return
		}

		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			// coverage:ignore reason: DB connection failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		uid, err := authtoken.Spend(r.Context(), tx, req.Token, authtoken.PurposeStaffPasswordReset, time.Now())
		if errors.Is(err, authtoken.ErrInvalid) {
			apierr.WriteError(w, "this link is invalid or has expired -- ask for a new one", http.StatusBadRequest)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := accounts.SetPassword(r.Context(), uid, req.NewPassword); err != nil {
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := endAllSessionsAndNotify(r.Context(), tx, uid); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed = true

		w.WriteHeader(http.StatusNoContent)
	})
}
