package payments

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"doula-cloud/api/internal/staffauth"
)

// The three connection statuses GetConnectStatusHandler can report, per
// #79's ticket body: not connected / onboarding incomplete / active.
const (
	StatusNotConnected         = "not_connected"
	StatusOnboardingIncomplete = "onboarding_incomplete"
	StatusActive               = "active"
)

// ConnectResponse is the body of PostConnectHandler's response: the
// Stripe-hosted Account Link onboarding page the caller's browser should
// be sent to.
type ConnectResponse struct {
	OnboardingURL string `json:"onboardingUrl"`
}

// ConnectStatusResponse is the body of GetConnectStatusHandler's response:
// a Practice's current Stripe Connect status, read live from Stripe.
type ConnectStatusResponse struct {
	Status           string `json:"status"`
	ChargesEnabled   bool   `json:"chargesEnabled"`
	PayoutsEnabled   bool   `json:"payoutsEnabled"`
	DetailsSubmitted bool   `json:"detailsSubmitted"`
}

// PostConnectHandler lets a Practice Owner start (or resume) Stripe
// Connect Standard onboarding: it lazily creates the Practice's Connect
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
// webhook-synced booleans on practices). Must be mounted behind
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

		var out ConnectStatusResponse
		if !accountID.Valid {
			out.Status = StatusNotConnected
		} else {
			status, err := client.RetrieveAccount(r.Context(), accountID.String)
			if err != nil {
				http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
				return
			}
			out.ChargesEnabled = status.ChargesEnabled
			out.PayoutsEnabled = status.PayoutsEnabled
			out.DetailsSubmitted = status.DetailsSubmitted
			if status.ChargesEnabled && status.PayoutsEnabled && status.DetailsSubmitted {
				out.Status = StatusActive
			} else {
				out.Status = StatusOnboardingIncomplete
			}
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(out); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}
