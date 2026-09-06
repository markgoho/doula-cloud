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

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authtoken"
	"doula-cloud/api/internal/pgerr"
	"doula-cloud/api/internal/staffauth"
)

// AddressChangeLifetime is #619's confirmation-link expiry. 24 hours,
// not MagicLinkLifetime's 15 minutes: this token mints no session and
// grants no credential -- spending it only moves an address she has
// already proved she can read -- so it sits in the same tier as
// PurposeStaffEmailVerification, whose own 24 hours ADR-0026 justifies
// as long enough to survive an outbox retry cycle.
const AddressChangeLifetime = 24 * time.Hour

// msgNotAPortalAccount refuses a caller whose session is real but whose
// identity is not a Portal Account -- a signed-in Staff member reaching
// this route. Nothing about her own account is disclosed.
const msgNotAPortalAccount = "this is not a portal account"

// The two refusals a new address can draw, in GOV.UK's own wording
// (ADR-0021): start with the field's noun, say what to do about it, and
// never write "please", "valid", "invalid" or "required".
const (
	MsgAddressRequired  = "Enter your new sign-in address"
	MsgAddressMalformed = "Enter an email address in the correct format, like name@example.com"
)

// RequestAddressChangeRequest is the body of a sign-in-address change
// request: the new address to prove. Named `email` rather than
// `newEmail` so portalAddressChangeRequestRules' JSONFieldRule can key
// on the same field name every other address-keyed rate limit in this
// product already uses.
type RequestAddressChangeRequest struct {
	Email string `json:"email"`
}

// RequestAddressChangeHandler starts a Client's own sign-in-address
// change (#619, ADR-0026). Authenticated by her live portal session --
// this is the one address change that is hers, not Staff's -- and the
// confirmation link goes to the *new* address, which is the whole point:
// nothing moves until she proves she reads that mailbox.
//
// It answers 202 whether or not the address is already some other Portal
// Account's, exactly as RequestMagicLinkHandler answers identically for
// an address that names an account and one that does not (#168's
// account-enumeration class). The collision is caught at spend time
// instead, where she has already proved the mailbox and so learns
// nothing she could not have learned by asking for a sign-in link at it.
func RequestAddressChangeHandler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Authenticate before reading a byte of the body, the order
		// staffauth's own email-change handler keeps: an unauthenticated
		// caller gets a 401, never a critique of the address she sent.
		tx, uid, _, ok := authn.Begin(w, r, db)
		if !ok {
			return
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		// A __session cookie is one cookie for two populations
		// (ADR-0026), so a signed-in Staff member's session reaches this
		// route with a perfectly valid uid that names no Portal Account.
		// Without this check her uid would be minted a token against a
		// portal_accounts row that does not exist, and the FK on
		// portal_sign_in_address_changes would surface it as a 500.
		holds, err := isPortalAccount(r.Context(), tx, uid)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !holds {
			apierr.WriteError(w, msgNotAPortalAccount, http.StatusForbidden)
			return
		}

		var req RequestAddressChangeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierr.WriteError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		address := staffauth.NormalizeAddress(req.Email)
		// Both refusals name the field they are about in Details
		// (docs/api-design.md section 7), which is what lets the screen put
		// the message above the input and give the error summary an entry
		// that focuses it -- a bare message would leave both adrift.
		if address == "" {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, MsgAddressRequired,
				map[string]string{"email": MsgAddressRequired})
			return
		}
		if !looksLikeAddress(address) {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, MsgAddressMalformed,
				map[string]string{"email": MsgAddressMalformed})
			return
		}

		// Mint deletes any prior unspent token for this identity and
		// purpose, and portal_sign_in_address_changes cascades off
		// token_hash -- so asking a second time, naming a different
		// address, retires the first address along with its token rather
		// than leaving two live links pointing at two different mailboxes.
		token, err := authtoken.Mint(r.Context(), tx, uid, authtoken.PurposeClientSignInAddressChange, AddressChangeLifetime, time.Now())
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO portal_sign_in_address_changes (token_hash, identifier, new_address) VALUES ($1, $2, $3)`,
			authtoken.Digest(token), uid, address,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := queueAddressChangeMail(r.Context(), tx, uid, address, token); err != nil {
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

// looksLikeAddress is GOV.UK's own email-address rule, no more: an "@"
// with something either side. Anything stricter refuses addresses that
// really deliver, and the only proof that matters here is whether the
// confirmation mail arrives.
func looksLikeAddress(address string) bool {
	at := strings.Index(address, "@")
	return at > 0 && at < len(address)-1
}

// isPortalAccount reports whether identifier names a portal_accounts row,
// read through portal_accounts_signin_lookup (00074) -- the same
// USING (true) SELECT policy the magic-link request reads through.
func isPortalAccount(ctx context.Context, tx *sql.Tx, identifier string) (bool, error) {
	var exists bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM portal_accounts WHERE identifier = $1)`, identifier).Scan(&exists)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, fmt.Errorf("clientauth: check portal account: %w", err)
	}
	return exists, nil
}

