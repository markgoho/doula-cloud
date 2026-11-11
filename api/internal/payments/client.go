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
	"doula-cloud/api/internal/invoicestatus"
	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/practicetimezone"
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
	Reference   string    `json:"reference"`
	BillingMode string    `json:"billingMode"` // stripe | by_hand
	CreatedAt   time.Time `json:"createdAt"`
	// RaisedOn (#2205) is the day the Invoice was sent to her, YYYY-MM-DD:
	// the day CreatedAt falls on in her Practice's zone, the same day the
	// Practice's own screens print as Raised (#2146), whatever zone her
	// browser is in.
	RaisedOn string `json:"raisedOn"`
	// DueOn (#2836) is the day an unpaid Invoice falls due, YYYY-MM-DD, in
	// her Practice's zone. Absent on an Invoice that is not open, and on
	// an open one raised before her Engagement's birth outcome was
	// recorded (dueDayGate): the BFF leaves it out, the page never hides
	// it. Never a count of days late; lateness stays the Practice's own
	// fact (ADR-0038).
	DueOn *string `json:"dueOn,omitempty"`
	// PaidOn (#1965) is the day she paid, YYYY-MM-DD, in her Practice's
	// zone -- the same day the Practice's own screens print
	// (invoicePaidOn).
	PaidOn *string `json:"paidOn,omitempty"`
	// PaidMethod is present only where a manual Payment record exists. A
	// Stripe-paid Invoice carries none, and it stays absent rather than
	// reading "Card": the model records no card-versus-bank fact about a
	// Stripe payment (#1011).
	PaidMethod *string `json:"paidMethod,omitempty"`
	// RefundedCents is how much of this Invoice has gone back to her, a
	// positive number, 0 when none has (#1009). A refunded Invoice stays
	// 'paid', so this is the only thing that says money came back.
	RefundedCents int64 `json:"refundedCents"`
	// PaymentProcessing (#2225) is true while her payment on this open
	// Invoice is on its way -- a bank payment, for days -- so the portal
	// says so rather than "Not yet paid". Status stays open, as Stripe's
	// does.
	PaymentProcessing bool `json:"paymentProcessing"`
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

