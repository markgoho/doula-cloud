package offer

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
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

// ListResponse is the standard cursor-pagination envelope from
// docs/api-design.md section 4, mirroring payments.ListInvoicesResponse.
// An Offer inbox grows for as long as a Doula works at a Practice, so it
// is paginated for the reason that section gives, not because any
// current screen needs a second page.
type ListResponse struct {
	Items      []Summary `json:"items"`
	NextCursor *string   `json:"nextCursor,omitempty"`
	HasMore    bool      `json:"hasMore"`
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
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		after, ok := parseCursor(w, r)
		if !ok {
			return
		}
		page, err := list(r.Context(), tx,
			`SELECT o.id, o.state::text, o.client_first_initial, o.client_area, o.due_date,
			        o.amount_cents, o.terms, o.employment_type::text, o.offered_at, o.expires_at,
			        o.decided_at, '', ''
			   FROM engagement_offers o
			  WHERE o.staff_id = $1
			    AND ($2::timestamptz IS NULL OR (o.offered_at, o.id) < ($2::timestamptz, $3::uuid))
			  ORDER BY o.offered_at DESC, o.id DESC
			  LIMIT $4`,
			staffID, after)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		writeJSON(w, page)
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
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		// The target's name comes from her staff row when she has one and
		// from the Invitation's address when she does not -- an Offer to
		// an email address names nobody until it is accepted.
		after, ok := parseCursor(w, r)
		if !ok {
			return
		}
		page, err := list(r.Context(), tx,
			`SELECT o.id, o.state::text, o.client_first_initial, o.client_area, o.due_date,
			        o.amount_cents, o.terms, o.employment_type::text, o.offered_at, o.expires_at,
			        o.decided_at, COALESCE(s.name, ''), COALESCE(s.email, pi.address, '')
			   FROM engagement_offers o
			   LEFT JOIN staff s ON s.id = o.staff_id
			   LEFT JOIN practice_invitations pi ON pi.id = o.invitation_id
			  WHERE o.engagement_id = $1
			    AND ($2::timestamptz IS NULL OR (o.offered_at, o.id) < ($2::timestamptz, $3::uuid))
			  ORDER BY o.offered_at DESC, o.id DESC
			  LIMIT $4`,
			engagementID, after)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		writeJSON(w, page)
	})
}

// parseCursor reads the optional ?cursor= page position, writing its own
// 400 on a malformed one rather than silently serving the first page.
func parseCursor(w http.ResponseWriter, r *http.Request) (*cursor, bool) {
	raw := r.URL.Query().Get("cursor")
	if raw == "" {
		return nil, true
	}
	c, err := decodeCursor(raw)
	if err != nil {
		apierr.WriteError(w, "invalid cursor", http.StatusBadRequest)
		return nil, false
	}
	return &c, true
}

// list runs one of the two queries above and scans one page of Summary
// rows out of it. Both select the same twelve columns in the same order
// and take the same four parameters, which is what lets one scanner
// serve both. It asks for pageSize+1 rows and returns pageSize, so
// "is there another page" is answered without a second count query.
func list(ctx context.Context, tx *sql.Tx, query, arg string, after *cursor) (ListResponse, error) {
	var afterTime *time.Time
	var afterID *string
	if after != nil {
		afterTime = &after.offeredAt
		afterID = &after.offerID
	}
	rows, err := tx.QueryContext(ctx, query, arg, afterTime, afterID, pageSize+1)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return ListResponse{}, fmt.Errorf("offer: query offers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := []Summary{}
	// The cursor is keyed on offered_at, which Summary carries only as a
	// formatted string, so the raw timestamps ride alongside rather than
	// being parsed back out of the response.
	offeredAts := []time.Time{}
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
			return ListResponse{}, fmt.Errorf("offer: scan offer row: %w", err)
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
		// #230: a terminal Offer keeps the fact of the asking and loses
		// the Client's own fields, so a stale row in her history is not a
		// stranger's due date sitting in it indefinitely.
		lapseClientFields(s.State, &s.ClientFirstInitial, &s.ClientArea, &s.DueDate)
		items = append(items, s)
		offeredAts = append(offeredAts, offeredAt)
	}
	// coverage:ignore reason: DB row iteration failure, not exercised by unit tests
	if err := rows.Err(); err != nil {
		return ListResponse{}, fmt.Errorf("offer: iterate offer rows: %w", err)
	}

	page := ListResponse{Items: items, HasMore: len(items) > pageSize}
	if page.HasMore {
		page.Items = items[:pageSize]
		next := encodeCursor(offeredAts[pageSize-1], page.Items[pageSize-1].OfferID)
		page.NextCursor = &next
	}
	return page, nil
}
