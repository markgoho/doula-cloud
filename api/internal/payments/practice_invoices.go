package payments

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/overdue"
	"doula-cloud/api/internal/personname"
	"doula-cloud/api/internal/practicetimezone"
	"doula-cloud/api/internal/staffauth"
)

// PracticeInvoiceView is one row of the Practice-wide Invoice list. It is
// deliberately a separate type from InvoiceView rather than a widening of
// it: the per-Engagement list is read from inside an Engagement that
// already names its Client, so it carries neither the Client nor the
// Engagement. The Practice-wide list is read from nowhere in particular,
// so a row that only carried an amount and a status would not answer
// "who owes us money" -- ClientName says who, and EngagementID is the way
// in to her Contract.
//
// ClientName is personname.Full: a list that sets Clients side by side
// names each in full, so two Clients called Quinn read apart (the brief's
// Voice amendment of 2026-10-26 on #2111, #2114). The legal name belongs to the documents
// -- the Contract's merge field and the Stripe Invoice itself -- not to a
// staff-facing list.
type PracticeInvoiceView struct {
	ID           string    `json:"id"`
	EngagementID string    `json:"engagementId"`
	ContractID   string    `json:"contractId"`
	ClientName   string    `json:"clientName"`
	Status       string    `json:"status"`
	AmountCents  int64     `json:"amountCents"`
	Currency     string    `json:"currency"`
	CreatedAt    time.Time `json:"createdAt"`
	// PaidOn and DueOn (#1965) are the same two calendar days InvoiceView
	// carries, in the Practice's zone, for the same reasons.
	PaidOn *string `json:"paidOn,omitempty"`
	DueOn  string  `json:"dueOn"`
	// RaisedOn (#2146) is InvoiceView's own: the day CreatedAt falls on in
	// the Practice's zone.
	RaisedOn string `json:"raisedOn"`
	// DaysOverdue (#1965) is how many whole Practice days this Invoice is
	// past DueOn, and 0 when it is not overdue or not open. It is counted
	// here, at the same instant and by the same rule as OverdueCount
	// above the list, rather than on the reader's own clock: lateness is
	// a fact about the Practice's day, and a screen counting it in the
	// reader's zone would call an Invoice late that the figure above it
	// does not count. It is as fresh as the page, the same as that figure.
	DaysOverdue int `json:"daysOverdue"`
	// Reference and BillingMode are #271's additions -- see InvoiceView's
	// own doc comment; the same two facts, on the Practice-wide row.
	Reference   string `json:"reference"`
	BillingMode string `json:"billingMode"`
}

// PracticeInvoicesResponse is the cursor-pagination envelope from
// docs/api-design.md section 4, plus the three whole-book totals the
// list exists for.
//
// The totals are of every Invoice at the Practice, never of the page --
// an outstanding figure that shrank as the reader paged would be a lie
// about the book. They are returned on every page so the frontend never
// has to hold the first page's numbers while it loads later ones.
//
// "Outstanding" is status 'open' alone: money billed to a Client and not
// yet collected. A 'draft' Invoice never reached her, a 'void' one was
// canceled, and an 'uncollectible' one was written off, so none of the
// three is owed.
type PracticeInvoicesResponse struct {
	Items            []PracticeInvoiceView `json:"items"`
	NextCursor       *string               `json:"nextCursor,omitempty"`
	HasMore          bool                  `json:"hasMore"`
	OutstandingCents int64                 `json:"outstandingCents"`
	OutstandingCount int                   `json:"outstandingCount"`
	// PaidCents is every 'paid' Invoice's full amount -- bills that were
	// settled. A refunded Invoice stays 'paid' (00115), so it still counts
	// here in full, and deliberately so: the bill was settled, and this
	// figure has always meant settled bills. What went back is
	// RefundedCents, a separate number beside it, never folded silently
	// into this one where a Practice could not see it had happened.
	PaidCents int64 `json:"paidCents"`
	// RefundedCents (#1009) is every Refund across the book, as a
	// positive number: money the Practice has returned to Clients.
	RefundedCents int64 `json:"refundedCents"`
	// OverdueCents and OverdueCount (#768) are the part of the
	// outstanding book that is past its due day: 'open' and due before
	// the start of the Practice's own today (#1965, overdue.From),
	// evaluated by Postgres at read. They are a narrowing of the
	// outstanding figures beside them, never a separate book -- an
	// overdue Invoice is counted in both pairs, because it is still money
	// billed and not yet collected.
	//
	// Whole-book, like every total here, and for the same reason: a
	// Practice reading "4 overdue" under a list showing three of them has
	// been told the truth about its book and can page to find the fourth.
	OverdueCents int64 `json:"overdueCents"`
	OverdueCount int   `json:"overdueCount"`
}

