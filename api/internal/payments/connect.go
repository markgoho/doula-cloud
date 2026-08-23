package payments

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"doula-cloud/api/internal/staffauth"
)

// ConnectStatus is the single status GetConnectStatusHandler reports for
// a Practice's Stripe Connect account, and the Payments settings screen
// shows.
//
// #79 defined three -- not connected / onboarding incomplete / active -- against v1's
// three booleans. Accounts v2 reports two four-valued capability statuses
// instead (#247), which the three cannot express, so two are added:
//
//   - StatusPending is Stripe reviewing what the Owner has already
//     supplied. It is not "onboarding incomplete" (there is nothing left
//     to fill in) and not "active" (no Client can be charged yet), which
//     is exactly the case a boolean could not represent.
//   - StatusPayoutsRestricted is card_payments active while payouts is
//     not: Clients can pay, and the money is stuck in Stripe rather than
//     reaching the Practice's bank. Under v1 this collapsed into
//     "onboarding incomplete" and read as if invoicing were broken, which
//     it is not.
type ConnectStatus string

// The five statuses ConnectStatus takes.
const (
	StatusNotConnected         ConnectStatus = "not_connected"
	StatusOnboardingIncomplete ConnectStatus = "onboarding_incomplete"
	StatusPending              ConnectStatus = "pending"
	StatusPayoutsRestricted    ConnectStatus = "payouts_restricted"
	StatusActive               ConnectStatus = "active"
)

// ConnectStatus projects the account onto the single status the
// Payments settings screen shows. card_payments leads, because being payable at all
// is the thing the Practice is here for; payouts only refines an otherwise
// active account.
//
// Outstanding requirements outrank a pending capability, and the order
// matters: the two capabilities move independently, so a real account can
// report card_payments restricted while payouts is pending. Reporting that
// as StatusPending would hide the onboarding button -- the screen would
// list what Stripe is waiting on and offer no way to supply it. If the
// Owner has something to give Stripe, the status has to say so.
func (status AccountStatus) ConnectStatus() ConnectStatus {
	switch {
	case status.CardPayments == CapabilityActive && status.Payouts == CapabilityActive:
		return StatusActive
	case status.CardPayments == CapabilityActive:
		return StatusPayoutsRestricted
	case len(status.RequirementsDue) > 0:
		return StatusOnboardingIncomplete
	case status.CardPayments == CapabilityPending || status.Payouts == CapabilityPending:
		return StatusPending
	default:
		return StatusOnboardingIncomplete
	}
}

// ConnectResponse is the body of PostConnectHandler's response: the
// Stripe-hosted Account Link onboarding page the caller's browser should
// be sent to.
type ConnectResponse struct {
	OnboardingURL string `json:"onboardingUrl"`
}

// ConnectStatusResponse is the body of GetConnectStatusHandler's response:
// a Practice's current Stripe Connect status, read live from Stripe.
type ConnectStatusResponse struct {
	Status ConnectStatus `json:"status"`
	// The two raw capability statuses behind Status, so the screen can say
	// which half of the account is held up rather than only that something
	// is. Both are "unsupported" when the Practice has no Connect account
	// at all.
	CardPaymentsStatus CapabilityStatus `json:"cardPaymentsStatus"`
	PayoutsStatus      CapabilityStatus `json:"payoutsStatus"`
	// The Stripe field paths still awaiting the Owner. Never null -- an
	// empty list rather than a missing one, so the client needs no
	// null-check.
	RequirementsDue []string `json:"requirementsDue"`
}

// PostConnectHandler lets a Practice Owner start (or resume) Stripe
// Connect merchant onboarding: it lazily creates the Practice's Connect
// account on its first attempt and returns an Account Link URL to
// onboard, reusing the stored account id on any later attempt so
// onboarding can be resumed instead of starting over. Must be mounted
// behind staffauth.Middleware.
func PostConnectHandler(client Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireOwner(w, r)
		if !ok {
			return
		}

		// Locks the Practice row for the rest of this transaction so two
		// concurrent connect-initiation requests can't both see a null
		// stripe_connect_account_id and both create a Stripe account --
		// the same race-prevention shape as billing.PostPurchaseHandler's
		// customer lock.
		if _, err := tx.ExecContext(r.Context(), `SELECT id FROM practices WHERE id = $1 FOR UPDATE`, practiceID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		var accountID sql.NullString
		if err := tx.QueryRowContext(r.Context(),
			`SELECT stripe_connect_account_id FROM practices WHERE id = $1`, practiceID,
		).Scan(&accountID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if !accountID.Valid {
			id, err := client.CreateAccount(r.Context(), practiceID)
			if err != nil {
				http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
				return
			}
			if _, err := tx.ExecContext(r.Context(),
				`UPDATE practices SET stripe_connect_account_id = $1 WHERE id = $2`, id, practiceID,
			); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
				return
			}
			accountID = sql.NullString{String: id, Valid: true}
		}

		onboardingURL, err := client.CreateAccountLink(r.Context(), accountID.String, practiceID)
		if err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(ConnectResponse{OnboardingURL: onboardingURL}); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// GetConnectStatusHandler lets any Staff member at the current Practice
// read its Stripe Connect status -- no Owner-only restriction, consistent
// with billing.GetBalanceHandler's practice-wide visibility default. A
// Practice with no stored account id is reported not_connected without any
// Stripe call; otherwise status is read live via an on-demand Account
// retrieve (#79's ticket body: this is deliberately not backed by the
// webhook-synced columns on practices). Must be mounted behind
// staffauth.Middleware.
func GetConnectStatusHandler(client Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}

		var accountID sql.NullString
		if err := tx.QueryRowContext(r.Context(),
			`SELECT stripe_connect_account_id FROM practices WHERE id = $1`, practiceID,
		).Scan(&accountID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		out := ConnectStatusResponse{
			Status:             StatusNotConnected,
			CardPaymentsStatus: CapabilityUnsupported,
			PayoutsStatus:      CapabilityUnsupported,
			RequirementsDue:    []string{},
		}
		if accountID.Valid {
			status, err := client.RetrieveAccount(r.Context(), accountID.String)
			if err != nil {
				http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
				return
			}
			out.CardPaymentsStatus = status.CardPayments
			out.PayoutsStatus = status.Payouts
			out.RequirementsDue = requirementsOrEmpty(status.RequirementsDue)
			out.Status = status.ConnectStatus()
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(out); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}
