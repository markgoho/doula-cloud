package payments

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/staffauth"
)

// invoicePageSize is the fixed number of Invoices GetInvoicesHandler
// returns per page -- mirrors message.pageSize's "fixed size is enough
// for paginated to be true" reasoning.
const invoicePageSize = 30

// InvoiceView is one Invoice, as returned by both PostInvoiceHandler (the
// row just created) and GetInvoicesHandler (a page of existing rows).
type InvoiceView struct {
	ID          string     `json:"id"`
	ContractID  string     `json:"contractId"`
	Status      string     `json:"status"`
	AmountCents int64      `json:"amountCents"`
	Currency    string     `json:"currency"`
	CreatedAt   time.Time  `json:"createdAt"`
	PaidAt      *time.Time `json:"paidAt,omitempty"`
}

// CreateInvoiceRequest is the body of a POST to PostInvoiceHandler: the
// amount Staff agreed with the Client for this Engagement. There is no
// description/line-item field -- every Invoice's line item and statement
// descriptor is InvoiceLineItemDescription, unconditionally.
type CreateInvoiceRequest struct {
	AmountCents int64 `json:"amountCents"`
}

// PostInvoiceResponse is the body of PostInvoiceHandler's response. Per
// #78's lazy-connect-prompt rule, a single 200 response covers three
// distinct outcomes the frontend switches on: ConnectRequired true with
// IsOwner true (route the Owner into the #79 connect flow), ConnectRequired
// true with IsOwner false (show the static "ask an Owner" message), or
// ConnectRequired false with Invoice set (the Invoice was created).
type PostInvoiceResponse struct {
	ConnectRequired bool         `json:"connectRequired"`
	IsOwner         bool         `json:"isOwner,omitempty"`
	Invoice         *InvoiceView `json:"invoice,omitempty"`
}

// ListInvoicesResponse is the standard cursor-pagination envelope from
// docs/api-design.md section 4, mirroring message.ListResponse.
type ListInvoicesResponse struct {
	Items      []InvoiceView `json:"items"`
	NextCursor *string       `json:"nextCursor,omitempty"`
	HasMore    bool          `json:"hasMore"`
}

