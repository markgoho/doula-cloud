package payments

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/client"
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
// ClientName is client.PreferredName, the conversation name every screen
// uses (ADR-0017's read table). The legal name belongs to the documents
// -- the Contract's merge field and the Stripe Invoice itself -- not to a
// staff-facing list.
type PracticeInvoiceView struct {
	ID           string     `json:"id"`
	EngagementID string     `json:"engagementId"`
	ContractID   string     `json:"contractId"`
	ClientName   string     `json:"clientName"`
	Status       string     `json:"status"`
	AmountCents  int64      `json:"amountCents"`
	Currency     string     `json:"currency"`
	CreatedAt    time.Time  `json:"createdAt"`
	PaidAt       *time.Time `json:"paidAt,omitempty"`
	// DueAt (#768) is the date this Invoice falls due -- see InvoiceView's
	// own field, which carries the same value for the same reason. The
	// row says when the money was due, never how many days late it is:
	// the day count is derived where it is read, so it is right at every
	// instant rather than right at the moment the page loaded.
	DueAt time.Time `json:"dueAt"`
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
	PaidCents        int64                 `json:"paidCents"`
	// OverdueCents and OverdueCount (#768) are the part of the
	// outstanding book that is past its due date: 'open' and due_at <
	// now(), evaluated by Postgres at read. They are a narrowing of the
	// outstanding figures beside them, never a separate book -- an
	// overdue Invoice is counted in both pairs, because it is still money
	// billed and not yet collected.
	//
	// Whole-book, like every total here, and for the same reason: a
	// Practice reading "4 overdue" under a list showing three of them has
	// been told the truth about its book and can page to find the fourth.
	OverdueCents int64 `json:"overdueCents"`
	OverdueCount int   `json:"overdueCount"`
	// ClientsCanPay (#270) is ClientsCanPay's own charges-active test --
	// an aggregate fact about the Practice's book, alongside the three
	// totals above, not one more per-row field. It lets the list say
	// "Clients cannot pay this Practice yet" as a standing line rather
	// than leaving that fact to be inferred from an empty book, which
	// looks identical to "nobody has billed anything yet".
	ClientsCanPay bool `json:"clientsCanPay"`
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
// due_at has passed -- the Invoices making up OverdueCents/OverdueCount,
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
		items, hasMore, err := listPracticeInvoices(r.Context(), tx, practiceID, after, narrowingFor(r))
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		totals, err := practiceInvoiceTotals(r.Context(), tx, practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		clientsCanPay, err := ClientsCanPay(r.Context(), tx, practiceID)
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
			OverdueCents:     totals.overdueCents,
			OverdueCount:     totals.overdueCount,
			ClientsCanPay:    clientsCanPay,
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
const practiceInvoiceColumns = `SELECT i.id, e.id, i.contract_id, cl.given_name, cl.preferred_name,
		i.status, i.amount_cents, i.currency, i.created_at, i.paid_at, i.due_at, i.reference, i.stripe_invoice_id
	FROM invoices i
	JOIN contracts c ON c.id = i.contract_id
	JOIN engagements e ON e.id = c.engagement_id
	JOIN clients cl ON cl.id = e.client_id
	WHERE i.practice_id = $1`

// invoiceNarrowing is which slice of the book a read asks for. The zero
// value is the whole book, so a request that names no narrowing needs no
// special case.
type invoiceNarrowing string

// The three narrowings the Practice-wide list offers. Overdue is a
// narrowing of unpaid, not a sibling of it: it repeats the 'open' test
// and adds the due-date comparison, so the two can never disagree about
// what "still owed" means.
const (
	narrowAll     invoiceNarrowing = ""
	narrowUnpaid  invoiceNarrowing = ` AND i.status = 'open'`
	narrowOverdue invoiceNarrowing = ` AND i.status = 'open' AND i.due_at < now()`
)

// narrowingFor reads the two query parameters the list accepts. ?overdue
// wins when both are set, because it is the narrower of the two and a
// caller asking for both means the narrower one.
//
// now() is Postgres's, not Go's: it is the same clock that stamped
// created_at and computed due_at at raise (payment_terms.go's
// invoiceDueAt), and the one simclock shifts when a sandbox runs
// compressed time. Nothing is stored, nothing is swept, and no webhook
// is involved -- Stripe emits no event when a due date passes, so this
// comparison is the only thing that can be true at every instant.
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
// reach the SQL.
func listPracticeInvoicesQuery(narrowing invoiceNarrowing) string {
	return practiceInvoiceColumns + string(narrowing) + `
	ORDER BY i.created_at DESC, i.id DESC LIMIT $2`
}

func listPracticeInvoicesAfterQuery(narrowing invoiceNarrowing) string {
	return practiceInvoiceColumns + string(narrowing) + `
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
func listPracticeInvoices(ctx context.Context, tx *sql.Tx, practiceID string, after *invoiceCursor, narrowing invoiceNarrowing) ([]PracticeInvoiceView, bool, error) {
	var rows *sql.Rows
	var err error
	if after != nil {
		rows, err = tx.QueryContext(ctx, listPracticeInvoicesAfterQuery(narrowing), practiceID, after.createdAt, after.invoiceID, invoicePageSize+1)
	} else {
		rows, err = tx.QueryContext(ctx, listPracticeInvoicesQuery(narrowing), practiceID, invoicePageSize+1)
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return nil, false, fmt.Errorf("payments: list practice invoices: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := []PracticeInvoiceView{}
	for rows.Next() {
		var it PracticeInvoiceView
		var givenName string
		var preferredName sql.NullString
		var paidAt sql.NullTime
		var stripeInvoiceID sql.NullString
		if err := rows.Scan(&it.ID, &it.EngagementID, &it.ContractID, &givenName, &preferredName,
			&it.Status, &it.AmountCents, &it.Currency, &it.CreatedAt, &paidAt, &it.DueAt, &it.Reference, &stripeInvoiceID); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, false, fmt.Errorf("payments: scan practice invoice row: %w", err)
		}
		it.ClientName = client.PreferredName(givenName, preferredName.String)
		if paidAt.Valid {
			it.PaidAt = &paidAt.Time
		}
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
}

// practiceInvoiceTotalsQuery reads all three totals in one pass with
// FILTER clauses rather than three queries or a GROUP BY the caller then
// has to re-shape. COALESCE covers the empty book, where SUM is null.
const practiceInvoiceTotalsQuery = `SELECT
		COALESCE(SUM(amount_cents) FILTER (WHERE status = 'open'), 0),
		COUNT(*) FILTER (WHERE status = 'open'),
		COALESCE(SUM(amount_cents) FILTER (WHERE status = 'paid'), 0),
		COALESCE(SUM(amount_cents) FILTER (WHERE status = 'open' AND due_at < now()), 0),
		COUNT(*) FILTER (WHERE status = 'open' AND due_at < now())
	FROM invoices WHERE practice_id = $1`

// practiceInvoiceTotals sums the Practice's outstanding and paid money.
// It reads every row rather than a page, which is why it stays an
// aggregate in Postgres instead of a fold over the fetched page: a
// 14-doula agency's book is thousands of rows, and only three numbers
// cross the wire.
func practiceInvoiceTotals(ctx context.Context, tx *sql.Tx, practiceID string) (invoiceTotals, error) {
	var t invoiceTotals
	if err := tx.QueryRowContext(ctx, practiceInvoiceTotalsQuery, practiceID).
		Scan(&t.outstandingCents, &t.outstandingCount, &t.paidCents, &t.overdueCents, &t.overdueCount); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return invoiceTotals{}, fmt.Errorf("payments: practice invoice totals: %w", err)
	}
	return t, nil
}
