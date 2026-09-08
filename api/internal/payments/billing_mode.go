package payments

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// BillingMode is a Practice's choice of billing rail (#271): Stripe-hosted
// Invoicing, or an Invoice the Practice raises and collects by hand.
// Never inferred from an absent Stripe Connect account -- a Practice that
// has never connected Stripe and a Practice that has deliberately chosen
// to bill by hand are different facts, and the grilling session that
// scoped this ticket rejected assuming one from the other.
type BillingMode string

// The two values BillingMode takes.
const (
	BillingModeStripe BillingMode = "stripe"
	BillingModeByHand BillingMode = "by_hand"
)

// actionBillingModeChanged records a change to practices.billing_mode --
// unexported, plain-string, Practice-scoped the same way
// staffauth.mfarequired.go's own action constant is: SubjectPractice
// carries no per-package action vocabulary of its own the way
// SubjectEngagement's EngagementAction does.
const actionBillingModeChanged = "billing_mode_changed"

// errBillingModeRequired is resolveBillingMode's refusal when a Practice
// has never chosen a rail and the request raising its first Invoice named
// none either.
var errBillingModeRequired = errors.New("payments: billing mode required")

// errBillingModeInvalid is resolveBillingMode's refusal when a supplied
// billing mode is neither "stripe" nor "by_hand".
var errBillingModeInvalid = errors.New("payments: billing mode invalid")

// MsgBillingModeRequired is PostInvoiceHandler's refusal when a Practice's
// billing_mode has never been set and the request did not supply one --
// the inline "ask once" #271 asks for, surfaced as a refusal rather than
// silently defaulting to a rail nobody chose.
const MsgBillingModeRequired = "This Practice has not chosen how it bills Clients yet. Choose Stripe or by-hand billing to raise this Invoice."

// fetchBillingMode reads practiceID's current billing_mode, unset (ok=false)
// when the Practice has never chosen one.
func fetchBillingMode(ctx context.Context, tx *sql.Tx, practiceID string) (mode BillingMode, ok bool, err error) {
	var current sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT billing_mode FROM practices WHERE id = $1`, practiceID).Scan(&current); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", false, fmt.Errorf("payments: read billing mode: %w", err)
	}
	if !current.Valid {
		return "", false, nil
	}
	return BillingMode(current.String), true, nil
}

// resolveBillingMode returns practiceID's billing mode for the purpose of
// raising an Invoice. If a mode is already set, requested is ignored --
// #271: changing an established mode happens only afterward, from
// Payments settings, by an Owner. If none is set yet, requested must name
// a valid mode -- this is the one path any Staff member (not only an
// Owner) may set billing_mode for the first time, since #271 asks for the
// choice "inline, the first time any Staff raises an Invoice." The chosen
// mode is persisted and recorded to the Activity log before this returns.
func resolveBillingMode(ctx context.Context, tx *sql.Tx, practiceID string, requested *string, staffID string) (BillingMode, error) {
	current, ok, err := fetchBillingMode(ctx, tx, practiceID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", err
	}
	if ok {
		return current, nil
	}
	if requested == nil {
		return "", errBillingModeRequired
	}
	mode := BillingMode(*requested)
	if mode != BillingModeStripe && mode != BillingModeByHand {
		return "", errBillingModeInvalid
	}
	if err := setBillingMode(ctx, tx, practiceID, mode, staffID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", err
	}
	return mode, nil
}

// setBillingMode writes practiceID's billing_mode and records the change
// to the Activity log -- the one write path both resolveBillingMode's
// initial set and PutBillingModeHandler's later change go through, so the
// two can never record the fact differently.
func setBillingMode(ctx context.Context, tx *sql.Tx, practiceID string, mode BillingMode, staffID string) error {
	if _, err := tx.ExecContext(ctx, `UPDATE practices SET billing_mode = $2 WHERE id = $1`, practiceID, string(mode)); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("payments: set billing mode: %w", err)
	}
	diff, err := json.Marshal(map[string]string{"billingMode": string(mode)})
	if err != nil {
		// coverage:ignore reason: a map of one string always marshals cleanly, not exercised by unit tests
		return fmt.Errorf("payments: marshal billing mode diff: %w", err)
	}
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectPractice,
		SubjectID:   practiceID,
		Action:      actionBillingModeChanged,
		Diff:        diff,
		Actor:       activity.StaffActor(staffID),
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("payments: record billing mode change: %w", err)
	}
	return nil
}

// BillingModeView is what GetBillingModeHandler and PutBillingModeHandler
// both return -- the mode is null until a Practice has chosen one.
type BillingModeView struct {
	BillingMode *string `json:"billingMode,omitempty"`
}

// billingModeView builds a BillingModeView from a possibly-unset mode.
func billingModeView(mode BillingMode, ok bool) BillingModeView {
	if !ok {
		return BillingModeView{}
	}
	s := string(mode)
	return BillingModeView{BillingMode: &s}
}

// GetBillingModeHandler reads a Practice's billing_mode. Any Staff member
// may read it (#271, following #270's own reasoning for "whether the
// Practice can raise an Invoice at all": a Doula meets this fact on the
// Invoice section she may already see). Must be mounted behind
// staffauth.Middleware with staffauth.AnyStaff.
func GetBillingModeHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		mode, has, err := fetchBillingMode(r.Context(), tx, practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, billingModeView(mode, has))
	})
}

// PutBillingModeRequest is the body of a PUT to PutBillingModeHandler.
type PutBillingModeRequest struct {
	BillingMode string `json:"billingMode"`
}

// PutBillingModeHandler changes a Practice's already-established
// billing_mode -- Owner-only (#271: "changing the mode is the Owner's
// alone, by analogy to Connect onboarding"), unlike the first-ever set,
// which rides PostInvoiceHandler's own request and is open to whichever
// Staff member happens to raise the first Invoice. A full-replacement
// PUT, so it needs no Idempotency-Key (docs/api-design.md section 3).
func PutBillingModeHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireOwner(w, r)
		if !ok {
			return
		}
		var req PutBillingModeRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		mode := BillingMode(req.BillingMode)
		if mode != BillingModeStripe && mode != BillingModeByHand {
			apierr.WriteError(w, `billingMode must be "stripe" or "by_hand"`, http.StatusBadRequest)
			return
		}
		staffID, _ := staffauth.StaffID(r.Context())
		if err := setBillingMode(r.Context(), tx, practiceID, mode, staffID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		s := string(mode)
		apierr.WriteJSON(w, http.StatusOK, BillingModeView{BillingMode: &s})
	})
}
