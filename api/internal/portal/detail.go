// Package portal holds the Client-portal-side BFF handlers. In v1 that's
// just viewing an Engagement's basic detail -- the landing point the
// SvelteKit portal login flow redirects a Client to. Relies on
// clientauth.Middleware having already resolved the caller's
// Client/Engagement ids and opened a request-scoped *sql.Tx with
// app.current_client_id set.
package portal

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/engagement"
)

// Detail is an Engagement's basic detail as seen from the Client portal:
// the Practice it's at, its status, and when it is due.
type Detail struct {
	EngagementID string `json:"engagementId"`
	PracticeName string `json:"practiceName"`
	// ClientName is the signed-in Client's own name, for the portal
	// shell's avatar (#452). The portal bar is named after the Practice,
	// so without this the one thing on the bar that says "this is your
	// account, not somebody else's" has nothing to render.
	//
	// A display name rather than a column: ADR-0017 replaced `name` with
	// given/family/preferred, and `preferred_name` wins wherever a Client
	// reads her own screen -- it is the whole reason the column exists.
	// family_name is optional, so the two are joined with concat_ws rather
	// than `||`, which would null the entire expression.
	ClientName string `json:"clientName"`
	Status     string `json:"status"`
	// DueDate is ADR-0017's `engagements.due_date`, nullable because a
	// postpartum-only Engagement has none (#505).
	DueDate *string `json:"dueDate,omitempty"`
	// CreatedAt is when the Engagement itself began (#310): the portal's
	// authenticated chrome names the Engagement she is in by it, the one
	// fact that tells two Engagements at this same Practice apart and
	// stays true for her whatever became of the pregnancy. It was dropped
	// from this DTO once before (#505, when the portal page had no use
	// for it) and is back now that the chrome does.
	CreatedAt time.Time `json:"createdAt"`
	// OffersBirthPlan is engagement.OffersBirthPlan's own resolved answer
	// (#311), not the Engagement's kind -- kind is staff-only
	// (CONTEXT.md's Engagement entry) and must never reach a Client-facing
	// response. Every portal surface that offers, links to or announces a
	// Birth Plan reads this one field, so they cannot drift apart.
	OffersBirthPlan bool `json:"offersBirthPlan"`
}

// DetailHandler views the caller's Engagement's basic detail. Must be
// mounted behind clientauth.Middleware.
func DetailHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, has := clientauth.Tx(r.Context())
		if !has {
			// coverage:ignore reason: clientauth.Middleware always sets a tx before this handler runs
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		clientID, _ := clientauth.ClientID(r.Context())
		engagementID, _ := clientauth.EngagementID(r.Context())

		var d Detail
		var dueDate sql.NullString
		var kind string
		err := tx.QueryRowContext(r.Context(),
			`SELECT e.id, p.name,
			        trim(concat_ws(' ', coalesce(c.preferred_name, c.given_name), c.family_name)),
			        e.status, e.due_date::text, e.created_at, e.kind::text
			 FROM engagements e
			 JOIN practices p ON p.id = e.practice_id
			 JOIN clients c ON c.id = e.client_id
			 WHERE e.id = $1 AND e.client_id = $2`,
			engagementID, clientID,
		).Scan(&d.EngagementID, &d.PracticeName, &d.ClientName, &d.Status, &dueDate, &d.CreatedAt, &kind)
		if errors.Is(err, sql.ErrNoRows) {
			// coverage:ignore reason: clientauth.Middleware already confirmed ownership; unreachable in practice
			apierr.WriteError(w, "engagement not found", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if dueDate.Valid {
			d.DueDate = &dueDate.String
		}
		d.OffersBirthPlan = engagement.OffersBirthPlan(engagement.BirthPlanInputs{Kind: engagement.Kind(kind)})

		apierr.WriteJSON(w, http.StatusOK, d)
	})
}
