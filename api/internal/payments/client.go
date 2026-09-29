package payments

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clientauth"
)

// msgClientInvoiceNotFound is the one thing a Client is told about an
// Invoice she cannot have. RLS returns zero rows for another Client's
// Invoice and for one that does not exist, and a malformed id reaches the
// same words, so the three are indistinguishable to her (#1011).
const msgClientInvoiceNotFound = "invoice not found"

// ClientInvoiceView is one Invoice as its Client reads it (#1011). It is
// a separate type from InvoiceView, not a narrowing of it: what a Client
// may read is a short allowlist, and a field added to the Staff view must
// never reach her by default.
//
// Never on it, by construction: stripe_invoice_id, stripe_customer_id,
// stripe_payment_reference, contract_id, and a Payment's note (#981: she
// reads the method, never the recorder's note). Status is never "draft" --
// invoices_client_visibility (00119) refuses one in Postgres and the query
// refuses it again, so neither layer depends on the other.
type ClientInvoiceView struct {
	ID          string `json:"id"`
	Status      string `json:"status"` // open | paid | void | uncollectible
	AmountCents int64  `json:"amountCents"`
	Currency    string `json:"currency"`
	// Reference is her Invoice number (#981).
	Reference   string     `json:"reference"`
	BillingMode string     `json:"billingMode"` // stripe | by_hand
	CreatedAt   time.Time  `json:"createdAt"`
	PaidAt      *time.Time `json:"paidAt,omitempty"`
	// PaidMethod is present only where a manual Payment record exists. A
	// Stripe-paid Invoice carries none, and it stays absent rather than
	// reading "Card": the model records no card-versus-bank fact about a
	// Stripe payment (#1011).
	PaidMethod *string `json:"paidMethod,omitempty"`
	// RefundedCents is how much of this Invoice has gone back to her, a
	// positive number, 0 when none has (#1009). A refunded Invoice stays
	// 'paid', so this is the only thing that says money came back.
	RefundedCents int64 `json:"refundedCents"`
}

// ClientInvoicesResponse is the docs/api-design.md section 4 envelope
// plus the Engagement's own totals. One list serves both "What you still
// owe" and "What you have paid": the second is the paid Invoices, so there
// is no second array (#1011).
//
// The totals are whole-Engagement on every page, never page-scoped -- a
// figure that shrank as she paged would be a lie about what she owes.
type ClientInvoicesResponse struct {
	Items           []ClientInvoiceView `json:"items"`
	NextCursor      *string             `json:"nextCursor,omitempty"`
	HasMore         bool                `json:"hasMore"`
	TotalToPayCents int64               `json:"totalToPayCents"`
	TotalToPayCount int                 `json:"totalToPayCount"`
}

// clientInvoiceSelect reads the ClientInvoiceView columns. The LATERAL
// picks the method of the live manual Payment -- one nothing has reversed
// -- and only for a paid Invoice, so a reopened Invoice never shows the
// method of money that is no longer counted.
//
// c.engagement_id = $1 and status <> 'draft' are the handler's own
// filters on top of the RLS the Client session already carries.
const clientInvoiceSelect = `SELECT i.id, i.status, i.amount_cents, i.currency, i.reference, i.stripe_invoice_id,
		i.created_at, i.paid_at, mp.method, ` + refundedCentsSubquery + `
	FROM invoices i
	JOIN contracts c ON c.id = i.contract_id
	LEFT JOIN LATERAL (SELECT p.method::text AS method FROM payments p
	 WHERE p.invoice_id = i.id AND p.kind = 'manual' AND i.status = 'paid'
	   AND NOT EXISTS (SELECT 1 FROM payments r WHERE r.target_payment_id = p.id AND r.kind = 'reversal')
	 ORDER BY p.created_at DESC LIMIT 1) mp ON true
	WHERE c.engagement_id = $1 AND i.status <> 'draft'`

const (
	clientInvoicesQuery = clientInvoiceSelect + `
	ORDER BY i.created_at DESC, i.id DESC LIMIT $2`
	clientInvoicesAfterQuery = clientInvoiceSelect + `
	AND (i.created_at, i.id) < ($2, $3)
	ORDER BY i.created_at DESC, i.id DESC LIMIT $4`
	clientInvoiceOneQuery = clientInvoiceSelect + ` AND i.id = $2`
)

// totalToPayQuery sums the Engagement's open Invoices and nothing else:
// the model holds no partial Payments (#981), so what she owes is the sum
// of her open Invoices.
const totalToPayQuery = `SELECT COALESCE(SUM(i.amount_cents), 0), COUNT(*)
	FROM invoices i
	JOIN contracts c ON c.id = i.contract_id
	WHERE c.engagement_id = $1 AND i.status = 'open'`

