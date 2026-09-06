package portalinvite

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/pgerr"
	"doula-cloud/api/internal/portalaccount"
	"doula-cloud/api/internal/sessionevict"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
)

// AcceptInviteRequest is the body of an accept-invite request: the
// one-time token from InviteResponse.
type AcceptInviteRequest struct {
	InviteToken string `json:"inviteToken"`
}

// AcceptInviteResponse identifies the Client the invite claimed. The
// frontend follows this up with GET /api/portal/session, same as after
// login, to decide where to land.
type AcceptInviteResponse struct {
	ClientID string `json:"clientId"`
}

// AcceptInviteHandler lets an invited Client claim their pending
// client_portal_users row. Public and pre-account, reading no Bearer
// token or session (#617, ADR-0026): a Client has no Identity Platform
// account to authenticate through any more, so the invitation token
// itself is the whole credential -- the same shape
// staffauth.SpendResetHandler and clientauth.RedeemMagicLinkHandler use
// for a mailed token. The identity this handler stores is a Portal
// Account identifier Doula Cloud mints itself (portalaccount.NewIdentifier),
// never an Identity Platform uid. Like clientauth.SessionHandler, this
// runs before any Client is resolved, so it never sets
// app.current_client_id.
//
// ADR-0026 makes the invitation the first magic link, so this is a
// Client sign-in and #610's cross-population check applies here exactly
// as it does to every later one -- enq is the nudge for the notice an
// evicted Staff session queues.
func AcceptInviteHandler(db *sql.DB, enq tasknudge.Enqueuer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		var req AcceptInviteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierr.WriteError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		req.InviteToken = strings.TrimSpace(req.InviteToken)
		if req.InviteToken == "" {
			apierr.WriteError(w, "inviteToken is required", http.StatusBadRequest)
			return
		}

		resp, identifier, engagementID, reused, status, code, msg := acceptInvite(r, tx, req.InviteToken)
		if status != http.StatusOK {
			apierr.Write(w, status, code, msg, nil)
			return
		}

		// #610, after acceptInvite and not before it: a dead or already
		// claimed invitation is refused above, so nobody is warned about
		// a session this token was never going to displace. The refusal
		// rolls back the rows acceptInvite just wrote, and the confirmed
		// retry writes them again.
		now := time.Now()
		evicted, ok := sessionevict.Apply(w, r, tx, authn.TierPortal, now)
		if !ok {
			return
		}

		// Create the session before committing, so a failure rolls the
		// new rows back instead of leaving them committed behind a
		// response that reports failure (#145). A Client never carries a
		// second factor -- Identity Platform is Staff-only from here on
		// (ADR-0026).
		cookie, err := authn.MintSession(r.Context(), tx, identifier, false, now)
		if err != nil {
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		// activity.ScopeToPractice's contract is "nothing runs after it
		// but the Record call it exists for" (see offer.recordPreAccountDecline):
		// this must be the last write before Commit.
		if err := activity.ScopeToPractice(r.Context(), tx, resp.practiceID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		// #309: an accept that reuses an existing Portal Account (ADR-0015)
		// records its own action -- no Portal Account came into being, an
		// existing one gained reach into this Practice.
		action := activity.ActionPortalAccountProvisioned
		if reused {
			action = activity.ActionPortalAccountLinked
		}
		if err := activity.Record(r.Context(), tx, activity.Entry{
			PracticeID:  resp.practiceID,
			SubjectKind: activity.SubjectEngagement,
			SubjectID:   engagementID,
			Action:      string(action),
			Actor:       activity.ClientActor(resp.ClientID),
		}); err != nil {
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

		if evicted {
			tasknudge.Fire(enq, tasknudge.SessionNotice)(r.Context())
		}
		http.SetCookie(w, cookie)

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(resp.AcceptInviteResponse); err != nil {
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// acceptResult is acceptInvite's success value: the public response plus
// the practiceID the handler needs for activity.ScopeToPractice, which
// portal_accounts and client_portal_users carry no column for.
type acceptResult struct {
	AcceptInviteResponse
	practiceID string
}

// acceptInvite spends inviteToken: it resolves the pending
// client_portal_users row it names, refuses one that has expired, reads
// the Client's own rows while app.current_client_id is set, then drops
// that setting before minting the Portal Account and claiming the row.
//
// That drop is load-bearing, not tidiness: Postgres checks an UPDATE's
// new row against a table's SELECT policies too, not only its own WITH
// CHECK (so a write can't produce a row the writer couldn't otherwise
// read back). client_portal_users_self_visibility (00006) is the only
// SELECT policy that can admit a row whose identity_uid now equals
// app.current_identity_uid, and its own qual demands
// app.current_client_id be unset -- so claiming the row while
// current_client_id is still set from the read above is invisible to
// every SELECT policy and the UPDATE is refused with a bare RLS error,
// no matter how correct the UPDATE policy's own WITH CHECK is.
func acceptInvite(r *http.Request, tx *sql.Tx, inviteToken string) (result acceptResult, identifier, engagementID string, reused bool, status int, code apierr.Code, msg string) {
	ctx := r.Context()

	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.invite_token', $1, true)`, inviteToken); err != nil {
		return acceptResult{}, "", "", false, http.StatusInternalServerError, apierr.CodeInternal, MsgInternalError
	}

	var clientID string
	var expiresAt sql.NullTime
	err := tx.QueryRowContext(ctx,
		`SELECT client_id, invite_token_expires_at FROM client_portal_users
		 WHERE identity_uid IS NULL AND invite_token = $1`,
		inviteToken,
	).Scan(&clientID, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return acceptResult{}, "", "", false, http.StatusNotFound, apierr.CodeNotFound, "invite not found or already accepted"
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return acceptResult{}, "", "", false, http.StatusInternalServerError, apierr.CodeInternal, MsgInternalError
	}

	// A pending row with no expiry can never be accepted -- the property
	// #616's AC wants -- and one past its expiry is refused the same way.
	// There is no status column to flip here the way staffauth.acceptInvite
	// flips practice_invitations.status: a re-invite already rotates both
	// invite_token and invite_token_expires_at, so nothing needs marking
	// on the way past. apierr has no dedicated Gone/expired code, so this
	// takes CodeForStatus's default the way staffauth's own 410 path does
	// (via apierr.WriteError) -- an explicit derivation, not a mistaken
	// CodeInternal literal for what is an expected, not a server, failure.
	if !expiresAt.Valid || !expiresAt.Time.After(time.Now()) {
		return acceptResult{}, "", "", false, http.StatusGone, apierr.CodeForStatus(http.StatusGone), "this invitation has expired -- ask your practice to send a new one"
	}

	// Opens clients_self_visibility (00009_messaging_client_portal_read.sql)
	// and engagements_client_visibility (00006_client_portal_users.sql),
	// the only doors that admit this Client's own rows before her Portal
	// Account exists to authenticate her. Read everything this Client-scoped
	// window has to offer now, then drop it below -- see the doc comment
	// above.
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_client_id', $1, true)`, clientID); err != nil {
		return acceptResult{}, "", "", false, http.StatusInternalServerError, apierr.CodeInternal, MsgInternalError
	}

	var email, practiceID string
	if err := tx.QueryRowContext(ctx, `SELECT email, practice_id FROM clients WHERE id = $1`, clientID).Scan(&email, &practiceID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests -- clientID came from a row this same tx just read
		return acceptResult{}, "", "", false, http.StatusInternalServerError, apierr.CodeInternal, MsgInternalError
	}

	// The most recent Engagement is the one an activity entry names when a
	// Client holds more than one -- v1 only ever invites a Client through a
	// specific Engagement's own portal-invite door (InviteHandler), so in
	// practice there is exactly one.
	if err := tx.QueryRowContext(ctx,
		`SELECT id FROM engagements WHERE client_id = $1 ORDER BY created_at DESC LIMIT 1`, clientID,
	).Scan(&engagementID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests -- InviteHandler's own door guarantees at least one Engagement exists
		return acceptResult{}, "", "", false, http.StatusInternalServerError, apierr.CodeInternal, MsgInternalError
	}

	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_client_id', '', true)`); err != nil {
		return acceptResult{}, "", "", false, http.StatusInternalServerError, apierr.CodeInternal, MsgInternalError
	}

	// The invitation is the single point where the Practice's contact
	// address and the Client's own sign-in address meet (ADR-0026):
	// accepting it proves that mailbox, so the Portal Account's sign-in
	// address defaults to it.
	identifier = portalaccount.NewIdentifier()
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_identity_uid', $1, true)`, identifier); err != nil {
		return acceptResult{}, "", "", false, http.StatusInternalServerError, apierr.CodeInternal, MsgInternalError
	}

	// SAVEPOINT, mirroring website.upsertWithSlugRetry (00045): a unique
	// violation on the INSERT below is an expected outcome this function
	// keeps querying past, not a fatal error, and Postgres aborts an
	// entire transaction on any unhandled error until it is rolled back
	// to a savepoint (or the whole tx). Without this, the reuse lookup
	// right after would itself fail with "current transaction is
	// aborted".
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SAVEPOINT portal_account_insert`); err != nil {
		return acceptResult{}, "", "", false, http.StatusInternalServerError, apierr.CodeInternal, MsgInternalError
	}

	signInAddress := staffauth.NormalizeAddress(email)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO portal_accounts (identifier, sign_in_address) VALUES ($1, $2)`,
		identifier, signInAddress,
	); err != nil {
		if !pgerr.IsUniqueViolationOn(err, "portal_accounts_sign_in_address") {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return acceptResult{}, "", "", false, http.StatusInternalServerError, apierr.CodeInternal, MsgInternalError
		}

		// coverage:ignore reason: DB query failure, not exercised by unit tests
		if _, err := tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT portal_account_insert`); err != nil {
			return acceptResult{}, "", "", false, http.StatusInternalServerError, apierr.CodeInternal, MsgInternalError
		}

		// ADR-0015: a Portal Account reaches many Clients, at most one per
		// Practice. This address already has one -- reuse it rather than
		// refuse the accept, unless it already reaches a Client at this
		// same Practice (a second invitation from the same Practice for
		// the same person, refused rather than silently merged).
		// portal_account_reuse_for_accept (00081) is a SECURITY DEFINER
		// function: this caller has no session context that would let any
		// existing RLS policy answer either half of that question.
		var conflictingClientID sql.NullString
		if err := tx.QueryRowContext(ctx,
			`SELECT identifier, conflicting_client_id FROM portal_account_reuse_for_accept($1, $2)`,
			signInAddress, practiceID,
		).Scan(&identifier, &conflictingClientID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests -- the INSERT above just proved a matching row exists
			return acceptResult{}, "", "", false, http.StatusInternalServerError, apierr.CodeInternal, MsgInternalError
		}
		if conflictingClientID.Valid {
			return acceptResult{}, "", "", false, http.StatusConflict, apierr.CodeConflict, "you already have portal access at this practice -- sign in instead of accepting a new invitation"
		}

		// Re-point app.current_identity_uid at the Portal Account being
		// reused: client_portal_users_accept_update's WITH CHECK (00026)
		// requires the row's new identity_uid to equal this setting, and
		// it was still holding the freshly minted (and now unused)
		// identifier from above.
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_identity_uid', $1, true)`, identifier); err != nil {
			return acceptResult{}, "", "", false, http.StatusInternalServerError, apierr.CodeInternal, MsgInternalError
		}
		reused = true
	} else {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		if _, err := tx.ExecContext(ctx, `RELEASE SAVEPOINT portal_account_insert`); err != nil {
			return acceptResult{}, "", "", false, http.StatusInternalServerError, apierr.CodeInternal, MsgInternalError
		}
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE client_portal_users
		    SET identity_uid = $1, invite_token = NULL, invite_token_expires_at = NULL
		  WHERE invite_token = $2`,
		identifier, inviteToken,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return acceptResult{}, "", "", false, http.StatusInternalServerError, apierr.CodeInternal, MsgInternalError
	}

	return acceptResult{AcceptInviteResponse: AcceptInviteResponse{ClientID: clientID}, practiceID: practiceID}, identifier, engagementID, reused, http.StatusOK, "", ""
}