// clientInvoiceSelect reads the ClientInvoiceView columns. The method is
// the standing Payment's (activePaymentJoin), when it was recorded by
// hand, and only on a paid Invoice: a reversed Payment no longer stands,
// so a reopened Invoice never shows the method of money that is no
// longer counted.
//
// c.engagement_id = $1 and status <> 'draft' are the handler's own
// filters on top of the RLS the Client session already carries.
const clientInvoiceSelect = `SELECT i.id, i.status, i.amount_cents, i.currency, i.reference, i.stripe_invoice_id,
		i.created_at, i.paid_at, i.due_at, CASE WHEN ap.kind = 'manual' AND i.status = 'paid' THEN ap.method END,
		` + refundedCentsColumn + `, ` + paymentProcessingExpr + `
	FROM invoices i
	JOIN contracts c ON c.id = i.contract_id
	` + activePaymentJoin + `
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
const totalToPayQuery = `SELECT COALESCE(SUM(i.amount_cents), 0), COUNT(*),
	       CASE WHEN COUNT(*) = 1 THEN MIN(i.id::text) END
	FROM invoices i
	JOIN contracts c ON c.id = i.contract_id
	WHERE c.engagement_id = $1 AND i.status = 'open'`

// ClientTotalToPay is what a Client still owes across one Engagement: the
// sum and count of its open Invoices. The one source of truth for the
// list's totals and for the portal hub's single line (#1011), so the two
// cannot disagree. tx must be a Client-portal transaction.
func ClientTotalToPay(ctx context.Context, tx *sql.Tx, engagementID string) (cents int64, count int, err error) {
	cents, count, _, err = ClientOpenInvoices(ctx, tx, engagementID)
	return cents, count, err
}

// ClientOpenInvoices is ClientTotalToPay plus the id of the Engagement's
// one open Invoice, nil unless exactly one is open, so the portal hub can
// link straight to it (#2697).
func ClientOpenInvoices(ctx context.Context, tx *sql.Tx, engagementID string) (cents int64, count int, onlyID *string, err error) {
	var only sql.NullString
	if err := tx.QueryRowContext(ctx, totalToPayQuery, engagementID).Scan(&cents, &count, &only); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return 0, 0, nil, fmt.Errorf("payments: total to pay: %w", err)
	}
	if only.Valid {
		onlyID = &only.String
	}
	return cents, count, onlyID, nil
}

// clientPracticeZone is the zone of the Practice behind engagementID
// (#1965), read on the Client's own transaction: engagements is hers to
// read under her RLS, and practicetimezone.Load reads practices by id.
func clientPracticeZone(ctx context.Context, tx *sql.Tx, engagementID string) (*time.Location, error) {
	var practiceID string
	if err := tx.QueryRowContext(ctx,
		`SELECT practice_id FROM engagements WHERE id = $1`, engagementID,
	).Scan(&practiceID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests -- clientauth.Middleware already proved she owns this Engagement
		return nil, fmt.Errorf("payments: read client engagement practice: %w", err)
	}
	zone, err := practicetimezone.Load(ctx, tx, practiceID)
	if err != nil {
		return nil, fmt.Errorf("payments: load client practice zone: %w", err)
	}
	return zone, nil
}

// clientPracticeView is what a Client's Invoice rows are read against:
// her Practice's zone, and whether a due day may be shown on an Invoice
// raised at a given instant.
type clientPracticeView struct {
	zone *time.Location
	gate dueDayGate
}

// dueDayGate decides, at the BFF, whether an open Invoice carries its due
// day (#2836, ADR-0038). With no birth outcome recorded every one does.
// Once an outcome is recorded, only an Invoice raised after it was does:
// raising one then is the Practice saying what is owed. An outcome with
// no recording time on file withholds every day, presuming nothing. The
// outcome itself, and which one it is, never leaves this type (ADR-0015).
type dueDayGate struct {
	outcomeRecorded bool
	recordedAt      sql.NullTime
}

func (g dueDayGate) shows(raisedAt time.Time) bool {
	if !g.outcomeRecorded {
		return true
	}
	return g.recordedAt.Valid && raisedAt.After(g.recordedAt.Time)
}

// clientDueDayGateQuery reads whether an outcome stands and when it was
// first recorded: the latest birth_outcome_recorded row that took the
// outcome from none to one. A correction (one outcome for another) is not
// a new recording, and an outcome cleared and entered again is.
const clientDueDayGateQuery = `SELECT e.birth_outcome IS NOT NULL,
	       (SELECT max(a.created_at) FROM activity a
	         WHERE a.practice_id = e.practice_id AND a.subject_kind = 'engagement'
	           AND a.subject_id = e.id AND a.action = 'birth_outcome_recorded'
	           AND a.diff ->> 'birthOutcomeBefore' IS NULL AND a.diff ->> 'birthOutcomeAfter' IS NOT NULL)
	  FROM engagements e WHERE e.id = $1`

// clientPracticeViewOf reads the zone and the gate for engagementID on
// the Client's own transaction. activity's policy compares against
// app.current_practice_id, which a Client transaction never sets, so it
// is set for this one read and cleared at once, the way
// portal.ActivityHandler sets it for its own read.
func clientPracticeViewOf(ctx context.Context, tx *sql.Tx, engagementID string) (clientPracticeView, error) {
	zone, err := clientPracticeZone(ctx, tx, engagementID)
	if err != nil {
		return clientPracticeView{}, err
	}
	var practiceID string
	if err := tx.QueryRowContext(ctx, `SELECT practice_id FROM engagements WHERE id = $1`, engagementID).Scan(&practiceID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests -- clientPracticeZone just read this row
		return clientPracticeView{}, fmt.Errorf("payments: read client engagement practice: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return clientPracticeView{}, fmt.Errorf("payments: scope client due day read: %w", err)
	}
	var gate dueDayGate
	if err := tx.QueryRowContext(ctx, clientDueDayGateQuery, engagementID).Scan(&gate.outcomeRecorded, &gate.recordedAt); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return clientPracticeView{}, fmt.Errorf("payments: read birth outcome recording: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_practice_id', '', true)`); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return clientPracticeView{}, fmt.Errorf("payments: clear client due day scope: %w", err)
	}
	return clientPracticeView{zone: zone, gate: gate}, nil
}

