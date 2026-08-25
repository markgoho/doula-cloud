package offer

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/staffauth"
)

// PreAccountOffer is what someone who is not Staff anywhere yet reads
// about work she has been offered: ADR-0008's four decidable facts --
// Client first initial, general area, exact due date, her fee -- plus the
// free-text terms, and nothing else. No Client name, no Engagement id, no
// Practice name, no Contract figure. The read is a copy of the Offer row
// alone; it joins no other table, which is what makes "the Offer is a
// copy, never a view" true at the query level and not only in prose.
type PreAccountOffer struct {
	OfferID            string  `json:"offerId"`
	State              string  `json:"state"`
	ClientFirstInitial string  `json:"clientFirstInitial"`
	ClientArea         string  `json:"clientArea"`
	DueDate            string  `json:"dueDate"`
	AmountCents        *int64  `json:"amountCents"`
	Terms              *string `json:"terms"`
	ExpiresAt          string  `json:"expiresAt"`
}

// ReadHandler is the pre-account Offer read (#230) -- the one route in
// this codebase mounted outside staffauth.Middleware and behind neither
// a Staff nor a Client session. It is authenticated by two things
// together: the Invitation token from the emailed link, and the six-digit
// code mailed to the same address. Both travel as query parameters,
// following the Invitation link's own precedent, and the code is bounded
// to maxAccessCodeAttempts guesses per Offer (00041) because a six-digit
// secret in front of an unauthenticated endpoint is otherwise a 10^6
// space anyone may walk.
//
// db must be the low-privilege app_runtime connection: the whole read
// runs under 00041's engagement_offers_token_lookup policy, keyed on the
// token digest set below, so a request carrying a token opens exactly the
// Offers that token was mailed with and nothing else.
//
// This route must be declared exempt by name in GatedRouter's registry
// (staffauth.GatedRouter.Exempt) -- ADR-0008 requires the declaration in
// the same change that mounts the route, so the guardrail test sees a
// deliberate entry rather than an absence.
func ReadHandler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimSpace(r.URL.Query().Get("token"))
		code := strings.TrimSpace(r.URL.Query().Get("code"))
		withTokenTx(w, r, db, token, code, func(_ context.Context, _ *sql.Tx, o PreAccountOffer) (any, int, string) {
			return o, http.StatusOK, ""
		})
	})
}

// DeclineByTokenRequest is the body of a pre-account decline: the same
// two credentials the read takes, in a body rather than a query string
// because this one changes something.
type DeclineByTokenRequest struct {
	Token string `json:"token"`
	Code  string `json:"code"`
}

// DeclineByTokenHandler lets the pre-account reader turn the work down
// without joining the Practice first. Accepting cannot work this way --
// engagement_attachments.staff_id is NOT NULL, so acceptance needs the
// person to exist, which means going through the Invitation and creating
// an account -- but declining must not require joining a Practice in
// order to say no to it.
//
// decided_by is NULL here for the same reason ADR-0008 leaves it NULL on
// completion's cascade: there is no staff row, and inventing one would
// record a person who does not exist. The audit answer is the row itself
// -- the Invitation it names, and the moment it was decided, which
// together identify the holder of a token and a mailed code.
func DeclineByTokenHandler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req DeclineByTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		withTokenTx(w, r, db, strings.TrimSpace(req.Token), strings.TrimSpace(req.Code),
			func(ctx context.Context, tx *sql.Tx, o PreAccountOffer) (any, int, string) {
				if o.State == stateDeclined {
					return DecisionResponse{OfferID: o.OfferID, State: stateDeclined}, http.StatusOK, ""
				}
				if o.State != stateOffered {
					return nil, http.StatusConflict, "that offer is no longer open -- it is " + o.State
				}
				if _, err := tx.ExecContext(ctx,
					`UPDATE engagement_offers SET state = 'declined', decided_at = now() WHERE id = $1 AND state = 'offered'`,
					o.OfferID,
				); err != nil {
					// coverage:ignore reason: DB query failure, not exercised by unit tests
					return nil, http.StatusInternalServerError, staffauth.MsgInternalError
				}
				return DecisionResponse{OfferID: o.OfferID, State: stateDeclined}, http.StatusOK, ""
			})
	})
}

