// Package engagement holds the Staff-side BFF handlers for Engagement
// detail, its status transition (#253) and its birth outcome (#293).
// All handlers rely on staffauth.Middleware
// having already resolved the caller's Staff/Practice ids and opened a
// request-scoped *sql.Tx with app.current_practice_id set, the same way
// staffauth's own Owner-only handlers (invite, role assignment) do. The
// Client write surface and the Clients list moved to package client
// (#397); this package's own surface is now just the Engagement itself.
package engagement

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/staffauth"
)

// Detail is an Engagement's basic detail: the Client it's for, its
// status, when it was created, and when it's due -- the landing point
// later features (Visits, Plans, Contracts, Messages) attach to.
type Detail struct {
	EngagementID string    `json:"engagementId"`
	ClientID     string    `json:"clientId"`
	ClientName   string    `json:"clientName"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"createdAt"`
	// DueDate is ADR-0017's `engagements.due_date`, nullable because a
	// postpartum-only Engagement has none. Mirrors portal.Detail's own
	// field (#505) -- same nullable-column read, same omitted-when-null
	// shape (#538).
	DueDate *string `json:"dueDate,omitempty"`
	// BirthOutcome/PregnancyEndedOn (#293) are ADR-0015's staff-only
	// birth outcome and the date the pregnancy ended, both absent from
	// the JSON until recorded. They are on this Staff-side DTO and
	// deliberately not on portal.Detail: ADR-0015 binds them as never
	// Client-facing, and portal.Detail's own read is what enforces it.
	BirthOutcome     *string `json:"birthOutcome,omitempty"`
	PregnancyEndedOn *string `json:"pregnancyEndedOn,omitempty"`
	// StatusMoves (#253) is legalMoves(reader, Status) -- exactly the
	// target statuses this caller may move to from Status, per ADR-0015's
	// six-move table narrowed by its role table. TransitionHandler
	// answers the same call with the same function, so the Engagement hub
	// renders exactly the controls the write endpoint will accept without
	// copying the role table into Svelte.
	StatusMoves []string `json:"statusMoves"`

	// ClientPortalInviteStatus/ClientEmailSuppressed/ClientHasEmail
	// (#255) are the Client's portal-invite state, using
	// client.FetchPortalInviteState -- the same derivation the Clients
	// list's own PortalInviteStatus/EmailSuppressed carry, reinstated
	// here so the Engagement hub can state it as standing information
	// (never invited / pending / accepted, and whether the address can
	// even be invited) rather than only as feedback after a Send.
	ClientPortalInviteStatus *string `json:"clientPortalInviteStatus,omitempty"`
	ClientEmailSuppressed    bool    `json:"clientEmailSuppressed"`
	ClientHasEmail           bool    `json:"clientHasEmail"`

	// ClientsCanPay (#270) is payments.ClientsCanPay -- one column,
	// practices.stripe_connect_card_payments_status = 'active', no Stripe
	// round-trip. A standing fact about the Practice's own billing
	// readiness, not the Owner's Stripe Connect account state
	// (ADR-0008's separate "Stripe Connect state" row): the Invoice
	// section reads this to show the Create Invoice form or a Notice
	// naming what is missing, before a Staff member ever presses Create.
	ClientsCanPay bool `json:"clientsCanPay"`
}

// DetailHandler views one Engagement's basic detail: every Staff role
// except a contractor Doula without an open, granted attachment on it
// (ADR-0008). Must be mounted behind staffauth.Middleware.
func DetailHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}

		engagementID := r.PathValue("engagementId")
		if !staffauth.ParseUUID(w, "engagement", engagementID) {
			return
		}

		reader, has := staffauth.ReaderFrom(r.Context())
		if !has {
			// coverage:ignore reason: staffauth.Middleware always places a Reader on context before this handler runs
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		canAccess, err := reader.CanAccessEngagement(r.Context(), tx, engagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !canAccess {
			apierr.WriteError(w, "engagement not found", http.StatusNotFound)
			return
		}

		var d Detail
		var givenName string
		var preferredName sql.NullString
		var dueDate sql.NullString
		err = tx.QueryRowContext(r.Context(),
			`SELECT e.id, c.id, c.given_name, c.preferred_name, e.status, e.created_at, e.due_date::text,
			        e.birth_outcome::text, e.pregnancy_ended_on::text
			 FROM engagements e
			 JOIN clients c ON c.id = e.client_id
			 WHERE e.id = $1 AND e.practice_id = $2`,
			engagementID, practiceID,
		).Scan(&d.EngagementID, &d.ClientID, &givenName, &preferredName, &d.Status, &d.CreatedAt, &dueDate,
			&d.BirthOutcome, &d.PregnancyEndedOn)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "engagement not found", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		d.ClientName = client.PreferredName(givenName, preferredName.String)
		if dueDate.Valid {
			d.DueDate = &dueDate.String
		}
		d.StatusMoves = legalMoves(reader, d.Status)

		portalState, err := client.FetchPortalInviteState(r.Context(), tx, d.ClientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		d.ClientPortalInviteStatus = portalState.Status
		d.ClientEmailSuppressed = portalState.EmailSuppressed
		d.ClientHasEmail = portalState.HasEmail

		d.ClientsCanPay, err = payments.ClientsCanPay(r.Context(), tx, practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, d)
	})
}
