package payments

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

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
// cancelled, and an 'uncollectible' one was written off, so none of the
// three is owed.
type PracticeInvoicesResponse struct {
	Items            []PracticeInvoiceView `json:"items"`
	NextCursor       *string               `json:"nextCursor,omitempty"`
	HasMore          bool                  `json:"hasMore"`
	OutstandingCents int64                 `json:"outstandingCents"`
	OutstandingCount int                   `json:"outstandingCount"`
	PaidCents        int64                 `json:"paidCents"`
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
// Who may read it: Owner and Admin only. ADR-0006 put "Contract -- money,
// and Invoice history" on the Owner/Admin row of its read table, and
// ADR-0008 (which supersedes that table) keeps it there, adding that a
// contractor Doula may read the money on her own Engagements and an
// employee Doula never may. Aggregating the whole Practice's book cannot
// be narrowed to "her own Engagements" without becoming a different
// screen, so this endpoint takes the unambiguous half of the row and is
// mounted Owner/Admin -- the same declaration the per-Engagement
// GetInvoicesHandler mount carries. A contractor's own-fee view stays
// where the per-Engagement Contract read already puts it.
//
// Must be mounted behind staffauth.Middleware.
func GetPracticeInvoicesHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireOwnerOrAdmin(w, r)
		if !ok {
			return
		}

		var after *invoiceCursor
		if raw := r.URL.Query().Get("cursor"); raw != "" {
			c, err := decodeInvoiceCursor(raw)
			if err != nil {
				http.Error(w, "invalid cursor", http.StatusBadRequest)
				return
			}
			after = &c
		}
		unpaidOnly := r.URL.Query().Get("unpaid") == "true"

		items, hasMore, err := listPracticeInvoices(r.Context(), tx, practiceID, after, unpaidOnly)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		totals, err := practiceInvoiceTotals(r.Context(), tx, practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		resp := PracticeInvoicesResponse{
			Items:            items,
			HasMore:          hasMore,
			OutstandingCents: totals.outstandingCents,
			OutstandingCount: totals.outstandingCount,
			PaidCents:        totals.paidCents,
		}
		if hasMore {
			next := encodeInvoiceCursor(items[len(items)-1].CreatedAt, items[len(items)-1].ID)
			resp.NextCursor = &next
		}
		writeJSON(w, http.StatusOK, resp)
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
		i.status, i.amount_cents, i.currency, i.created_at, i.paid_at
	FROM invoices i
	JOIN contracts c ON c.id = i.contract_id
	JOIN engagements e ON e.id = c.engagement_id
	JOIN clients cl ON cl.id = e.client_id
	WHERE i.practice_id = $1`

// practiceInvoiceUnpaidFilter is appended for ?unpaid=true -- 'open' is
// the same "billed and not yet collected" definition
// practiceInvoiceTotalsQuery's outstanding totals already use.
const practiceInvoiceUnpaidFilter = ` AND i.status = 'open'`

const listPracticeInvoicesQuery = practiceInvoiceColumns + `
	ORDER BY i.created_at DESC, i.id DESC LIMIT $2`

const listPracticeInvoicesUnpaidQuery = practiceInvoiceColumns + practiceInvoiceUnpaidFilter + `
	ORDER BY i.created_at DESC, i.id DESC LIMIT $2`

const listPracticeInvoicesAfterQuery = practiceInvoiceColumns + `
	AND (i.created_at, i.id) < ($2, $3)
	ORDER BY i.created_at DESC, i.id DESC LIMIT $4`

const listPracticeInvoicesUnpaidAfterQuery = practiceInvoiceColumns + practiceInvoiceUnpaidFilter + `
	AND (i.created_at, i.id) < ($2, $3)
	ORDER BY i.created_at DESC, i.id DESC LIMIT $4`

// listPracticeInvoices fetches one page of the Practice's Invoices,
// filtered explicitly on practice_id on top of the RLS scoping
// staffauth.Middleware already set up on tx -- the app layer's own
// filter, so a bug in either one alone can't leak rows.
//
// The ordering matches invoices_practice_created_idx
// (00056_invoices_practice_listing_index.sql), so the page is an index
// scan of at most invoicePageSize+1 rows rather than a sort of the
// Practice's whole book.
func listPracticeInvoices(ctx context.Context, tx *sql.Tx, practiceID string, after *invoiceCursor, unpaidOnly bool) ([]PracticeInvoiceView, bool, error) {
	var rows *sql.Rows
	var err error
	switch {
	case after != nil && unpaidOnly:
		rows, err = tx.QueryContext(ctx, listPracticeInvoicesUnpaidAfterQuery, practiceID, after.createdAt, after.invoiceID, invoicePageSize+1)
	case after != nil:
		rows, err = tx.QueryContext(ctx, listPracticeInvoicesAfterQuery, practiceID, after.createdAt, after.invoiceID, invoicePageSize+1)
	case unpaidOnly:
		rows, err = tx.QueryContext(ctx, listPracticeInvoicesUnpaidQuery, practiceID, invoicePageSize+1)
	default:
		rows, err = tx.QueryContext(ctx, listPracticeInvoicesQuery, practiceID, invoicePageSize+1)
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
		if err := rows.Scan(&it.ID, &it.EngagementID, &it.ContractID, &givenName, &preferredName,
			&it.Status, &it.AmountCents, &it.Currency, &it.CreatedAt, &paidAt); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, false, fmt.Errorf("payments: scan practice invoice row: %w", err)
		}
		it.ClientName = client.PreferredName(givenName, preferredName.String)
		if paidAt.Valid {
			it.PaidAt = &paidAt.Time
		}
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
}

// practiceInvoiceTotalsQuery reads all three totals in one pass with
// FILTER clauses rather than three queries or a GROUP BY the caller then
// has to re-shape. COALESCE covers the empty book, where SUM is null.
const practiceInvoiceTotalsQuery = `SELECT
		COALESCE(SUM(amount_cents) FILTER (WHERE status = 'open'), 0),
		COUNT(*) FILTER (WHERE status = 'open'),
		COALESCE(SUM(amount_cents) FILTER (WHERE status = 'paid'), 0)
	FROM invoices WHERE practice_id = $1`

// practiceInvoiceTotals sums the Practice's outstanding and paid money.
// It reads every row rather than a page, which is why it stays an
// aggregate in Postgres instead of a fold over the fetched page: a
// 14-doula agency's book is thousands of rows, and only three numbers
// cross the wire.
func practiceInvoiceTotals(ctx context.Context, tx *sql.Tx, practiceID string) (invoiceTotals, error) {
	var t invoiceTotals
	if err := tx.QueryRowContext(ctx, practiceInvoiceTotalsQuery, practiceID).
		Scan(&t.outstandingCents, &t.outstandingCount, &t.paidCents); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return invoiceTotals{}, fmt.Errorf("payments: practice invoice totals: %w", err)
	}
	return t, nil
}
