package billing

import (
	"database/sql"
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/legal"
	"doula-cloud/api/internal/staffauth"
)

// PurchaseRequest is the body of a POST to PostPurchaseHandler: how many
// credits the Practice wants to buy. Pricing (how a user picks N) is a
// product decision left to the frontend -- this endpoint is a generic
// "buy N credits" mechanism, not a fixed set of packs/SKUs.
type PurchaseRequest struct {
	Quantity int `json:"quantity"`
}

// PurchaseResponse is the body of PostPurchaseHandler's response: the
// Stripe-hosted Checkout page the caller's browser should be sent to.
type PurchaseResponse struct {
	CheckoutURL string `json:"checkoutUrl"`
}

// PostPurchaseHandler lets a Practice Owner or Admin start a credit
// purchase -- ADR-0017's correction: an Admin who may approve an
// Engagement Request, and who already reads the balance and the ledger,
// must be able to top up the balance she approves against. It lazily
// creates the Practice's Stripe Customer on its first purchase and
// returns a Checkout Session URL for quantity credits. The ledger itself
// is never credited here -- only PostPurchaseWebhookHandler does that,
// once Stripe confirms the payment actually succeeded. Must be mounted
// behind staffauth.Middleware.
//
// Owner and Admin is declared at the mount, not checked here (#1016,
// following #970's and #990's own move): the handler no longer calls
// staffauth.RequireOwnerOrAdmin, because a Doula is refused by the gate
// before this runs. Widening or narrowing this route means editing its
// role list in mount.go.
//
// terms is the version history the price of a Credit is read from:
// legal.Terms in production, a fixture in a test.
func PostPurchaseHandler(stripeClient StripeClient, terms legal.Document) http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		var req PurchaseRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		if req.Quantity < 1 {
			apierr.WriteError(w, "quantity must be at least 1", http.StatusBadRequest)
			return
		}

		// Locks the Practice row for the rest of this transaction so two
		// concurrent purchase-initiation requests can't both see a null
		// stripe_customer_id and both create a Stripe Customer -- the same
		// race-prevention shape as ConsumeCredit's practice lock.
		if _, err := c.Tx.ExecContext(r.Context(), `SELECT id FROM practices WHERE id = $1 FOR UPDATE`, c.PracticeID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		var customerID sql.NullString
		if err := c.Tx.QueryRowContext(r.Context(),
			`SELECT stripe_customer_id FROM practices WHERE id = $1`, c.PracticeID,
		).Scan(&customerID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if !customerID.Valid {
			id, err := stripeClient.CreateCustomer(r.Context(), c.PracticeID)
			if err != nil {
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
			if _, err := c.Tx.ExecContext(r.Context(),
				`UPDATE practices SET stripe_customer_id = $1 WHERE id = $2`, id, c.PracticeID,
			); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
			customerID = sql.NullString{String: id, Valid: true}
		}

		// How the sale is apportioned for New York sales tax: the share
		// taxed as prewritten software is New York-located Staff over all
		// Staff (#389). Counted here, inside the request's transaction,
		// because row-level security is what scopes staff to this
		// Practice -- the Stripe client has no database.
		var newYorkStaff, totalStaff int
		if err := c.Tx.QueryRowContext(r.Context(),
			`SELECT count(*) FILTER (WHERE s.work_state = 'NY'), count(*)
			 FROM staff s
			 JOIN practice_memberships pm ON pm.staff_id = s.id
			 WHERE pm.practice_id = $1`, c.PracticeID,
		).Scan(&newYorkStaff, &totalStaff); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		// The price is the one the version of the Terms in force states,
		// read at this instant (#1834): a published rise starts on its
		// date, and nothing a Stripe Price or an environment variable
		// does can charge more than the Terms allow.
		pricing, ok := requireCreditPrice(w, r, c.Tx, terms)
		if !ok {
			return
		}

		checkoutURL, err := stripeClient.CreateCheckoutSession(r.Context(), CheckoutSessionRequest{
			CustomerID:      customerID.String,
			PracticeID:      c.PracticeID,
			Quantity:        req.Quantity,
			NewYorkStaff:    newYorkStaff,
			TotalStaff:      totalStaff,
			UnitAmountCents: pricing.today.UnitAmountCents,
			TermsVersion:    pricing.termsVersion,
		})
		if err != nil {
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, PurchaseResponse{CheckoutURL: checkoutURL})
	})
}