// PostInvoiceHandler creates an Invoice against :engagementId's current
// Contract for the amount Staff supplies -- open to any Staff with
// practice access, no assigned-staff or Owner gating (matching Contract's
// default, #68). If the Practice has no Stripe Connect account linked yet
// (practices.stripe_connect_account_id is null), no Invoice is created and
// a 200 gate response is returned instead: an Owner gets ConnectRequired
// so the frontend can route them into the #79 connect flow, a non-Owner
// gets the same flag so the frontend can show the static "ask an Owner"
// message. Must be mounted behind staffauth.Middleware.
func PostInvoiceHandler(client Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, engagementID, contractID, ok := resolveInvoiceEngagement(w, r)
		if !ok {
			return
		}

		accountID, connected, err := fetchConnectAccount(r.Context(), tx, practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !connected {
			reader, has := staffauth.ReaderFrom(r.Context())
			if !has {
				// coverage:ignore reason: staffauth.Middleware always places a Reader on context before this handler runs
				apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusOK, PostInvoiceResponse{ConnectRequired: true, IsOwner: reader.Has("owner")})
			return
		}

		var req CreateInvoiceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierr.WriteError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.AmountCents <= 0 {
			apierr.WriteError(w, "amountCents must be greater than zero", http.StatusBadRequest)
			return
		}

		clientID, clientName, clientEmail, err := fetchClientContact(r.Context(), tx, engagementID)
		if errors.Is(err, errClientNoEmail) {
			apierr.WriteError(w, "this client has no email on file -- add one before invoicing her", http.StatusUnprocessableEntity)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests -- the Contract already resolved above implies the Engagement/Client rows exist
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		staffID, _ := staffauth.StaffID(r.Context())
		stripeCustomerID, err := resolveStripeCustomer(r.Context(), tx, client, stripeCustomerFor{
			PracticeID: practiceID,
			ClientID:   clientID,
			AccountID:  accountID,
			Email:      clientEmail,
			Name:       clientName,
			StaffID:    staffID,
		})
		if err != nil {
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		stripeInvoiceID, err := client.CreateInvoice(r.Context(), accountID, stripeCustomerID, InvoiceLineItemDescription, req.AmountCents)
		if err != nil {
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		var invoiceID string
		var createdAt time.Time
		if err := tx.QueryRowContext(r.Context(),
			`INSERT INTO invoices (practice_id, contract_id, stripe_invoice_id, stripe_customer_id, status, amount_cents, currency)
			 VALUES ($1, $2, $3, $4, 'draft', $5, 'usd') RETURNING id, created_at`,
			practiceID, contractID, stripeInvoiceID, stripeCustomerID, req.AmountCents,
		).Scan(&invoiceID, &createdAt); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		// A draft invoices row is now persisted (staffauth.Middleware
		// commits the request-scoped tx regardless of the status this
		// handler writes) even if FinalizeInvoice below fails -- so a
		// Stripe-side failure here never leaves an Invoice that exists on
		// Stripe with no corresponding Doula Cloud record.
		if _, err := client.FinalizeInvoice(r.Context(), accountID, stripeInvoiceID); err != nil {
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if _, err := tx.ExecContext(r.Context(), `UPDATE invoices SET status = 'open' WHERE id = $1`, invoiceID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		diff, err := json.Marshal(map[string]int64{"amountCents": req.AmountCents})
		if err != nil {
			// coverage:ignore reason: a map of one int64 always marshals cleanly, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := activity.Record(r.Context(), tx, activity.Entry{
			PracticeID:  practiceID,
			SubjectKind: activity.SubjectEngagement,
			SubjectID:   engagementID,
			Action:      string(activity.ActionInvoiceRaised),
			Diff:        diff,
			Actor:       activity.StaffActor(staffID),
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusCreated, PostInvoiceResponse{
			Invoice: &InvoiceView{
				ID:          invoiceID,
				ContractID:  contractID,
				Status:      "open",
				AmountCents: req.AmountCents,
				Currency:    "usd",
				CreatedAt:   createdAt,
			},
		})
	})
}

// GetInvoicesHandler lists every Invoice ever created against
// :engagementId's Contract(s), newest first, cursor-paginated -- by
// Engagement rather than "the current Contract row" alone, so an
// Invoice's billing history survives a Contract Void-then-recreate
// (#72): a superseded, voided Contract's Invoices stay visible. Must be
// mounted behind staffauth.Middleware.
func GetInvoicesHandler() http.Handler {
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
		if err := requireEngagementAtPractice(r.Context(), tx, engagementID, practiceID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				apierr.WriteError(w, "engagement not found", http.StatusNotFound)
				return
			}
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
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

		items, hasMore, err := listInvoices(r.Context(), tx, engagementID, after)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		resp := ListInvoicesResponse{Items: items, HasMore: hasMore}
		if hasMore {
			next := encodeInvoiceCursor(items[len(items)-1].CreatedAt, items[len(items)-1].ID)
			resp.NextCursor = &next
		}
		writeJSON(w, http.StatusOK, resp)
	})
}

// resolveInvoiceEngagement resolves the request-scoped tx, Practice id,
// and :engagementId path segment, confirms the Engagement belongs to the
// current Practice, and fetches its current Contract id (the most
// recently created row, mirroring contracts.fetchContract's "most recent
// wins" rule) -- the shared prologue for PostInvoiceHandler. Writes the
// appropriate error response itself and returns ok=false on any failure.
func resolveInvoiceEngagement(w http.ResponseWriter, r *http.Request) (tx *sql.Tx, practiceID, engagementID, contractID string, ok bool) {
	tx, practiceID, ok = staffauth.RequireTx(w, r)
	// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
	if !ok {
		return nil, "", "", "", false
	}

	engagementID = r.PathValue("engagementId")
	if !staffauth.ParseUUID(w, "engagement", engagementID) {
		return nil, "", "", "", false
	}
	if err := requireEngagementAtPractice(r.Context(), tx, engagementID, practiceID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "engagement not found", http.StatusNotFound)
			return nil, "", "", "", false
		}
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return nil, "", "", "", false
	}

	contractID, err := fetchCurrentContractID(r.Context(), tx, engagementID)
	if errors.Is(err, sql.ErrNoRows) {
		apierr.WriteError(w, "no contract found for this engagement", http.StatusNotFound)
		return nil, "", "", "", false
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return nil, "", "", "", false
	}

	return tx, practiceID, engagementID, contractID, true
}

// requireEngagementAtPractice confirms engagementID exists and belongs to
// practiceID, returning sql.ErrNoRows if not -- mirrors
// contracts.requireEngagementAtPractice and message's own copy.
func requireEngagementAtPractice(ctx context.Context, tx *sql.Tx, engagementID, practiceID string) error {
	var exists bool
	err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM engagements WHERE id = $1 AND practice_id = $2)`,
		engagementID, practiceID,
	).Scan(&exists)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return fmt.Errorf("payments: check engagement at practice: %w", err)
	}
	if !exists {
		return sql.ErrNoRows
	}
	return nil
}

// fetchCurrentContractID returns the id of engagementID's most recently
// created Contract row, or a wrapped sql.ErrNoRows if none exists yet.
func fetchCurrentContractID(ctx context.Context, tx *sql.Tx, engagementID string) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM contracts WHERE engagement_id = $1 ORDER BY created_at DESC LIMIT 1`, engagementID,
	).Scan(&id)
	// coverage:ignore reason: the sql.ErrNoRows branch is exercised by unit tests; a non-ErrNoRows DB failure here is not
	if err != nil {
		return "", fmt.Errorf("payments: fetch current contract: %w", err)
	}
	return id, nil
}

