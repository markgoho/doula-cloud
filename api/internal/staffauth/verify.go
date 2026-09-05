package staffauth

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authmail"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authtoken"
)

// RequestVerificationHandler lets a signed-in Staff member ask for a
// fresh email-verification link. This is the AC #613 added while
// resolving #169: a 24-hour link and ADR-0010's retry window are roughly
// the same length ("about five attempts over about a day"), so a link
// delivered on a late retry can arrive already dead -- and unlike
// password reset, whose request endpoint *is* the re-request, nothing
// else lets a verified-nowhere-yet Staff member ask again. She is signed
// in (#606's Practice gate refuses her until she is verified, but
// SessionHandler and this route both run before that gate), so this
// mints straight from her session's identity -- no address to type, no
// invitation token to hold.
//
// Mounted outside the Practice-scoped middleware, like UpdateWorkStateHandler:
// verifying an address is a fact about the person, not about a
// Membership.
func RequestVerificationHandler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, uid, ok := authn.Begin(w, r, db)
		if !ok {
			return
		}
		committed := false
		defer func() {
			// coverage:ignore reason: every step below this point that can fail without committing is itself a DB failure already marked coverage:ignore, so this defer's rollback body is reached only alongside one of those, never standalone
			if !committed {
				_ = tx.Rollback()
			}
		}()

		token, err := authtoken.Mint(r.Context(), tx, uid, authtoken.PurposeStaffEmailVerification, authmail.VerificationLinkLifetime, time.Now())
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := authmail.QueueTokenMail(r.Context(), tx, uid, authmail.KindEmailVerification, token); err != nil {
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

// SpendVerificationRequest is the body of a verification-link spend: the
// token from the emailed link.
type SpendVerificationRequest struct {
	Token string `json:"token"`
}

// SpendVerificationHandler turns a verification link into a verified
// Identity Platform account. It runs before any session necessarily
// exists -- a verification link can be opened in a browser signed out of
// everything -- so unlike signup and invitation acceptance it reads no
// Bearer token at all: the link's own token is the whole credential, the
// same shape offer.ReadHandler's pre-account read uses. Not mounted
// behind authn.BeginBootstrap, and not behind authn.Begin either.
func SpendVerificationHandler(accounts authn.AccountManager, db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req SpendVerificationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierr.WriteError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		req.Token = strings.TrimSpace(req.Token)
		if req.Token == "" {
			apierr.WriteError(w, "token is required", http.StatusBadRequest)
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

		uid, err := authtoken.Spend(r.Context(), tx, req.Token, authtoken.PurposeStaffEmailVerification, time.Now())
		if errors.Is(err, authtoken.ErrInvalid) {
			apierr.WriteError(w, "this link is invalid or has expired -- ask for a new one", http.StatusBadRequest)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := accounts.SetEmailVerified(r.Context(), uid); err != nil {
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
