package staffauth

import (
	"database/sql"
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
//
// It resolves the caller's `staff` row before minting anything (#1024).
// Every other route in this family already did -- work state, email
// change, saved-code rotation, login deletion all answer
// MsgNoMatchingStaffAccount -- and this one did not, so it was the one
// place a uid with no Staff row still queued a staff_token_mail_outbox
// row. #892's send-time recheck then skipped that row, which made the
// hole look closed from the outside while a token and an outbox row were
// still being written on every call. Refusing here is the difference
// between not writing the row and writing one nothing will send.
func RequestVerificationHandler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, uid, _, ok := authn.Begin(w, r, db, authn.TierStaff)
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

		// Sets app.current_identity_uid on the way, which is what `staff`'s
		// own self-visibility RLS policy reads -- the same call
		// staffauth.Middleware makes before any Practice is known.
		if _, found, err := setIdentityAndResolveStaff(r.Context(), tx, uid); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		} else if !found {
			apierr.WriteError(w, MsgNoMatchingStaffAccount, http.StatusNotFound)
			return
		}

		token, err := authtoken.Mint(r.Context(), tx, uid, authtoken.PurposeStaffEmailVerification, authmail.VerificationLinkLifetime, time.Now())
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := authmail.QueueTokenMail(r.Context(), tx, uid, authmail.KindEmailVerification, token); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
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
		if !apierr.DecodeJSON(w, r, &req) {
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
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
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
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := accounts.SetEmailVerified(r.Context(), uid); err != nil {
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed = true

		w.WriteHeader(http.StatusNoContent)
	})
}