// fetchConnectAccount reads practiceID's stored Stripe Connect account id,
// reporting connected=false (rather than an error) when none is linked
// yet -- the lazy-connect-check case #78's ticket body describes.
func fetchConnectAccount(ctx context.Context, tx *sql.Tx, practiceID string) (accountID string, connected bool, err error) {
	var acct sql.NullString
	if err := tx.QueryRowContext(ctx,
		`SELECT stripe_connect_account_id FROM practices WHERE id = $1`, practiceID,
	).Scan(&acct); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", false, fmt.Errorf("payments: fetch connect account: %w", err)
	}
	return acct.String, acct.Valid, nil
}

// errClientNoEmail is fetchClientContact's refusal when the Engagement's
// Client has no email on file. ADR-0017 relaxed clients.email to
// nullable; Stripe invoicing must refuse rather than send to an empty
// string.
var errClientNoEmail = errors.New("payments: client has no email on file")

// fetchClientContact resolves engagementID's Client legal name and email
// -- the only Client-identifying fields an Invoice ever carries, per
// #78's no-PHI-to-Stripe rule (no visit, Care Plan, Birth Plan, or other
// clinical content). name uses client.LegalName -- the document name
// Stripe invoicing reads, per ADR-0017's read table.
func fetchClientContact(ctx context.Context, tx *sql.Tx, engagementID string) (clientID, name, email string, err error) {
	var givenName string
	var familyName, clientEmail sql.NullString
	err = tx.QueryRowContext(ctx,
		`SELECT c.id, c.given_name, c.family_name, c.email
		 FROM clients c JOIN engagements e ON e.client_id = c.id WHERE e.id = $1`,
		engagementID,
	).Scan(&clientID, &givenName, &familyName, &clientEmail)
	// coverage:ignore reason: DB query failure, not exercised by unit tests -- resolveInvoiceEngagement already proved the Engagement (and therefore its Client) exists
	if err != nil {
		return "", "", "", fmt.Errorf("payments: fetch client contact: %w", err)
	}
	if !clientEmail.Valid || clientEmail.String == "" {
		return "", "", "", errClientNoEmail
	}
	return clientID, client.LegalName(givenName, familyName.String), clientEmail.String, nil
}

// stripeCustomerFor is everything resolveStripeCustomer needs to find, or
// failing that make, one Client's Stripe Customer on one connected
// account. Grouped into a struct rather than seven positional arguments,
// four of which are strings that would sit next to each other.
type stripeCustomerFor struct {
	PracticeID string
	ClientID   string
	AccountID  string
	Email      string
	Name       string
	// StaffID is who is raising the Invoice that needed the Customer --
	// recorded on the mapping row as who caused it to exist. Empty only
	// where staffauth did not set one, which the middleware guarantees it
	// does.
	StaffID string
}

