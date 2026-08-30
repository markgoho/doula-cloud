package billing

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"doula-cloud/api/internal/staffauth"
)

// RefundRequest is the body of a POST to RefundHandler: which Practice is
// being refunded, and how many Credits she asked back.
type RefundRequest struct {
	PracticeID string `json:"practiceId"`
	Quantity   int    `json:"quantity"`
}

// authorizeInternal is the X-Internal-Secret check the two endpoints
// below share -- the same guard registerInternalRoutes puts in front of
// every worker endpoint, because these are the same kind of thing: no
// session, no Practice context of their own, authenticated by a secret
// only the operator holds.
func authorizeInternal(w http.ResponseWriter, r *http.Request, secret string) bool {
	if secret == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Internal-Secret")), []byte(secret)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

// RefundHandler issues a refund a Practice has asked for.
//
// It is deliberately not a screen a Practice can press. /support says "To
// ask for a refund, email us", and that is a legal position, not a
// missing feature: a refund the platform initiates itself inherits the
// original balance's dormancy date, while one issued on the Practice's
// recorded request restarts the clock (APL 1315). The request is the
// email; this endpoint is how it is honoured.
//
// The refusal rules live in Refund, not here, so they hold however the
// operation is reached.
func RefundHandler(db *sql.DB, client StripeClient, secret string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !authorizeInternal(w, r, secret) {
			return
		}

		var req RefundRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if !staffauth.ParseUUID(w, "practice", req.PracticeID) {
			return
		}
		if req.Quantity < 1 {
			http.Error(w, "quantity must be at least 1", http.StatusBadRequest)
			return
		}
		// The name of this request, and the reason a retry is safe: the
		// same key returns the refund already issued instead of issuing
		// a second one. Required, not optional -- a refund moves money,
		// and an unnamed request cannot be recognised on its way back
		// through. Same header docs/api-design.md sets for every other
		// repeatable write.
		requestKey := r.Header.Get("Idempotency-Key")
		if requestKey == "" {
			http.Error(w, "Idempotency-Key header is required", http.StatusBadRequest)
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

		// No staffauth.Middleware in front of this endpoint, so nothing
		// else sets the variable credit_ledger's policy reads -- the
		// secret checked above is what licenses setting it, exactly as
		// the Stripe signature does in PostPurchaseWebhookHandler.
		if _, err := tx.ExecContext(r.Context(),
			`SELECT set_config('app.current_practice_id', $1, true)`, req.PracticeID,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		receipt, err := Refund(r.Context(), tx, client, req.PracticeID, requestKey, req.Quantity, time.Now())
		if errors.Is(err, ErrNothingRefundable) || errors.Is(err, ErrRefundExceedsLot) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		if err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed = true

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(receipt); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// DormantPracticesHandler lists the Practices holding Credits nobody has
// touched for DormancyNoticeYears -- the list the annual balance notice
// and the December/January due-diligence mailings are worked from. Read
// only: it identifies balances, and nothing anywhere writes one off.
func DormantPracticesHandler(db *sql.DB, secret string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !authorizeInternal(w, r, secret) {
			return
		}

		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			// coverage:ignore reason: DB connection failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		defer func() { _ = tx.Rollback() }()

		dormant, err := DormantPractices(r.Context(), tx, time.Now().AddDate(-DormancyNoticeYears, 0, 0))
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(dormant); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// FoundingGrantRequest is the body of a POST to FoundingGrantHandler:
// which Practice is joining the pilot, and who is issuing its Credits.
type FoundingGrantRequest struct {
	PracticeID string `json:"practiceId"`
	GrantedBy  string `json:"grantedBy"`
}

// FoundingGrantHandler issues a Practice's founding grant (#449).
//
// It is an operator endpoint on the same X-Internal-Secret guard as the
// refund, and for the same reason: at roughly fifty doulas across a
// handful of Practices, issuing grants by hand is right and an admin
// screen is not worth building. "By hand" still has to leave an audit
// record, so this exists instead of an ad-hoc INSERT -- it names the
// grantor, sizes the grant from the roster rather than from whatever the
// caller typed, and refuses a second grant to a Practice that already has
// one.
func FoundingGrantHandler(db *sql.DB, secret string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !authorizeInternal(w, r, secret) {
			return
		}

		var req FoundingGrantRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if !staffauth.ParseUUID(w, "practice", req.PracticeID) {
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

		// Same reason RefundHandler sets it: no staffauth.Middleware in
		// front of this endpoint, so nothing else sets the variable
		// credit_ledger's and practice_memberships' policies read.
		if _, err := tx.ExecContext(r.Context(),
			`SELECT set_config('app.current_practice_id', $1, true)`, req.PracticeID,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		receipt, err := FoundingGrant(r.Context(), tx, req.PracticeID, req.GrantedBy)
		if errors.Is(err, ErrNoGrantor) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if errors.Is(err, ErrAlreadyGranted) || errors.Is(err, ErrNoStaff) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB failure inside FoundingGrant, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed = true

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(receipt); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}