// ClientTotalToPay is what a Client still owes across one Engagement: the
// sum and count of its open Invoices. The one source of truth for the
// list's totals and for the portal hub's single line (#1011), so the two
// cannot disagree. tx must be a Client-portal transaction.
func ClientTotalToPay(ctx context.Context, tx *sql.Tx, engagementID string) (cents int64, count int, err error) {
	if err := tx.QueryRowContext(ctx, totalToPayQuery, engagementID).Scan(&cents, &count); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return 0, 0, fmt.Errorf("payments: total to pay: %w", err)
	}
	return cents, count, nil
}

// scanClientInvoice reads one row of clientInvoiceSelect.
func scanClientInvoice(row interface{ Scan(dest ...any) error }) (ClientInvoiceView, error) {
	var it ClientInvoiceView
	var paidAt sql.NullTime
	var stripeInvoiceID, method sql.NullString
	if err := row.Scan(&it.ID, &it.Status, &it.AmountCents, &it.Currency, &it.Reference, &stripeInvoiceID,
		&it.CreatedAt, &paidAt, &method, &it.RefundedCents); err != nil {
		return ClientInvoiceView{}, fmt.Errorf("payments: scan client invoice: %w", err)
	}
	if paidAt.Valid {
		it.PaidAt = &paidAt.Time
	}
	if method.Valid {
		it.PaidMethod = &method.String
	}
	it.BillingMode = billingModeOf(stripeInvoiceID)
	return it, nil
}

// ClientListInvoicesHandler lists the caller's Engagement's Invoices,
// newest first, cursor-paginated. Must be mounted behind
// clientauth.Middleware, which has already confirmed her Client owns
// :engagementId.
func ClientListInvoicesHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, has := clientauth.Tx(r.Context())
		if !has {
			// coverage:ignore reason: clientauth.Middleware always sets a tx before this handler runs
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		engagementID, _ := clientauth.EngagementID(r.Context())

		var after *invoiceCursor
		if raw := r.URL.Query().Get("cursor"); raw != "" {
			c, err := decodeInvoiceCursor(raw)
			if err != nil {
				apierr.WriteError(w, "invalid cursor", http.StatusBadRequest)
				return
			}
			after = &c
		}

		items, hasMore, err := listClientInvoices(r.Context(), tx, engagementID, after)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		cents, count, err := ClientTotalToPay(r.Context(), tx, engagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		resp := ClientInvoicesResponse{Items: items, HasMore: hasMore, TotalToPayCents: cents, TotalToPayCount: count}
		if hasMore {
			next := encodeInvoiceCursor(items[len(items)-1].CreatedAt, items[len(items)-1].ID)
			resp.NextCursor = &next
		}
		apierr.WriteJSON(w, http.StatusOK, resp)
	})
}

// listClientInvoices fetches one page under engagementID.
func listClientInvoices(ctx context.Context, tx *sql.Tx, engagementID string, after *invoiceCursor) ([]ClientInvoiceView, bool, error) {
	var rows *sql.Rows
	var err error
	if after != nil {
		rows, err = tx.QueryContext(ctx, clientInvoicesAfterQuery, engagementID, after.createdAt, after.invoiceID, invoicePageSize+1)
	} else {
		rows, err = tx.QueryContext(ctx, clientInvoicesQuery, engagementID, invoicePageSize+1)
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return nil, false, fmt.Errorf("payments: list client invoices: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := []ClientInvoiceView{}
	for rows.Next() {
		it, err := scanClientInvoice(rows)
		if err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, false, fmt.Errorf("payments: scan client invoice row: %w", err)
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, false, fmt.Errorf("payments: iterate client invoice rows: %w", err)
	}

	hasMore := len(items) > invoicePageSize
	if hasMore {
		items = items[:invoicePageSize]
	}
	return items, hasMore, nil
}

// ClientGetInvoiceHandler returns one Invoice of the caller's Engagement:
// the same row the list carries, so a Client cannot see one figure on the
// list and another on the page. A 404 covers both "no such Invoice" and
// "not hers". Must be mounted behind clientauth.Middleware.
func ClientGetInvoiceHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, has := clientauth.Tx(r.Context())
		if !has {
			// coverage:ignore reason: clientauth.Middleware always sets a tx before this handler runs
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		engagementID, _ := clientauth.EngagementID(r.Context())
		invoiceID := r.PathValue("invoiceId")
		if _, err := uuid.Parse(invoiceID); err != nil {
			apierr.WriteError(w, msgClientInvoiceNotFound, http.StatusNotFound)
			return
		}

		it, err := scanClientInvoice(tx.QueryRowContext(r.Context(), clientInvoiceOneQuery, engagementID, invoiceID))
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, msgClientInvoiceNotFound, http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, it)
	})
}
