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
	OfferID            string `json:"offerId"`
	State              string `json:"state"`
	ClientFirstInitial string `json:"clientFirstInitial"`
	ClientArea         string `json:"clientArea"`
	DueDate            string `json:"dueDate"`
	// DueMonth (#2383) is the due date's year and month, "2027-02", on an
	// accepted Offer read from her own inbox only: the least identifying
	// fact that tells one birth she worked from another after #230 has
	// lapsed the rest. The Practice-side read never carries it.
	DueMonth       string  `json:"dueMonth,omitempty"`
	AmountCents    *int64  `json:"amountCents,omitempty"`
	Terms          *string `json:"terms,omitempty"`
	EmploymentType string  `json:"employmentType"`
	OfferedAt      string  `json:"offeredAt"`
	ExpiresAt      string  `json:"expiresAt"`
	DecidedAt      *string `json:"decidedAt,omitempty"`
	// TargetName and TargetAddress name who the Offer went to. Both are
	// empty on the inbox read -- she knows who she is -- and filled on
	// the Practice-side read, which is Owner/Admin only.
	TargetName    string `json:"targetName,omitempty"`
	TargetAddress string `json:"targetAddress,omitempty"`
	// FeePaid (#2271) is the Fee Payment that stands against the fee this
	// accepted Offer put on her Attachment, open or ended. Only her own
	// inbox carries it, and it is absent until one is recorded.
	FeePaid *FeePaid `json:"feePaid,omitempty"`
}

// FeePaid is what she reads of a Fee Payment: how much, how, and the day
// the Practice says it paid. The recorder's note is the Practice's own.
type FeePaid struct {
	AmountCents int64  `json:"amountCents"`
	Method      string `json:"method"`
	PaidOn      string `json:"paidOn"`
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
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		after, ok := parseCursor(w, r)
		if !ok {
			return
		}
		page, err := list(r.Context(), tx,
			`SELECT o.id, o.state::text, o.client_first_initial, o.client_area, o.due_date,
			        o.amount_cents, o.terms, o.employment_type::text, o.offered_at, o.expires_at,
			        o.decided_at, '', '', fp.amount_cents, fp.method, fp.paid_on::text,
			        CASE WHEN o.state = 'accepted' THEN to_char(o.due_date, 'YYYY-MM') END
			   FROM engagement_offers o
			   -- The Attachment this Offer's acceptance granted: the latest
			   -- one with a fee on the pair, opened no later than the
			   -- acceptance (staffauth.Grant writes both in one transaction).
			   LEFT JOIN LATERAL (
			        SELECT ea.id FROM engagement_attachments ea
			         WHERE o.state = 'accepted'
			           AND ea.engagement_id = o.engagement_id AND ea.staff_id = o.staff_id
			           AND ea.origin = 'granted' AND ea.fee_amount_cents IS NOT NULL
			           AND ea.attached_at <= o.decided_at
			         ORDER BY ea.attached_at DESC
			         LIMIT 1
			   ) att ON true
			   LEFT JOIN LATERAL (
			        SELECT f.amount_cents, f.method::text AS method, f.paid_on
			          FROM attachment_fee_payments f
			         WHERE f.attachment_id = att.id AND f.kind = 'recorded'
			           AND NOT EXISTS (SELECT 1 FROM attachment_fee_payments rv WHERE rv.target_id = f.id)
			         ORDER BY f.recorded_at DESC
			         LIMIT 1
			   ) fp ON true
			  WHERE o.staff_id = $1
			    AND ($2::timestamptz IS NULL OR (o.offered_at, o.id) < ($2::timestamptz, $3::uuid))
			  ORDER BY o.offered_at DESC, o.id DESC
			  LIMIT $4`,
			staffID, after)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, page)
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
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
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
			        o.decided_at, COALESCE(s.name, ''), COALESCE(s.email, pi.address, ''),
			        NULL::bigint, NULL::text, NULL::text, NULL::text
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
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, page)
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
// rows out of it. Both select the same seventeen columns in the same order
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
		var terms, paidMethod, paidOn, dueMonth sql.NullString
		var paidCents sql.NullInt64
		if err := rows.Scan(&s.OfferID, &s.State, &s.ClientFirstInitial, &s.ClientArea, &dueDate,
			&amountCents, &terms, &s.EmploymentType, &offeredAt, &expiresAt, &decidedAt,
			&s.TargetName, &s.TargetAddress, &paidCents, &paidMethod, &paidOn, &dueMonth); err != nil {
			// coverage:ignore reason: DB scan failure, not exercised by unit tests
			return ListResponse{}, fmt.Errorf("offer: scan offer row: %w", err)
		}
		s.DueDate = dueDate.Format(time.DateOnly)
		s.OfferedAt = offeredAt.UTC().Format(time.RFC3339)
		s.ExpiresAt = expiresAt.UTC().Format(time.RFC3339)
		s.AmountCents = nullableInt64(amountCents)
		s.Terms = nullableString(terms)
		if paidCents.Valid {
			s.FeePaid = &FeePaid{AmountCents: paidCents.Int64, Method: paidMethod.String, PaidOn: paidOn.String}
		}
		if decidedAt.Valid {
			decided := decidedAt.Time.UTC().Format(time.RFC3339)
			s.DecidedAt = &decided
		}
		// #230: a terminal Offer keeps the fact of the asking and loses
		// the Client's own fields, so a stale row in her history is not a
		// stranger's due date sitting in it indefinitely. #2383: an
		// accepted Offer on her inbox keeps DueMonth.
		lapseClientFields(s.State, &s.ClientFirstInitial, &s.ClientArea, &s.DueDate)
		s.DueMonth = dueMonth.String
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
