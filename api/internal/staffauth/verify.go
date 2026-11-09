package staffauth

import (
	"database/sql"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authmail"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authtoken"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/dbtx"
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
	return authn.Handler(db, authn.TierStaff, func(w http.ResponseWriter, r *http.Request, tx *sql.Tx, uid string) {
		// Sets app.current_identity_uid on the way, which is what `staff`'s
		// own self-visibility RLS policy reads -- the pre-Practice
		// preamble every route in this family now reaches for rather
		// than writing out (#1182).
		if _, ok := requireSelf(w, r, tx, uid); !ok {
			return
		}

		queueLinkAndAccept(w, r, tx, uid, authtoken.PurposeStaffEmailVerification, authmail.VerificationLinkLifetime, authmail.KindEmailVerification)
	})
}

// queueLinkAndAccept mints a fresh single-use link token for uid, queues
// the mail that carries it, and commits tx with a 202 -- the whole of a
// request-a-link route once its caller is known. A failure writes the 500
// itself.
func queueLinkAndAccept(w http.ResponseWriter, r *http.Request, tx *sql.Tx, uid string, purpose authtoken.Purpose, lifetime time.Duration, kind authmail.TokenMailKind) {
	token, err := authtoken.Mint(r.Context(), tx, uid, purpose, lifetime, clock.Now(r.Context()))
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}
	if err := authmail.QueueTokenMail(r.Context(), tx, uid, kind, token); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}

	dbtx.CommitWith(w, tx, http.StatusAccepted)
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
		req, ok := authtoken.DecodeSpend(w, r, func(q *SpendVerificationRequest) *string { return &q.Token })
		if !ok {
			return
		}

		tx, ok := dbtx.Begin(w, r, db)
		if !ok {
			// coverage:ignore reason: the helper already wrote the failure response; reached only on a DB failure or a missing Middleware value, not exercised by unit tests
			return
		}
		defer dbtx.Rollback(tx)

		uid, ok := authtoken.SpendOrRefuse(w, r, tx, req.Token, authtoken.PurposeStaffEmailVerification, clock.Now(r.Context()))
		if !ok {
			return
		}

		if err := accounts.SetEmailVerified(r.Context(), uid); err != nil {
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		dbtx.CommitWith(w, tx, http.StatusNoContent)
	})
}