// GetPracticeInvoicesHandler lists every Invoice the Practice has ever
// billed, newest first, cursor-paginated, with the whole book's
// outstanding and paid totals alongside -- the Practice-wide view gap
// RA-G7 (#265) found missing, where the only way to answer "who owes us
// money" was to open every Engagement in turn.
//
// ?unpaid=true narrows the list to the Invoices making up
// OutstandingCents/OutstandingCount (#427) -- status 'open' alone, the
// same definition practiceInvoiceTotalsQuery already uses. The totals
// stay whole-book regardless: a Practice landing block that shows "3
// unpaid, $450 outstanding" needs both numbers to agree with the list
// underneath it, not with whatever page the reader happens to be on.
//
// ?overdue=true (#768) narrows further, to the open Invoices whose own
// due day has passed in the Practice's zone (#1965) -- the Invoices
// making up OverdueCents/OverdueCount,
// by the same rule, so the list and the figure above it cannot disagree.
// It is a narrowing of ?unpaid=true, not an alternative to it, and wins
// when a caller passes both.
//
// Who may read it: Owner, Admin, and an employed Doula (ADR-0008's money
// row as amended by #282). Aggregating the whole Practice's book cannot
// be narrowed to "her own Engagements", so a contractor is refused
// outright rather than given a partial view -- her own-fee view stays
// where the per-Engagement Contract read already puts it.
//
// Must be mounted behind staffauth.Middleware.
func GetPracticeInvoicesHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireNotAmbientContractor(w, r)
		if !ok {
			return
		}

		var after *invoiceCursor
		if raw := r.URL.Query().Get("cursor"); raw != "" {
			c, err := decodeInvoiceCursor(raw)
			if err != nil {
				apierr.WriteError(w, "invalid cursor", http.StatusBadRequest)
				return
			}
			after = &c
		}
		// #1965: one zone and one instant for the whole response, so the
		// rows, the narrowing and the totals cannot disagree about which
		// day it is at the Practice.
		zone, err := practicetimezone.Load(r.Context(), tx, practiceID)
		if err != nil {
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		days := bookDays{zone: zone, now: clock.Now(r.Context())}

		items, hasMore, err := listPracticeInvoices(r.Context(), tx, practiceID, after, narrowingFor(r), days)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		totals, err := practiceInvoiceTotals(r.Context(), tx, practiceID, overdue.From(days.now, days.zone))
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		resp := PracticeInvoicesResponse{
			Items:            items,
			HasMore:          hasMore,
			OutstandingCents: totals.outstandingCents,
			OutstandingCount: totals.outstandingCount,
			PaidCents:        totals.paidCents,
			RefundedCents:    totals.refundedCents,
			OverdueCents:     totals.overdueCents,
			OverdueCount:     totals.overdueCount,
		}
		if hasMore {
			next := encodeInvoiceCursor(items[len(items)-1].CreatedAt, items[len(items)-1].ID)
			resp.NextCursor = &next
		}
		apierr.WriteJSON(w, http.StatusOK, resp)
	})
}

// practiceInvoiceColumns and practiceInvoiceJoins are shared by the first-
// page and after-cursor queries, which differ only in the cursor's WHERE
// clause and the LIMIT placeholder position -- the same split
// listInvoicesQuery / listInvoicesAfterQuery uses.
//
// The JOIN runs invoices -> contracts -> engagements -> clients: an
// Invoice hangs off a Contract, and only the Engagement behind that
// Contract knows whose Invoice it is. Joining through a since-voided
// Contract still resolves, so a voided Contract's Invoices keep their
// Client's name here exactly as they keep their place in the per-
// Engagement list (#72).
//
// activePaymentJoin is the per-Engagement list's own (invoice.go): the
// covering Payment's kind is what says whether paid_at holds a typed day
// or Stripe's instant (#1965, invoicePaidOn).
const practiceInvoiceColumns = `SELECT i.id, e.id, i.contract_id, cl.given_name, cl.preferred_name, cl.family_name,
		i.status, i.amount_cents, i.currency, i.created_at, i.paid_at, i.due_at, i.reference, i.stripe_invoice_id, ap.kind
	FROM invoices i
	JOIN contracts c ON c.id = i.contract_id
	JOIN engagements e ON e.id = c.engagement_id
	JOIN clients cl ON cl.id = e.client_id
	` + activePaymentJoin + `
	WHERE i.practice_id = $1`

