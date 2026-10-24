package staffauth

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
)

// The three acts after which the product recommends a second factor to
// an Owner who has none (#1533, ADR-0026's amendment for #1492). Each is
// a moment her account starts to reach more than it did. The names are
// the values second_factor_recommendation_closures.act admits.
const (
	recommendationActFirstValue      = "first_value"
	recommendationActFirstInvitation = "first_invitation"
	recommendationActStripeConnected = "stripe_connected"
)

// SecondFactorRecommendationResponse is GET
// .../second-factor-recommendation's body: one fact, whether the
// notice shows. It names no act and counts nothing, because the notice
// states one fact about her account and reads no use of hers (ADR-0046
// P5).
type SecondFactorRecommendationResponse struct {
	Recommended bool `json:"recommended"`
}

// reachedRecommendationActs returns the acts on the list this Owner has
// reached at this Practice. Before First Value it returns none, whatever
// else is true: the empty Practice asks for one act and nothing else
// (ADR-0048), so an Invitation or Stripe that came first shows its
// notice after First Value.
//
// First Value is an Engagement existing (ADR-0048). Her first
// Invitation is one she sent, whatever became of it. Stripe connected
// is a Stripe account on the Practice, the line between the Getting
// paid card's "Not connected" and every other state.
func reachedRecommendationActs(ctx context.Context, tx *sql.Tx, practiceID, staffID string) ([]string, error) {
	var firstValue, invited, connected bool
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM engagements WHERE practice_id = $1),
		        EXISTS (SELECT 1 FROM practice_invitations WHERE practice_id = $1 AND invited_by = $2),
		        (SELECT stripe_connect_account_id IS NOT NULL FROM practices WHERE id = $1)`,
		practiceID, staffID,
	).Scan(&firstValue, &invited, &connected); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("staffauth: read second-factor recommendation acts: %w", err)
	}
	if !firstValue {
		return nil, nil
	}
	acts := []string{recommendationActFirstValue}
	if invited {
		acts = append(acts, recommendationActFirstInvitation)
	}
	if connected {
		acts = append(acts, recommendationActStripeConnected)
	}
	return acts, nil
}

// isClosedFor reports whether this Owner has closed the notice for act.
func isClosedFor(ctx context.Context, tx *sql.Tx, practiceID, staffID, act string) (bool, error) {
	var closed bool
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM second_factor_recommendation_closures
		                 WHERE practice_id = $1 AND staff_id = $2 AND act = $3)`,
		practiceID, staffID, act,
	).Scan(&closed); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, fmt.Errorf("staffauth: read second-factor recommendation closure: %w", err)
	}
	return closed, nil
}

// GetSecondFactorRecommendationHandler answers whether the notice that
// recommends a second factor shows to the caller. It shows to an Owner
// whose session has no second factor, once First Value is reached, while
// some act she has reached is one she has not closed it for.
//
// The second-factor fact is the session's own (SecondFactor), the same
// one the five acts and the Practice boundary read: Identity Platform
// challenges an enrolled factor at every sign-in, so a session with
// none is a person with none.
//
// Owner-only is declared at the mount. An Admin or a Doula is refused
// there, and the app asks only for an Owner.
func GetSecondFactorRecommendationHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		if SecondFactor(r.Context()) {
			apierr.WriteJSON(w, http.StatusOK, SecondFactorRecommendationResponse{})
			return
		}
		staffID, _ := StaffID(r.Context())

		acts, err := reachedRecommendationActs(r.Context(), tx, practiceID, staffID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		recommended := false
		for _, act := range acts {
			closed, err := isClosedFor(r.Context(), tx, practiceID, staffID, act)
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
			if !closed {
				recommended = true
				break
			}
		}
		apierr.WriteJSON(w, http.StatusOK, SecondFactorRecommendationResponse{Recommended: recommended})
	})
}

// CloseSecondFactorRecommendationHandler records that the caller closed
// the notice, for every act on the list she has reached now. So the
// notice comes back only at an act she has not reached yet, once for
// each. A row per act says who closed it and when; a repeated close
// finds its rows already there and writes nothing.
func CloseSecondFactorRecommendationHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		staffID, _ := StaffID(r.Context())

		acts, err := reachedRecommendationActs(r.Context(), tx, practiceID, staffID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		now := clock.Now(r.Context())
		for _, act := range acts {
			if _, err := tx.ExecContext(r.Context(),
				`INSERT INTO second_factor_recommendation_closures (practice_id, staff_id, act, closed_at)
				 VALUES ($1, $2, $3, $4)
				 ON CONFLICT (practice_id, staff_id, act) DO NOTHING`,
				practiceID, staffID, act, now,
			); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