// SpendAddressChangeRequest is the body of a confirmation-link spend: the
// token from the mail sent to the new address.
type SpendAddressChangeRequest struct {
	Token string `json:"token"`
}

// spendAddressChangeResponse reports the address now in force, so the
// confirmation screen can say which mailbox signs her in from here --
// she has just proved she reads it, so it discloses nothing.
type spendAddressChangeResponse struct {
	SignInAddress string `json:"signInAddress"`
}

// MsgAddressTaken refuses a confirmation whose address another Portal
// Account has claimed since the link was sent. Safe to say plainly, and
// only here: she has proved this mailbox, so she could have learned the
// same fact by asking for a sign-in link at it.
const MsgAddressTaken = "that address already signs in to Doula Cloud -- choose another"

// MsgAddressLinkInvalid is the one outcome an unusable confirmation link
// gets, whatever made it unusable -- never minted, already spent, or
// expired -- the same single-outcome rule authtoken.ErrInvalid keeps.
const MsgAddressLinkInvalid = "this link is invalid or has expired -- ask for a new one"

// SpendAddressChangeHandler proves the new mailbox and moves the
// address (#619, ADR-0026). Public and session-free by design: the link
// is read in the new mailbox, which may be on a device she has never
// signed in on, and the token is the whole credential. It mints no
// session -- the old address keeps signing her in right up to this
// point, and the new one signs her in from here.
//
// Called only by the POST behind the confirmation page's Continue
// button, never by the GET that renders it (ADR-0026) -- a scanner
// following the link must not burn it before she reads the mail.
func SpendAddressChangeHandler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req SpendAddressChangeRequest
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

		address, status, msg := applyAddressChange(r.Context(), tx, req.Token)
		if status != http.StatusOK {
			apierr.WriteError(w, msg, status)
			return
		}

		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed = true

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(spendAddressChangeResponse{SignInAddress: address}); err != nil {
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// applyAddressChange is the whole spend, inside the caller's one
// transaction: the address moves, the history lands, and a collision
// rolls both back together. Leaving her token spent on a change that did
// not happen would be wrong, so the spend rolls back with it and the
// link she holds still works once she picks another address. (A failed
// UPDATE aborts the Postgres transaction outright, so nothing may run on
// tx afterwards but the rollback the handler's defer performs.)
func applyAddressChange(ctx context.Context, tx *sql.Tx, token string) (address string, status int, msg string) {
	identifier, err := authtoken.Spend(ctx, tx, token, authtoken.PurposeClientSignInAddressChange, time.Now())
	if errors.Is(err, authtoken.ErrInvalid) {
		return "", http.StatusBadRequest, MsgAddressLinkInvalid
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", http.StatusInternalServerError, MsgInternalError
	}

	err = tx.QueryRowContext(ctx,
		`SELECT new_address FROM portal_sign_in_address_changes WHERE token_hash = $1`,
		authtoken.Digest(token),
	).Scan(&address)
	if err != nil {
		// coverage:ignore reason: the companion row is inserted in the same transaction as its token and cascades with it, so a spendable token always has one
		return "", http.StatusInternalServerError, MsgInternalError
	}

	// portal_accounts_self_update (00079) admits exactly one row, the one
	// the spent token named -- so this set_config is the authorization,
	// not a convenience.
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_identity_uid', $1, true)`, identifier); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", http.StatusInternalServerError, MsgInternalError
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE portal_accounts SET sign_in_address = $1 WHERE identifier = $2`, address, identifier,
	); err != nil {
		if pgerr.IsUniqueViolation(err) {
			return "", http.StatusConflict, MsgAddressTaken
		}
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", http.StatusInternalServerError, MsgInternalError
	}

	if err := recordForEachClient(ctx, tx, identifier, activity.ActionPortalSignInAddressChanged); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", http.StatusInternalServerError, MsgInternalError
	}
	return address, http.StatusOK, ""
}

// clientSubject is one Practice's record of the person whose sign-in
// address just moved: ADR-0015 says one Portal Account reaches a Client
// at more than one Practice, so "record the change" is one row per
// Practice that holds her, not one row overall.
//
// It is one row today whatever this code does --
// client_portal_users.identity_uid carries a UNIQUE constraint (00006),
// which is the schema disagreeing with the ADR rather than this loop
// being speculative. That disagreement is #819's to settle; written to
// the ADR because that is the rule as it stands, and because a set of
// rows is what the query returns whichever way #819 goes.
type clientSubject struct {
	clientID   string
	practiceID string
}

// recordForEachClient writes ADR-0022's history for a Portal-Account-wide
// action -- one row per Practice holding a Client behind identifier, since
// ADR-0015 says a single Portal Account can reach Clients at more than
// one Practice. #619's own sign-in-address change was this loop's first
// caller; #618's sign-out-everywhere is its second, which is the bar
// this codebase extracts a shared name at rather than duplicating.
//
// The two loops inside it are deliberately not one: activity.ScopeToPractice
// widens every practice_id-scoped read the transaction makes afterwards,
// so its own doc comment says nothing but the activity insert may follow
// it. Every clients read this needs is therefore done first, and the
// second loop does nothing but scope and record.
func recordForEachClient(ctx context.Context, tx *sql.Tx, identifier string, action activity.ClientAction) error {
	clientIDs, err := listPortalClientIDs(ctx, tx, identifier)
	if err != nil {
		// coverage:ignore reason: listPortalClientIDs only errors on a DB failure, not exercised by unit tests
		return err
	}

	subjects := make([]clientSubject, 0, len(clientIDs))
	for _, clientID := range clientIDs {
		// clients_self_visibility (00009) is the only policy that admits
		// this read without a Staff Practice context: it matches on
		// app.current_client_id, so each row is read under its own scope.
		if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_client_id', $1, true)`, clientID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return fmt.Errorf("clientauth: set current client id: %w", err)
		}
		var practiceID string
		if err := tx.QueryRowContext(ctx, `SELECT practice_id FROM clients WHERE id = $1`, clientID).Scan(&practiceID); err != nil {
			// coverage:ignore reason: client_portal_users.client_id carries a FK to clients, so a listed row always resolves
			return fmt.Errorf("clientauth: resolve client practice: %w", err)
		}
		subjects = append(subjects, clientSubject{clientID: clientID, practiceID: practiceID})
	}

	for _, s := range subjects {
		if err := activity.ScopeToPractice(ctx, tx, s.practiceID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return fmt.Errorf("clientauth: scope activity to practice: %w", err)
		}
		if err := activity.Record(ctx, tx, activity.Entry{
			PracticeID:  s.practiceID,
			SubjectKind: activity.SubjectClient,
			SubjectID:   s.clientID,
			Action:      string(action),
			Actor:       activity.ClientActor(s.clientID),
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return fmt.Errorf("clientauth: record activity: %w", err)
		}
	}
	return nil
}

// listPortalClientIDs reads every Client record this Portal Account
// reaches, through client_portal_users' own self-visibility policy --
// which is why app.current_identity_uid is set before this runs.
func listPortalClientIDs(ctx context.Context, tx *sql.Tx, identifier string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT client_id FROM client_portal_users WHERE identity_uid = $1`, identifier)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("clientauth: list portal client ids: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("clientauth: scan portal client id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("clientauth: iterate portal client ids: %w", err)
	}
	return ids, nil
}
