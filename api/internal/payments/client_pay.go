package payments

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/ratelimit"
)

// Refusals a Client reads when an Invoice cannot be paid in the portal
// (#1020). They state the product's own record and nothing about anyone's
// conduct (#982), and they are written for her, not for a Staff member.
const (
	// msgClientInvoiceByHand is the refusal for a by-hand Invoice: there is
	// no Stripe object behind it, so there is nothing to pay in the browser
	// by construction (#980). It is the same sentence the Invoice page
	// shows in place of a pay control (#983).
	msgClientInvoiceByHand = "Your Practice collects this Invoice directly."
	// msgClientInvoiceNotOpen is the refusal for an Invoice that is paid,
	// void or written off. Nothing is waiting on her.
	msgClientInvoiceNotOpen = "This Invoice is not waiting for a payment."
	// msgClientCannotPayYet is the refusal when the Practice's Stripe
	// account is not connected or cannot take a card, which the Practice
	// can fix and she cannot.
	msgClientCannotPayYet = "Your Practice cannot take this payment yet."
	// msgClientPaymentUnavailable is the answer when Stripe itself could not
	// be reached or would not give the Invoice a payment. She can try again.
	msgClientPaymentUnavailable = "We could not start this payment. Please try again."
)

// clientPaymentRules limits the pay-secret endpoint as a heavy one
// (docs/api-design.md section 6): every call reaches Stripe live. The
// session rule bounds one Client, the Invoice rule bounds hammering one
// Invoice from any number of sessions, and the IP rule bounds volume
// across many Invoices. Sized for a Client opening the Invoice page a few
// times while she finds her card, well below a script's rate.
var clientPaymentRules = []ratelimit.Rule{
	ratelimit.SessionCookieRule(30, time.Hour),
	ratelimit.PathValueRule("invoiceId", 30, time.Hour),
	ratelimit.IPRule(100, time.Hour),
}

// ClientPaymentResponse is what the browser needs to mount the Payment
// Element on one Invoice (#1020) and nothing more. StripeAccountID is the
// Practice's connected account, resolved here and never taken from the
// request: the Element is constructed with it as `stripeAccount`, and it
// is an identifier, not a credential. PublishableKey is the platform's
// public key, which is meant to reach a browser. No secret key, and no
// other account credential, crosses.
type ClientPaymentResponse struct {
	ClientSecret    string `json:"clientSecret"`
	StripeAccountID string `json:"stripeAccountId"`
	PublishableKey  string `json:"publishableKey"`
}

// payableInvoiceQuery resolves the one Invoice she asked for, and the
// Practice's connected account behind it, in a single read through her own
// Client-portal transaction: an Invoice outside her Engagement, and a
// draft one, are zero rows.
const payableInvoiceQuery = `SELECT i.status, i.stripe_invoice_id, p.stripe_connect_account_id, p.stripe_connect_card_payments_status
	FROM invoices i
	JOIN contracts c ON c.id = i.contract_id
	JOIN engagements e ON e.id = c.engagement_id
	JOIN practices p ON p.id = e.practice_id
	WHERE c.engagement_id = $1 AND i.id = $2 AND i.status <> 'draft'`

// ClientGetPaymentHandler returns the client secret, connected account and
// publishable key the portal's Payment Element mounts with, for one of her
// own open, Stripe-rail Invoices (#1020). It is a live call to Stripe on
// every use and the secret is a credential for one payment, so it is a
// separate route from the Invoice read (#1011) and is never cached.
//
// Refused: an Invoice that is not hers or not there (404, the same words
// as the read), a by-hand Invoice, an Invoice that is not open, and a
// Practice that cannot take a card yet, or a deployment with no publishable
// key (each 409). publishableKey is the
// platform's public key from configuration.
//
// Must be mounted behind clientauth.Middleware.
func ClientGetPaymentHandler(client Client, publishableKey string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, has := clientauth.Tx(r.Context())
		if !has {
			// coverage:ignore reason: clientauth.Middleware always sets a tx before this handler runs
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		engagementID, _ := clientauth.EngagementID(r.Context())
		invoiceID := r.PathValue("invoiceId")
		if _, err := uuid.Parse(invoiceID); err != nil {
			apierr.WriteError(w, msgClientInvoiceNotFound, http.StatusNotFound)
			return
		}

		var status string
		var stripeInvoiceID, accountID, cardStatus sql.NullString
		err := tx.QueryRowContext(r.Context(), payableInvoiceQuery, engagementID, invoiceID).
			Scan(&status, &stripeInvoiceID, &accountID, &cardStatus)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, msgClientInvoiceNotFound, http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		switch {
		case !stripeInvoiceID.Valid:
			apierr.Write(w, http.StatusConflict, apierr.CodeFailedPrecondition, msgClientInvoiceByHand, nil)
			return
		case status != invoiceStatusOpen:
			apierr.Write(w, http.StatusConflict, apierr.CodeFailedPrecondition, msgClientInvoiceNotOpen, nil)
			return
		// A blank publishable key is a deployment that has not been given
		// one: the Element cannot be constructed without it, and no blank
		// key is handed to a browser to find that out.
		case publishableKey == "" || !accountID.Valid || cardStatus.String != string(CapabilityActive):
			apierr.Write(w, http.StatusConflict, apierr.CodeFailedPrecondition, msgClientCannotPayYet, nil)
			return
		}

		secret, err := client.RetrieveInvoiceClientSecret(r.Context(), accountID.String, stripeInvoiceID.String)
		if err != nil {
			log.Printf("payments: client secret retrieve failed: %v", err)
			apierr.WriteError(w, msgClientPaymentUnavailable, http.StatusBadGateway)
			return
		}

		// A credential: no caller and no intermediary may keep it.
		w.Header().Set("Cache-Control", "no-store")
		apierr.WriteJSON(w, http.StatusOK, ClientPaymentResponse{
			ClientSecret:    secret,
			StripeAccountID: accountID.String,
			PublishableKey:  publishableKey,
		})
	})
}
