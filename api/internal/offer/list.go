package offer

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/staffauth"
)

// Summary is one Offer as either side reads it. The four decidable facts
// and the terms are the whole of what an offered-not-accepted Doula may
// read about the work (ADR-0008's fifth read column) -- there is
// deliberately no Client name, no Engagement id, and no Contract figure
// here, so this one DTO is safe to serve to her and informative enough
// for the Owner who sent it.
type Summary struct {
	OfferID            string  `json:"offerId"`
	State              string  `json:"state"`
	ClientFirstInitial string  `json:"clientFirstInitial"`
	ClientArea         string  `json:"clientArea"`
	DueDate            string  `json:"dueDate"`
	AmountCents        *int64  `json:"amountCents,omitempty"`
	Terms              *string `json:"terms,omitempty"`
	EmploymentType     string  `json:"employmentType"`
	OfferedAt          string  `json:"offeredAt"`
	ExpiresAt          string  `json:"expiresAt"`
	DecidedAt          *string `json:"decidedAt,omitempty"`
	// TargetName and TargetAddress name who the Offer went to. Both are
	// empty on the inbox read -- she knows who she is -- and filled on
	// the Practice-side read, which is Owner/Admin only.
	TargetName    string `json:"targetName,omitempty"`
	TargetAddress string `json:"targetAddress,omitempty"`
}

// ListResponse wraps a list of Offers.
type ListResponse struct {
	Offers []Summary `json:"offers"`
}

// InboxHandler serves a Staff member her own Offers, open and past --
// #230's "what a Doula reads about work she has been offered and not
// taken". Open to every Staff role at the mount: it is scoped to the
// caller's own staff_id in SQL, so there is nothing here a role
// declaration could usefully narrow.
func InboxHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, _, ok := staffauth.RequireTx(w, r)
		if !ok {
			// coverage:ignore reason: Middleware always sets a tx before this handler runs
			return
		}
		staffID, _ := staffauth.StaffID(r.Context())

		if err := expireOpen(r.Context(), tx, byStaffID, staffID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		offers, err := list(r.Context(), tx,
			`SELECT o.id, o.state::text, o.client_first_initial, o.client_area, o.due_date,
			        o.amount_cents, o.terms, o.employment_type::text, o.offered_at, o.expires_at,
			        o.decided_at, '', ''
			   FROM engagement_offers o
			  WHERE o.staff_id = $1
			  ORDER BY o.offered_at DESC`,
			staffID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		writeJSON(w, ListResponse{Offers: offers})
	})
}

// EngagementListHandler serves every Offer made on one Engagement, so the
// make-an-offer screen can show who has been asked and what each of them
// said. Owner and Admin only: it names the people offered, which is
// roster-shaped information ADR-0008's read table keeps from a Doula.
func EngagementListHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, _, ok := staffauth.RequireTx(w, r)
		if !ok {
			// coverage:ignore reason: Middleware always sets a tx before this handler runs
			return
		}

		engagementID := r.PathValue("engagementId")
		if !staffauth.ParseUUID(w, "engagement", engagementID) {
			return
		}
		if err := expireOpen(r.Context(), tx, byEngagementID, engagementID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		// The target's name comes from her staff row when she has one and
		// from the Invitation's address when she does not -- an Offer to
		// an email address names nobody until it is accepted.
		offers, err := list(r.Context(), tx,
			`SELECT o.id, o.state::text, o.client_first_initial, o.client_area, o.due_date,
			        o.amount_cents, o.terms, o.employment_type::text, o.offered_at, o.expires_at,
			        o.decided_at, COALESCE(s.name, ''), COALESCE(s.email, pi.address, '')
			   FROM engagement_offers o
			   LEFT JOIN staff s ON s.id = o.staff_id
			   LEFT JOIN practice_invitations pi ON pi.id = o.invitation_id
			  WHERE o.engagement_id = $1
			  ORDER BY o.offered_at DESC`,
			engagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		writeJSON(w, ListResponse{Offers: offers})
	})
}

// list runs one of the two queries above and scans it into Summary rows.
// Both select the same twelve columns in the same order, which is what
// lets one scanner serve both.
func list(ctx context.Context, tx *sql.Tx, query, arg string) ([]Summary, error) {
	rows, err := tx.QueryContext(ctx, query, arg)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("offer: query offers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	offers := []Summary{}
	for rows.Next() {
		var s Summary
		var dueDate, offeredAt, expiresAt time.Time
		var decidedAt sql.NullTime
		var amountCents sql.NullInt64
		var terms sql.NullString
		if err := rows.Scan(&s.OfferID, &s.State, &s.ClientFirstInitial, &s.ClientArea, &dueDate,
			&amountCents, &terms, &s.EmploymentType, &offeredAt, &expiresAt, &decidedAt,
			&s.TargetName, &s.TargetAddress); err != nil {
			// coverage:ignore reason: DB scan failure, not exercised by unit tests
			return nil, fmt.Errorf("offer: scan offer row: %w", err)
		}
		s.DueDate = dueDate.Format(time.DateOnly)
		s.OfferedAt = offeredAt.UTC().Format(time.RFC3339)
		s.ExpiresAt = expiresAt.UTC().Format(time.RFC3339)
		s.AmountCents = nullableInt64(amountCents)
		s.Terms = nullableString(terms)
		if decidedAt.Valid {
			decided := decidedAt.Time.UTC().Format(time.RFC3339)
			s.DecidedAt = &decided
		}
		offers = append(offers, s)
	}
	// coverage:ignore reason: DB row iteration failure, not exercised by unit tests
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("offer: iterate offer rows: %w", err)
	}
	return offers, nil
}