// invoiceNarrowing is which slice of the book a read asks for. The zero
// value is the whole book, so a request that names no narrowing needs no
// special case.
type invoiceNarrowing int

// The three narrowings the Practice-wide list offers. Overdue is a
// narrowing of unpaid, not a sibling of it: it repeats the 'open' test
// and adds the due-date comparison, so the two can never disagree about
// what "still owed" means.
const (
	narrowAll invoiceNarrowing = iota
	narrowUnpaid
	narrowOverdue
)

// clause is the narrowing's fixed WHERE text. Overdue compares due_at
// with overdue.From's instant, passed as the query parameter cutoffParam
// names, so the boundary is the start of the Practice's own today
// (#1965) and the comparison stays an instant one on the stored column.
func (n invoiceNarrowing) clause(cutoffParam string) string {
	switch n {
	case narrowUnpaid:
		return ` AND i.status = 'open'`
	case narrowOverdue:
		return ` AND i.status = 'open' AND i.due_at < ` + cutoffParam
	case narrowAll:
	}
	return ""
}

// narrowingFor reads the two query parameters the list accepts. ?overdue
// wins when both are set, because it is the narrower of the two and a
// caller asking for both means the narrower one.
//
// The instant overdue is judged at is clock.Now(ctx), read once by the
// handler and turned into the start of the Practice's today
// (overdue.From). Before #1965 it was Postgres's now() against due_at
// itself; a due date is a day to a person, and the Practice's day is the
// one that decides. Nothing is stored, nothing is swept, and no webhook
// is involved -- Stripe emits no event when a due date passes, so this
// comparison is still the only thing that can be true at every instant.
func narrowingFor(r *http.Request) invoiceNarrowing {
	switch {
	case r.URL.Query().Get("overdue") == "true":
		return narrowOverdue
	case r.URL.Query().Get("unpaid") == "true":
		return narrowUnpaid
	default:
		return narrowAll
	}
}

// listPracticeInvoicesQuery and its after-cursor twin take the narrowing
// as a fixed clause from invoiceNarrowing rather than as a parameter --
// it is never request text, so no interpolation of a caller's input can
// reach the SQL. The overdue cutoff is the one value it carries, and it
// rides as the parameter after the query's own, bound only when the
// overdue narrowing names it.
func listPracticeInvoicesQuery(narrowing invoiceNarrowing) string {
	return practiceInvoiceColumns + narrowing.clause("$3") + `
	ORDER BY i.created_at DESC, i.id DESC LIMIT $2`
}

func listPracticeInvoicesAfterQuery(narrowing invoiceNarrowing) string {
	return practiceInvoiceColumns + narrowing.clause("$5") + `
	AND (i.created_at, i.id) < ($2, $3)
	ORDER BY i.created_at DESC, i.id DESC LIMIT $4`
}

