// Package sessionmint is #837's one session-issue seam: the ritual every
// sign-in path copied around authn.MintSession -- begin the transaction,
// run the caller's own business step, evict a live session in the other
// population (#610), silently end whatever session this browser already
// held in the same population, mint the new one, run the caller's own
// post-mint step, commit, nudge the eviction notice, set the cookie and
// write the body.
//
// Two adapters describe the only two things that vary across the seven
// callers: which tier is being minted and whether the mint carries a
// second factor. Staff builds one from a verified ID token; Portal is
// always the same value, because a Client never carries a second factor
// (ADR-0026).
//
// It could not live in authn (authn importing sessionnotice, which
// sessionevict needs, would cycle back through authn) or in session
// (session already imports staffauth for staffauth.GatedRouter, and
// three of the seven callers live in staffauth). sessionevict already
// imports exactly what eviction needs and is imported by every one of
// the seven callers' packages without looping back, so this package
// sits beside it rather than inside authn or session, and composes it
// rather than duplicating it.
package sessionmint

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/sessionevict"
	"doula-cloud/api/internal/tasknudge"
)

// msgInternalError is the body a caller sees for a failure that carries
// no more specific detail -- the same literal every package in this
// cluster writes for its own internal errors.
const msgInternalError = "internal error"

// Adapter is the two-field difference between minting a Staff session
// and a Portal Account one: which tier's rules govern eviction and
// lifetime, and whether the session shows a second factor.
type Adapter struct {
	Tier         authn.Tier
	SecondFactor bool
}

// Staff builds the adapter a Staff mint uses, reading the second factor
// off the token that was verified to reach the call -- #606's decision
// 3, unchanged by this package.
func Staff(verified authn.VerifiedToken) Adapter {
	return Adapter{Tier: authn.TierStaff, SecondFactor: verified.SecondFactor}
}

// Portal is the adapter every Portal Account mint uses. A Client never
// carries a second factor: Identity Platform is Staff-only from ADR-0026
// on.
func Portal() Adapter {
	return Adapter{Tier: authn.TierPortal, SecondFactor: false}
}

// Refusal is what a Step reports when it declines to mint. It is not an
// error: an expired invitation, a dead magic link and an already-claimed
// invite are ordinary answers, not failures of the BFF.
type Refusal struct {
	Status int
	// Code names the refusal for a caller that switches on it rather
	// than matching prose. The zero value defers to
	// apierr.CodeForStatus(Status), which is what most refusals want.
	Code apierr.Code
	// Message is the body text a caller reads.
	Message string
	// Keep tells Issue to commit the transaction despite refusing to
	// mint, for the one caller whose step wrote something that must
	// survive the refusal -- staffauth's accept-invite flips a pending
	// Invitation to 'expired' on the way past and that discovery is
	// worth keeping. Every other refusal rolls back, which is what
	// leaves a signup or invitation still claimable on the confirmed
	// retry.
	Keep bool
}

// Result is what a Step reports on success: the identity to mint a
// session for -- known upfront for a Staff sign-in, freshly minted for a
// Portal Account accept -- the body to encode, and the status to write
// it with. Status of zero writes nothing explicit, which is a plain 200.
type Result struct {
	IdentityUID string
	Body        any
	Status      int
	Refusal     *Refusal
}

// Step is the caller's in-transaction business logic: whatever has to
// happen before a session can be minted, running on the same
// transaction the mint and the eviction check share. Its own rollback
// undoes what it wrote when eviction is refused or the step itself
// refuses -- which is why a magic-link redeem may safely spend its token
// first and only ask about eviction afterwards.
type Step func(ctx context.Context, tx *sql.Tx) (Result, error)

// Finish is the rare post-mint step a caller runs after the new session
// exists but before the transaction commits. Only one caller needs it:
// portalinvite's accept records an activity.Entry that must be the last
// write before commit, so it cannot run inside Step, before eviction and
// the mint. nil for every other caller.
type Finish func(ctx context.Context, tx *sql.Tx) error