// resolveStripeCustomer returns the Stripe Customer that bills spec.ClientID
// on spec.AccountID, creating it at Stripe and recording the mapping only
// when the Client has none there yet (#780). A Client has at most one
// Stripe Customer per connected account: her second Invoice bills the
// Customer her first one made, and her whole billing history sits under
// one Customer rather than one per bill.
//
// Because the mapping is a row rather than something the product infers,
// a simulation run can write it first -- with a Customer it created
// against a Stripe test clock -- and this finds a Customer and creates
// nothing. That is why no test-only parameter exists on any api/ path.
//
// The Client row is locked for the rest of the request's transaction
// first, so two concurrent Invoices for the same Client cannot both miss
// the mapping and both create a Customer -- the same race-prevention
// shape as billing.PostPurchaseHandler's Practice lock.
func resolveStripeCustomer(ctx context.Context, tx *sql.Tx, stripeClient Client, spec stripeCustomerFor) (string, error) {
	if _, err := tx.ExecContext(ctx, `SELECT id FROM clients WHERE id = $1 FOR UPDATE`, spec.ClientID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", fmt.Errorf("payments: lock client for customer resolution: %w", err)
	}

	var customerID string
	err := tx.QueryRowContext(ctx,
		`SELECT stripe_customer_id FROM client_stripe_customers
		  WHERE client_id = $1 AND stripe_account_id = $2`,
		spec.ClientID, spec.AccountID,
	).Scan(&customerID)
	if err == nil {
		return customerID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", fmt.Errorf("payments: read stripe customer mapping: %w", err)
	}

	customerID, err = stripeClient.CreateCustomer(ctx, spec.AccountID, spec.Email, spec.Name)
	if err != nil {
		return "", fmt.Errorf("payments: create stripe customer: %w", err)
	}

	var staffID sql.NullString
	if spec.StaffID != "" {
		staffID = sql.NullString{String: spec.StaffID, Valid: true}
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO client_stripe_customers
		     (practice_id, client_id, stripe_account_id, stripe_customer_id, created_by_staff_id)
		 VALUES ($1, $2, $3, $4, $5)`,
		spec.PracticeID, spec.ClientID, spec.AccountID, customerID, staffID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests -- the row above is locked, so the UNIQUE constraint cannot be raced
		return "", fmt.Errorf("payments: record stripe customer mapping: %w", err)
	}
	return customerID, nil
}

// writeJSON encodes body as the response, setting the Content-Type header
// and status first.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// coverage:ignore reason: response encoding failure, not exercised by unit tests
	if err := json.NewEncoder(w).Encode(body); err != nil {
		apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
	}
}

// listInvoicesQuery and listInvoicesAfterQuery share the same column list
// and JOIN; the only difference is the cursor's WHERE clause and LIMIT
// placeholder position -- mirrors message's listMessagesQuery /
// listMessagesAfterQuery split. Joined through contracts (rather than a
// direct contract_id = $1 filter) so an Invoice created against a
// since-voided Contract still lists under the Engagement that Contract
// belonged to.
const listInvoicesQuery = `SELECT i.id, i.contract_id, i.status, i.amount_cents, i.currency, i.created_at, i.paid_at
	FROM invoices i
	JOIN contracts c ON c.id = i.contract_id
	WHERE c.engagement_id = $1
	ORDER BY i.created_at DESC, i.id DESC LIMIT $2`

const listInvoicesAfterQuery = `SELECT i.id, i.contract_id, i.status, i.amount_cents, i.currency, i.created_at, i.paid_at
	FROM invoices i
	JOIN contracts c ON c.id = i.contract_id
	WHERE c.engagement_id = $1 AND (i.created_at, i.id) < ($2, $3)
	ORDER BY i.created_at DESC, i.id DESC LIMIT $4`

// listInvoices fetches one page of Invoices under engagementID, filtered
// explicitly on top of the RLS scoping staffauth.Middleware already set
// up on tx -- the app layer's own filter, so a bug in either one alone
// can't leak rows.
func listInvoices(ctx context.Context, tx *sql.Tx, engagementID string, after *invoiceCursor) ([]InvoiceView, bool, error) {
	var rows *sql.Rows
	var err error
	if after != nil {
		rows, err = tx.QueryContext(ctx, listInvoicesAfterQuery, engagementID, after.createdAt, after.invoiceID, invoicePageSize+1)
	} else {
		rows, err = tx.QueryContext(ctx, listInvoicesQuery, engagementID, invoicePageSize+1)
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return nil, false, fmt.Errorf("payments: list invoices: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := []InvoiceView{}
	for rows.Next() {
		var it InvoiceView
		var paidAt sql.NullTime
		if err := rows.Scan(&it.ID, &it.ContractID, &it.Status, &it.AmountCents, &it.Currency, &it.CreatedAt, &paidAt); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, false, fmt.Errorf("payments: scan invoice row: %w", err)
		}
		if paidAt.Valid {
			it.PaidAt = &paidAt.Time
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, false, fmt.Errorf("payments: iterate invoice rows: %w", err)
	}

	hasMore := len(items) > invoicePageSize
	if hasMore {
		items = items[:invoicePageSize]
	}
	return items, hasMore, nil
}

// invoiceCursor is a page boundary: the (created_at, id) tuple of the
// last Invoice on the previous page, matching the DESC tiebreak
// listInvoices orders by -- mirrors message.messageCursor.
type invoiceCursor struct {
	createdAt time.Time
	invoiceID string
}

// encodeInvoiceCursor packs a cursor as opaque base64 so callers never
// construct one by hand. The packing is pagecursor's, shared with offer
// and message.
func encodeInvoiceCursor(createdAt time.Time, invoiceID string) string {
	return pagecursor.Encode(createdAt, invoiceID)
}

// decodeInvoiceCursor reverses encodeInvoiceCursor, rejecting anything
// malformed rather than letting a bad cursor silently return the wrong
// page.
func decodeInvoiceCursor(s string) (invoiceCursor, error) {
	c, err := pagecursor.Decode(s)
	if err != nil {
		return invoiceCursor{}, fmt.Errorf("payments: decode invoice cursor: %w", err)
	}
	return invoiceCursor{createdAt: c.At, invoiceID: c.ID}, nil
}