// listPracticeInvoices fetches one page of the Practice's Invoices,
// filtered explicitly on practice_id on top of the RLS scoping
// staffauth.Middleware already set up on tx -- the app layer's own
// filter, so a bug in either one alone can't leak rows.
//
// The ordering matches invoices_practice_created_idx
// (00056_invoices_practice_listing_index.sql), so the page is an index
// scan of at most invoicePageSize+1 rows rather than a sort of the
// Practice's whole book.
func listPracticeInvoices(ctx context.Context, tx *sql.Tx, practiceID string, after *invoiceCursor, narrowing invoiceNarrowing, days bookDays) ([]PracticeInvoiceView, bool, error) {
	query := listPracticeInvoicesQuery(narrowing)
	args := []any{practiceID, invoicePageSize + 1}
	if after != nil {
		query = listPracticeInvoicesAfterQuery(narrowing)
		args = []any{practiceID, after.createdAt, after.invoiceID, invoicePageSize + 1}
	}
	if narrowing == narrowOverdue {
		args = append(args, overdue.From(days.now, days.zone))
	}
	rows, err := tx.QueryContext(ctx, query, args...)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return nil, false, fmt.Errorf("payments: list practice invoices: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := []PracticeInvoiceView{}
	for rows.Next() {
		var it PracticeInvoiceView
		var givenName string
		var preferredName, familyName sql.NullString
		var paidAt sql.NullTime
		var dueAt time.Time
		var stripeInvoiceID, activePaymentKind sql.NullString
		if err := rows.Scan(&it.ID, &it.EngagementID, &it.ContractID, &givenName, &preferredName, &familyName,
			&it.Status, &it.AmountCents, &it.Currency, &it.CreatedAt, &paidAt, &dueAt, &it.Reference, &stripeInvoiceID, &activePaymentKind); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, false, fmt.Errorf("payments: scan practice invoice row: %w", err)
		}
		it.ClientName = personname.Full(givenName, preferredName.String, familyName.String)
		it.PaidOn = invoicePaidOn(paidAt, activePaymentKind.String == paymentKindManual, days.zone)
		it.DueOn = practiceDay(dueAt, days.zone)
		it.RaisedOn = practiceDay(it.CreatedAt, days.zone)
		it.DaysOverdue = overdue.Days(it.Status, dueAt, days.now, days.zone)
		it.BillingMode = billingModeOf(stripeInvoiceID)
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, false, fmt.Errorf("payments: iterate practice invoice rows: %w", err)
	}

	hasMore := len(items) > invoicePageSize
	if hasMore {
		items = items[:invoicePageSize]
	}
	return items, hasMore, nil
}

// invoiceTotals is the whole-book summary the Practice-wide list carries
// on every page.
type invoiceTotals struct {
	outstandingCents int64
	outstandingCount int
	paidCents        int64
	overdueCents     int64
	overdueCount     int
	refundedCents    int64
}

// practiceInvoiceTotalsQuery reads every total in one pass with FILTER
// clauses rather than several queries or a GROUP BY the caller then has
// to re-shape. COALESCE covers the empty book, where SUM is null.
//
// The refunded total (#1009) is a scalar subquery over payments, since
// it is a sum of rows that are not Invoices. It is the whole book's money
// gone back to Clients, as a positive number.
//
// $2 is overdue.From's instant, the start of the Practice's today
// (#1965): the same boundary the ?overdue=true narrowing binds.
const practiceInvoiceTotalsQuery = `SELECT
		COALESCE(SUM(amount_cents) FILTER (WHERE status = 'open'), 0),
		COUNT(*) FILTER (WHERE status = 'open'),
		COALESCE(SUM(amount_cents) FILTER (WHERE status = 'paid'), 0),
		COALESCE(SUM(amount_cents) FILTER (WHERE status = 'open' AND due_at < $2), 0),
		COUNT(*) FILTER (WHERE status = 'open' AND due_at < $2),
		(SELECT COALESCE(-SUM(p.amount_cents), 0) FROM payments p
		   JOIN invoices ri ON ri.id = p.invoice_id
		  WHERE ri.practice_id = $1 AND p.kind = 'refund')
	FROM invoices WHERE practice_id = $1`

// practiceInvoiceTotals sums the Practice's outstanding and paid money.
// It reads every row rather than a page, which is why it stays an
// aggregate in Postgres instead of a fold over the fetched page: a
// 14-doula agency's book is thousands of rows, and only three numbers
// cross the wire.
func practiceInvoiceTotals(ctx context.Context, tx *sql.Tx, practiceID string, overdueCutoff time.Time) (invoiceTotals, error) {
	var t invoiceTotals
	if err := tx.QueryRowContext(ctx, practiceInvoiceTotalsQuery, practiceID, overdueCutoff).
		Scan(&t.outstandingCents, &t.outstandingCount, &t.paidCents, &t.overdueCents, &t.overdueCount, &t.refundedCents); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return invoiceTotals{}, fmt.Errorf("payments: practice invoice totals: %w", err)
	}
	return t, nil
}