// scanClientInvoice reads one row of clientInvoiceSelect. A live manual
// Payment's method is the sign paid_at holds the typed day rather than
// Stripe's instant (invoicePaidOn).
func scanClientInvoice(row interface{ Scan(dest ...any) error }, practice clientPracticeView) (ClientInvoiceView, error) {
	zone := practice.zone
	var it ClientInvoiceView
	var paidAt sql.NullTime
	var dueAt time.Time
	var stripeInvoiceID, method sql.NullString
	if err := row.Scan(&it.ID, &it.Status, &it.AmountCents, &it.Currency, &it.Reference, &stripeInvoiceID,
		&it.CreatedAt, &paidAt, &dueAt, &method, &it.RefundedCents, &it.PaymentProcessing); err != nil {
		return ClientInvoiceView{}, fmt.Errorf("payments: scan client invoice: %w", err)
	}
	it.RaisedOn = practiceDay(it.CreatedAt, zone)
	if it.Status == string(invoicestatus.Open) && practice.gate.shows(it.CreatedAt) {
		day := practiceDay(dueAt, zone)
		it.DueOn = &day
	}
	it.PaidOn = invoicePaidOn(paidAt, method.Valid, zone)
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
	return clientauth.Handler(func(w http.ResponseWriter, r *http.Request, c clientauth.Call) {
		after, ok := pagecursor.FromQuery(w, r, decodeInvoiceCursor, "invalid cursor")
		if !ok {
			return
		}

		practice, err := clientPracticeViewOf(r.Context(), c.Tx, c.EngagementID)
		if err != nil {
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		items, next, err := listClientInvoices(r.Context(), c.Tx, c.EngagementID, after, practice)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		cents, count, err := ClientTotalToPay(r.Context(), c.Tx, c.EngagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, ClientInvoicesResponse{Items: items, HasMore: next != nil, NextCursor: next, TotalToPayCents: cents, TotalToPayCount: count})
	})
}

// listClientInvoices fetches one page under engagementID.
func listClientInvoices(ctx context.Context, tx *sql.Tx, engagementID string, after *invoiceCursor, practice clientPracticeView) ([]ClientInvoiceView, *string, error) {
	var rows *sql.Rows
	var err error
	if after != nil {
		rows, err = tx.QueryContext(ctx, clientInvoicesAfterQuery, engagementID, after.createdAt, after.invoiceID, invoicePageSize+1)
	} else {
		rows, err = tx.QueryContext(ctx, clientInvoicesQuery, engagementID, invoicePageSize+1)
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return nil, nil, fmt.Errorf("payments: list client invoices: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := []ClientInvoiceView{}
	for rows.Next() {
		it, err := scanClientInvoice(rows, practice)
		if err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, nil, fmt.Errorf("payments: scan client invoice row: %w", err)
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, nil, fmt.Errorf("payments: iterate client invoice rows: %w", err)
	}

	items, next, _ := pagecursor.Cut(items, invoicePageSize, func(v ClientInvoiceView) (time.Time, string) {
		return v.CreatedAt, v.ID
	})
	return items, next, nil
}

// clientInvoiceID reads {invoiceId}, answering a malformed one with the
// same 404 an Invoice that is not hers gets.
func clientInvoiceID(w http.ResponseWriter, r *http.Request) (string, bool) {
	invoiceID := r.PathValue("invoiceId")
	if _, err := uuid.Parse(invoiceID); err != nil {
		apierr.WriteError(w, msgClientInvoiceNotFound, http.StatusNotFound)
		return "", false
	}
	return invoiceID, true
}

// ClientGetInvoiceHandler returns one Invoice of the caller's Engagement:
// the same row the list carries, so a Client cannot see one figure on the
// list and another on the page. A 404 covers both "no such Invoice" and
// "not hers". Must be mounted behind clientauth.Middleware.
func ClientGetInvoiceHandler() http.Handler {
	return clientauth.Handler(func(w http.ResponseWriter, r *http.Request, c clientauth.Call) {
		invoiceID, ok := clientInvoiceID(w, r)
		if !ok {
			return
		}

		practice, err := clientPracticeViewOf(r.Context(), c.Tx, c.EngagementID)
		if err != nil {
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		it, err := scanClientInvoice(c.Tx.QueryRowContext(r.Context(), clientInvoiceOneQuery, c.EngagementID, invoiceID), practice)
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