// Issue runs the seven-seam ritual on tx, an already-open transaction --
// staffauth's three bootstrap paths get theirs from authn.BeginBootstrap,
// which reads a Bearer ID token inside it, so Issue takes what it is
// given rather than opening a second one. IssueFromDB, below, is the
// version for a caller that has only a *sql.DB.
//
// Order is step, evict, end, mint, finish, commit, nudge, cookie, body --
// exactly the acceptance criterion's own words. step runs first so its
// rollback is what undoes a refused mint. Eviction is #610's
// cross-population check: a live session in the other population is
// refused once (409, unconfirmed) and deleted on the confirmed retry.
// Whatever this browser's cookie names in adapter.Tier's own population
// is then ended silently and unconditionally, confirmed or not -- the
// same replacement mfaenroll always did for its own pre-enrolment
// session, generalised to every seam: the cookie is being overwritten
// either way, so a same-population row left behind is a token that still
// verifies, exactly the failure #610 named for the cross-population
// case. EndSession no-ops for a token naming nothing, so this is safe to
// run unconditionally once eviction has already been settled.
//
// Issue always writes a response before returning, success or refusal,
// so a caller's own defer-rollback need only roll back when committed
// comes back false.
func Issue(w http.ResponseWriter, r *http.Request, tx *sql.Tx, enq tasknudge.Enqueuer, adapter Adapter, step Step, finish Finish) (committed bool) {
	ctx := r.Context()
	now := time.Now()

	result, err := step(ctx, tx)
	if err != nil {
		apierr.WriteError(w, msgInternalError, http.StatusInternalServerError)
		return false
	}
	if result.Refusal != nil {
		return writeRefusal(w, tx, result.Refusal)
	}

	queued, ok := sessionevict.Apply(w, r, tx, adapter.Tier, now)
	if !ok {
		return false
	}
	if cookie, err := r.Cookie(authn.SessionCookieName); err == nil {
		if err := authn.EndSession(ctx, tx, cookie.Value); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, msgInternalError, http.StatusInternalServerError)
			return false
		}
	}

	cookie, err := authn.MintSession(ctx, tx, result.IdentityUID, adapter.SecondFactor, now)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, msgInternalError, http.StatusInternalServerError)
		return false
	}

	if finish != nil {
		if err := finish(ctx, tx); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, msgInternalError, http.StatusInternalServerError)
			return false
		}
	}

	if err := tx.Commit(); err != nil {
		// coverage:ignore reason: DB commit failure, not exercised by unit tests
		apierr.WriteError(w, msgInternalError, http.StatusInternalServerError)
		return false
	}

	if queued {
		tasknudge.Fire(enq, tasknudge.SessionNotice)(ctx)
	}
	http.SetCookie(w, cookie)
	writeBody(w, result.Status, result.Body)
	return true
}

// IssueFromDB is Issue for a caller that has not already begun a
// transaction: session.CreateHandler, portalinvite.AcceptInviteHandler
// and clientauth.RedeemMagicLinkHandler each read no Bearer ID token
// inside their own transaction, so nothing forces them to open one
// before calling in, unlike staffauth's three bootstrap paths. It
// returns whether the mint committed, for the one caller
// (session.CreateHandler) that has a best-effort step of its own to run
// once the session is real -- #345's new-sign-in notice, queued on the
// pool rather than this transaction because it is best-effort and must
// never turn a legitimate sign-in into a 500.
func IssueFromDB(w http.ResponseWriter, r *http.Request, db *sql.DB, enq tasknudge.Enqueuer, adapter Adapter, step Step, finish Finish) (committed bool) {
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		// coverage:ignore reason: DB connection failure, not exercised by unit tests
		apierr.WriteError(w, msgInternalError, http.StatusInternalServerError)
		return false
	}
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	committed = Issue(w, r, tx, enq, adapter, step, finish)
	return committed
}

// writeRefusal writes ref's response, committing tx first when ref.Keep
// asks for what the step already wrote to survive -- matching the order
// every deleted per-handler ritual used: commit (or roll back), then
// answer.
func writeRefusal(w http.ResponseWriter, tx *sql.Tx, ref *Refusal) (committed bool) {
	if ref.Keep {
		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			apierr.WriteError(w, msgInternalError, http.StatusInternalServerError)
			return false
		}
		committed = true
	}
	code := ref.Code
	if code == "" {
		code = apierr.CodeForStatus(ref.Status)
	}
	apierr.Write(w, ref.Status, code, ref.Message, nil)
	return committed
}

// writeBody writes status if it is not the zero value, then encodes
// body -- Content-Type first, the same order every deleted per-handler
// ritual used.
func writeBody(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	if status != 0 {
		w.WriteHeader(status)
	}
	// coverage:ignore reason: response encoding failure, not exercised by unit tests
	if err := json.NewEncoder(w).Encode(body); err != nil {
		apierr.WriteError(w, msgInternalError, http.StatusInternalServerError)
	}
}
