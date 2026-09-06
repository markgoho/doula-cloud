package clientauth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authtoken"
	"doula-cloud/api/internal/sessionmint"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
)

// MagicLinkLifetime is #617's 15-minute expiry (ADR-0026): Mailgun
// retains message logs as a sold plan feature (ADR-0012), so the token
// sits in a vendor's records for longer than it sits in her inbox. A
// short life is what limits that exposure.
const MagicLinkLifetime = 15 * time.Minute

// RequestMagicLinkRequest is the body of a sign-in link request: the
// address to send it to.
type RequestMagicLinkRequest struct {
	Email string `json:"email"`
}

// RequestMagicLinkHandler starts a Client sign-in. Public and
// unauthenticated -- a Client has no password and no other credential to
// present -- and answers identically whether or not the address names a
// Portal Account: #168 named account-enumeration oracles as a class to
// refuse, and this is the same class staffauth.RequestResetHandler
// refuses it for. Rate limiting (#602) is applied by the route table,
// keyed on this request's own email field (ratelimit.JSONFieldRule)
// since there is no session to key on this early.
func RequestMagicLinkHandler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req RequestMagicLinkRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierr.WriteError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		address := staffauth.NormalizeAddress(req.Email)
		if address == "" {
			apierr.WriteError(w, "email is required", http.StatusBadRequest)
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

		identifier, found, err := findPortalAccountByAddress(r.Context(), tx, address)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !found {
			// Identical response to the address-found branch below --
			// nothing here may tell a caller whether a Portal Account
			// exists. No token minted, nothing queued, nothing to commit.
			w.WriteHeader(http.StatusAccepted)
			return
		}

		token, err := authtoken.Mint(r.Context(), tx, identifier, authtoken.PurposeClientMagicLink, MagicLinkLifetime, time.Now())
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := queueMagicLinkMail(r.Context(), tx, identifier, token); err != nil {
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

// findPortalAccountByAddress resolves address to the Portal Account
// identifier it signs in as, via portal_accounts_signin_lookup (00074) --
// the one door that admits this read before app.current_identity_uid has
// a value.
func findPortalAccountByAddress(ctx context.Context, tx *sql.Tx, address string) (identifier string, found bool, err error) {
	err = tx.QueryRowContext(ctx, `SELECT identifier FROM portal_accounts WHERE lower(sign_in_address) = $1`, address).Scan(&identifier)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", false, fmt.Errorf("clientauth: find portal account by address: %w", err)
	}
	return identifier, true, nil
}

// RedeemMagicLinkRequest is the body of a sign-in link spend: the token
// from the emailed link.
type RedeemMagicLinkRequest struct {
	Token string `json:"token"`
}

// redeemMagicLinkResponse is the minimal success body -- the frontend
// follows this up with GET /api/portal/session, same as after any other
// sign-in, to decide where to land.
type redeemMagicLinkResponse struct {
	OK bool `json:"ok"`
}

// RedeemMagicLinkHandler turns a sign-in link into a session. Public and
// pre-account, reading no Bearer token or cookie: the link's own token is
// the whole credential, the same shape staffauth.SpendResetHandler and
// staffauth.SpendVerificationHandler use.
//
// Called only by the POST behind the redeem page's Continue button, never
// by the GET that renders it (ADR-0026) -- a scanner following the link
// to inspect it must not burn it before she reads the mail. Spending the
// token and minting the session happen inside one transaction, so a
// failure after the spend never strands her with a burned link and no
// session.
//
// That same Continue button is where #610 hangs its cross-population
// warning, which is why enq is here: evicting a live Staff session
// queues a notice, and the outbox wants a nudge once this commits.
func RedeemMagicLinkHandler(db *sql.DB, enq tasknudge.Enqueuer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req RedeemMagicLinkRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierr.WriteError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		req.Token = strings.TrimSpace(req.Token)
		if req.Token == "" {
			apierr.WriteError(w, "token is required", http.StatusBadRequest)
			return
		}

		step := func(ctx context.Context, tx *sql.Tx) (sessionmint.Result, error) {
			// #610/#837, after the spend and not before it: a token that
			// is already burned or was never issued gets the 400 below,
			// so a stranger following a dead link is never shown a
			// warning about a session he is not going to displace. A
			// refusal here rolls the transaction back, which un-spends
			// the token -- so the confirmed retry redeems the same live
			// link.
			identifier, err := authtoken.Spend(ctx, tx, req.Token, authtoken.PurposeClientMagicLink, time.Now())
			if errors.Is(err, authtoken.ErrInvalid) {
				return sessionmint.Result{Refusal: &sessionmint.Refusal{
					Status: http.StatusBadRequest, Message: "this link is invalid or has expired -- ask for a new one",
				}}, nil
			}
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				return sessionmint.Result{}, fmt.Errorf("clientauth: spend magic link: %w", err)
			}
			return sessionmint.Result{IdentityUID: identifier, Body: redeemMagicLinkResponse{OK: true}}, nil
		}

		sessionmint.IssueFromDB(w, r, db, enq, sessionmint.Portal(), step, nil)
	})
}
