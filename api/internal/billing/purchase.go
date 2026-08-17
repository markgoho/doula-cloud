package billing

import (
	"database/sql"
	"encoding/json"
	"net/http"

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

// PostPurchaseHandler lets a Practice Owner start a credit purchase: it
// lazily creates the Practice's Stripe Customer on its first purchase and
// returns a Checkout Session URL for quantity credits. The ledger itself
// is never credited here -- only PostPurchaseWebhookHandler does that,
// once Stripe confirms the payment actually succeeded. Must be mounted
// behind staffauth.Middleware.
func PostPurchaseHandler(stripeClient StripeClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireOwner(w, r)
		if !ok {
			return
		}

		var req PurchaseRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.Quantity < 1 {
			http.Error(w, "quantity must be at least 1", http.StatusBadRequest)
			return
		}

		// Locks the Practice row for the rest of this transaction so two
		// concurrent purchase-initiation requests can't both see a null
		// stripe_customer_id and both create a Stripe Customer -- the same
		// race-prevention shape as ConsumeCredit's practice lock.
		if _, err := tx.ExecContext(r.Context(), `SELECT id FROM practices WHERE id = $1 FOR UPDATE`, practiceID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		var customerID sql.NullString
		if err := tx.QueryRowContext(r.Context(),
			`SELECT stripe_customer_id FROM practices WHERE id = $1`, practiceID,
		).Scan(&customerID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if !customerID.Valid {
			id, err := stripeClient.CreateCustomer(r.Context(), practiceID)
			if err != nil {
				http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
				return
			}
			if _, err := tx.ExecContext(r.Context(),
				`UPDATE practices SET stripe_customer_id = $1 WHERE id = $2`, id, practiceID,
			); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
				return
			}
			customerID = sql.NullString{String: id, Valid: true}
		}

		checkoutURL, err := stripeClient.CreateCheckoutSession(r.Context(), customerID.String, practiceID, req.Quantity)
		if err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(PurchaseResponse{CheckoutURL: checkoutURL}); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}