// withTokenTx opens a transaction on the token-lookup door, resolves the
// one Offer the token and code together open, runs fn, and commits. Both
// pre-account routes share it because both need exactly the same
// authentication, expiry handling, and attempt accounting -- and because
// a second copy of that logic is the shape of a hole.
func withTokenTx(w http.ResponseWriter, r *http.Request, db *sql.DB, token, code string,
	fn func(context.Context, *sql.Tx, PreAccountOffer) (any, int, string)) {
	offerID := r.PathValue("offerId")
	if !staffauth.ParseUUID(w, "offer", offerID) {
		return
	}
	if token == "" || code == "" {
		http.Error(w, "a token and a code are required", http.StatusBadRequest)
		return
	}

	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		// coverage:ignore reason: DB connection failure, not exercised by unit tests
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	offer, status, msg := resolveByToken(r.Context(), tx, offerID, token, code)
	if status != http.StatusOK {
		// A wrong code is a write -- the attempt counter moved -- and that
		// discovery is worth keeping, the same reasoning acceptInvite
		// applies to an expiry it flips on the way past.
		if status == http.StatusForbidden {
			if err := tx.Commit(); err != nil {
				// coverage:ignore reason: DB commit failure, not exercised by unit tests
				http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
				return
			}
			committed = true
		}
		http.Error(w, msg, status)
		return
	}

	body, status, msg := fn(r.Context(), tx, offer)
	if status != http.StatusOK {
		http.Error(w, msg, status)
		return
	}
	if err := tx.Commit(); err != nil {
		// coverage:ignore reason: DB commit failure, not exercised by unit tests
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return
	}
	committed = true
	writeJSON(w, body)
}

// resolveByToken opens the token-lookup door, finds the Offer, checks the
// code, and expires the Offer if it has run past its own expires_at.
//
// A wrong token and a wrong code are the same 403, and a token that opens
// nothing is a 404 with no detail: whoever is holding a link they were
// not sent learns only that it does not work.
func resolveByToken(ctx context.Context, tx *sql.Tx, offerID, token, code string) (PreAccountOffer, int, string) {
	if _, err := tx.ExecContext(ctx,
		`SELECT set_config('app.invite_token_digest', $1, true)`, staffauth.TokenDigest(token),
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return PreAccountOffer{}, http.StatusInternalServerError, staffauth.MsgInternalError
	}

	var o PreAccountOffer
	var dueDate, expiresAt time.Time
	var amountCents sql.NullInt64
	var terms, codeDigest sql.NullString
	var attempts int
	err := tx.QueryRowContext(ctx,
		`SELECT id, state::text, client_first_initial, client_area, due_date, amount_cents, terms,
		        expires_at, access_code_digest, access_code_attempts
		   FROM engagement_offers WHERE id = $1`,
		offerID,
	).Scan(&o.OfferID, &o.State, &o.ClientFirstInitial, &o.ClientArea, &dueDate, &amountCents, &terms,
		&expiresAt, &codeDigest, &attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return PreAccountOffer{}, http.StatusNotFound, "offer not found"
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return PreAccountOffer{}, http.StatusInternalServerError, staffauth.MsgInternalError
	}

	if attempts >= maxAccessCodeAttempts {
		return PreAccountOffer{}, http.StatusTooManyRequests, "too many incorrect codes -- ask the practice to send this offer again"
	}
	if subtle.ConstantTimeCompare([]byte(codeDigest.String), []byte(staffauth.TokenDigest(code))) != 1 {
		if _, err := tx.ExecContext(ctx,
			`UPDATE engagement_offers SET access_code_attempts = access_code_attempts + 1 WHERE id = $1`, o.OfferID,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return PreAccountOffer{}, http.StatusInternalServerError, staffauth.MsgInternalError
		}
		return PreAccountOffer{}, http.StatusForbidden, "that code is not right"
	}

	if err := expireOpen(ctx, tx, byID, o.OfferID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return PreAccountOffer{}, http.StatusInternalServerError, staffauth.MsgInternalError
	}
	if o.State == stateOffered && !expiresAt.After(time.Now()) {
		o.State = "expired"
	}

	o.DueDate = dueDate.Format(time.DateOnly)
	o.ExpiresAt = expiresAt.UTC().Format(time.RFC3339)
	o.AmountCents = nullableInt64(amountCents)
	o.Terms = nullableString(terms)
	return o, http.StatusOK, ""
}
