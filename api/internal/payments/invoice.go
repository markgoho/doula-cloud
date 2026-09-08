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
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/staffauth"
)

// invoicePageSize is the fixed number of Invoices GetInvoicesHandler
// returns per page -- mirrors message.pageSize's "fixed size is enough
// for paginated to be true" reasoning.
const invoicePageSize = 30

// invoiceStatusOpen is the invoices.status value shared by every read and
// write site in this package that tests for "billed and not yet
// collected" -- named once so goconst's repeat threshold, crossed once
// #271 added a second reader (manual_payment.go), doesn't see the same
// literal typed twice.
const invoiceStatusOpen = "open"

// InvoiceView is one Invoice, as returned by both PostInvoiceHandler (the
// row just created) and GetInvoicesHandler (a page of existing rows).
//
// Reference and BillingMode are #271's additions: Reference is a
// human-readable identifier a check "for invoice ___" can be matched
// against on paper (a by-hand Invoice's own per-Practice sequence, or
// Stripe's own `number`), and BillingMode says which rail the Invoice was
// born on -- an Invoice keeps the rail it was raised under even if the
// Practice's own billing_mode later changes, so this is read off the row
// itself, never off the Practice's current setting.
type InvoiceView struct {
	ID          string     `json:"id"`
	ContractID  string     `json:"contractId"`
	Status      string     `json:"status"`
	AmountCents int64      `json:"amountCents"`
	Currency    string     `json:"currency"`
	CreatedAt   time.Time  `json:"createdAt"`
	PaidAt      *time.Time `json:"paidAt,omitempty"`
	Reference   string     `json:"reference"`
	BillingMode string     `json:"billingMode"`
}

// CreateInvoiceRequest is the body of a POST to PostInvoiceHandler.
// #947 removed the amountCents a caller used to supply: an Invoice's
// amount now derives entirely from the Contract it bills
// (contracts.amount_cents, #967), so no request body -- an Owner's
// included -- can make an Invoice carry a figure that disagrees with the
// signed Contract. There is no description/line-item field either --
// every Invoice's line item and statement descriptor is
// InvoiceLineItemDescription, unconditionally.
//
// BillingMode is read only the first time this Practice ever raises an
// Invoice -- resolveBillingMode ignores it once a Practice's billing_mode
// is already set (#271).
type CreateInvoiceRequest struct {
	BillingMode *string `json:"billingMode,omitempty"`
}

// billingModeOf reports the billing rail a row belongs to, straight off
// its own stored stripe_invoice_id -- NULL means by-hand, following the
// migration's own "a NULL is the flag; there is no second column" rule.
func billingModeOf(stripeInvoiceID sql.NullString) string {
	if stripeInvoiceID.Valid {
		return string(BillingModeStripe)
	}
	return string(BillingModeByHand)
}

// MsgClientsCannotPay is PostInvoiceHandler's refusal (#270) when this
// Practice cannot yet take a Client's card payment -- Stripe Connect
// either isn't linked at all or its card_payments capability isn't
// active. A fact about the Practice and the role that clears it, not a
// refusal aimed at whoever happened to press Create Invoice: #78's old
// 200 gate response routed an Owner into the connect flow and told a
// non-Owner to ask one, both in the words of a person being refused
// something. The frontend now carries that same routing decision itself,
// from EngagementDetail.ClientsCanPay -- a standing fact read before the
// form is ever shown -- so this is only reachable by a caller that
// bypasses the UI (or a race against a webhook mid-flight), and needs no
// role-specific wording of its own.
const MsgClientsCannotPay = "Clients cannot pay this Practice yet. A Practice Owner has to connect Stripe before an Invoice can be sent."

// ListInvoicesResponse is the standard cursor-pagination envelope from
// docs/api-design.md section 4, mirroring message.ListResponse.
type ListInvoicesResponse struct {
	Items      []InvoiceView `json:"items"`
	NextCursor *string       `json:"nextCursor,omitempty"`
	HasMore    bool          `json:"hasMore"`
}

// PostInvoiceHandler creates an Invoice against :engagementId's current
// Contract, for the amount that Contract itself carries
// (contracts.amount_cents, #967) -- no caller, Owner included, can make
// it carry a different figure (#947). Raising stays open to any Staff
// with reach, no role gate (matching Contract's default, #68), mounted
// with attaching=true so an unattached contractor 404s instead.
//
// #275: the Contract must be billable at all, per contracts.TransitionBill
// -- Signed and still in force. A Draft, a Sent (unsigned), or a Voided
// Contract 409s here, before anything else is checked.
//
// #271 then branches on the Practice's billing_mode, resolved by
// resolveBillingMode (which also handles the "ask once, inline" case
// where none is set yet). On BillingModeByHand, createByHandInvoice
// raises the Invoice directly -- open, no Stripe call, no Connect gate,
// no Client-email requirement (amending #430: a by-hand Invoice mails
// nothing, so there is nothing an absent email would break). On
// BillingModeStripe, the pre-#271 flow runs unchanged: the
// clients-can-pay gate (#270), the no-email refusal (#430), and Stripe's
// own Create + Finalize Invoice calls.
func PostInvoiceHandler(client Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, engagementID, contractID, contractStatus, amountCents, ok := resolveInvoiceEngagement(w, r)
		if !ok {
			return
		}
		if billable, refusal := contracts.TransitionBill.Check(contractStatus); !billable {
			apierr.WriteError(w, refusal, http.StatusConflict)
			return
		}

		var req CreateInvoiceRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}

		staffID, _ := staffauth.StaffID(r.Context())

		mode, billingModeAlreadySet, err := resolveBillingMode(r.Context(), tx, practiceID, req.BillingMode)
		if errors.Is(err, errBillingModeRequired) {
			apierr.Write(w, http.StatusUnprocessableEntity, apierr.CodeFailedPrecondition, MsgBillingModeRequired, nil)
			return
		}
		if errors.Is(err, errBillingModeInvalid) {
			apierr.WriteError(w, `billingMode must be "stripe" or "by_hand"`, http.StatusBadRequest)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		var view InvoiceView
		if mode == BillingModeByHand {
			view, err = createByHandInvoice(r.Context(), tx, practiceID, contractID, amountCents)
		} else {
			view, err = createStripeInvoice(r.Context(), tx, client, practiceID, engagementID, contractID, amountCents)
		}
		if errors.Is(err, errClientNoEmail) {
			apierr.WriteError(w, "this client has no email on file -- add one before invoicing her", http.StatusUnprocessableEntity)
			return
		}
		if errors.Is(err, errClientsCannotPay) {
			apierr.Write(w, http.StatusConflict, apierr.CodeFailedPrecondition, MsgClientsCannotPay, nil)
			return
		}
		if err != nil {
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		// Only persist a first-time billing mode once the Invoice it was
		// requested for has actually been created -- see
		// resolveBillingMode's comment on why this cannot happen earlier.
		if !billingModeAlreadySet {
			if err := setBillingMode(r.Context(), tx, practiceID, mode, staffID); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
		}

		diff, err := json.Marshal(map[string]int64{"amountCents": amountCents})
		if err != nil {
			// coverage:ignore reason: a map of one int64 always marshals cleanly, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
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
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		apierr.WriteJSON(w, http.StatusCreated, view)
	})
}

// errClientsCannotPay is createStripeInvoice's refusal (#270) when this
// Practice cannot yet take a Client's card payment.
var errClientsCannotPay = errors.New("payments: clients cannot pay this practice yet")

// createByHandInvoice raises a by-hand Invoice (#271): open immediately,
// no Stripe call, no Connect gate, no Client-email requirement. Its
// reference is claimed from the Practice's own per-Practice sequence,
// atomically, so two concurrent by-hand Invoices can never collide.
func createByHandInvoice(ctx context.Context, tx *sql.Tx, practiceID, contractID string, amountCents int64) (InvoiceView, error) {
	var seq int
	if err := tx.QueryRowContext(ctx,
		`UPDATE practices SET next_invoice_sequence = next_invoice_sequence + 1 WHERE id = $1 RETURNING next_invoice_sequence - 1`,
		practiceID,
	).Scan(&seq); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return InvoiceView{}, fmt.Errorf("payments: claim invoice sequence: %w", err)
	}
	reference := fmt.Sprintf("INV-%04d", seq)

	var invoiceID string
	var createdAt time.Time
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO invoices (practice_id, contract_id, status, amount_cents, currency, reference)
		 VALUES ($1, $2, 'open', $3, 'usd', $4) RETURNING id, created_at`,
		practiceID, contractID, amountCents, reference,
	).Scan(&invoiceID, &createdAt); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return InvoiceView{}, fmt.Errorf("payments: create by-hand invoice: %w", err)
	}

	return InvoiceView{
		ID:          invoiceID,
		ContractID:  contractID,
		Status:      invoiceStatusOpen,
		AmountCents: amountCents,
		Currency:    "usd",
		CreatedAt:   createdAt,
		Reference:   reference,
		BillingMode: string(BillingModeByHand),
	}, nil
}

// createStripeInvoice is the pre-#271 Stripe-backed flow, unchanged in
// substance: the clients-can-pay gate, the Client-email requirement, and
// Stripe's own Create + Finalize Invoice calls. The invoices row is
// inserted 'draft' with its reference temporarily set to the Stripe
// invoice id (never NULL, satisfying the NOT NULL reference column)
// before FinalizeInvoice is called, so a Stripe-side failure at that
// point still leaves a persisted Doula Cloud record rather than an
// Invoice that exists on Stripe with no local row -- same fail-safe
// property the pre-#271 code had. Once Finalize succeeds, the row is
// updated to 'open' with Stripe's own human-readable `number` as its
// real reference.
func createStripeInvoice(ctx context.Context, tx *sql.Tx, client Client, practiceID, engagementID, contractID string, amountCents int64) (InvoiceView, error) {
	canPay, err := ClientsCanPay(ctx, tx, practiceID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return InvoiceView{}, fmt.Errorf("payments: check clients can pay: %w", err)
	}
	if !canPay {
		return InvoiceView{}, errClientsCannotPay
	}

	accountID, err := fetchConnectAccountID(ctx, tx, practiceID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return InvoiceView{}, err
	}

	clientID, clientName, clientEmail, err := fetchClientContact(ctx, tx, engagementID)
	if err != nil {
		return InvoiceView{}, err
	}

	staffID, _ := staffauth.StaffID(ctx)
	stripeCustomerID, err := resolveStripeCustomer(ctx, tx, client, stripeCustomerFor{
		PracticeID: practiceID,
		ClientID:   clientID,
		AccountID:  accountID,
		Email:      clientEmail,
		Name:       clientName,
		StaffID:    staffID,
	})
	if err != nil {
		return InvoiceView{}, err
	}

	stripeInvoiceID, err := client.CreateInvoice(ctx, accountID, stripeCustomerID, InvoiceLineItemDescription, amountCents)
	if err != nil {
		return InvoiceView{}, fmt.Errorf("payments: create stripe invoice: %w", err)
	}

	var invoiceID string
	var createdAt time.Time
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO invoices (practice_id, contract_id, stripe_invoice_id, stripe_customer_id, status, amount_cents, currency, reference)
		 VALUES ($1, $2, $3, $4, 'draft', $5, 'usd', $3) RETURNING id, created_at`,
		practiceID, contractID, stripeInvoiceID, stripeCustomerID, amountCents,
	).Scan(&invoiceID, &createdAt); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return InvoiceView{}, fmt.Errorf("payments: create stripe invoice row: %w", err)
	}

	// A draft invoices row is now persisted (staffauth.Middleware commits
	// the request-scoped tx regardless of the status this handler writes)
	// even if FinalizeInvoice below fails.
	hostedInvoiceURL, number, err := client.FinalizeInvoice(ctx, accountID, stripeInvoiceID)
	_ = hostedInvoiceURL // not yet surfaced anywhere; see #78's own discard of it
	if err != nil {
		return InvoiceView{}, fmt.Errorf("payments: finalize stripe invoice: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE invoices SET status = 'open', reference = $2 WHERE id = $1`, invoiceID, number,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return InvoiceView{}, fmt.Errorf("payments: finalize stripe invoice row: %w", err)
	}

	return InvoiceView{
		ID:          invoiceID,
		ContractID:  contractID,
		Status:      invoiceStatusOpen,
		AmountCents: amountCents,
		Currency:    "usd",
		CreatedAt:   createdAt,
		Reference:   number,
		BillingMode: string(BillingModeStripe),
	}, nil
}

// GetInvoicesHandler lists every Invoice ever created against
// :engagementId's Contract(s), newest first, cursor-paginated -- by
// Engagement rather than "the current Contract row" alone, so an
// Invoice's billing history survives a Contract Void-then-recreate
// (#72): a superseded, voided Contract's Invoices stay visible. Who may
// read it: Owner, Admin, and an employed Doula (ADR-0008's money row as
// amended by #282); RequireNotAmbientContractor refuses a contractor
// regardless of any attachment she holds, since her own fee is never
// this route. Must be mounted behind staffauth.Middleware.
func GetInvoicesHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireNotAmbientContractor(w, r)
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
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
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
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		resp := ListInvoicesResponse{Items: items, HasMore: hasMore}
		if hasMore {
			next := encodeInvoiceCursor(items[len(items)-1].CreatedAt, items[len(items)-1].ID)
			resp.NextCursor = &next
		}
		apierr.WriteJSON(w, http.StatusOK, resp)
	})
}

// resolveInvoiceEngagement resolves the request-scoped tx, Practice id,
// and :engagementId path segment, confirms the Engagement belongs to the
// current Practice, and fetches its current Contract's id, status and
// amount (the most recently created row, mirroring
// contracts.fetchContract's "most recent wins" rule) -- the shared
// prologue for PostInvoiceHandler. amountCents is the Contract's own
// amount_cents (#967): the only figure an Invoice is ever raised for
// (#947). Writes the appropriate error response itself and returns
// ok=false on any failure.
func resolveInvoiceEngagement(w http.ResponseWriter, r *http.Request) (tx *sql.Tx, practiceID, engagementID, contractID string, contractStatus contracts.Status, amountCents int64, ok bool) {
	tx, practiceID, ok = staffauth.RequireTx(w, r)
	// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
	if !ok {
		return nil, "", "", "", "", 0, false
	}

	engagementID = r.PathValue("engagementId")
	if !staffauth.ParseUUID(w, "engagement", engagementID) {
		return nil, "", "", "", "", 0, false
	}
	if err := requireEngagementAtPractice(r.Context(), tx, engagementID, practiceID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "engagement not found", http.StatusNotFound)
			return nil, "", "", "", "", 0, false
		}
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return nil, "", "", "", "", 0, false
	}

	contractID, contractStatus, amountCents, err := fetchCurrentContract(r.Context(), tx, engagementID)
	if errors.Is(err, sql.ErrNoRows) {
		apierr.WriteError(w, "no contract found for this engagement", http.StatusNotFound)
		return nil, "", "", "", "", 0, false
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return nil, "", "", "", "", 0, false
	}

	return tx, practiceID, engagementID, contractID, contractStatus, amountCents, true
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

// fetchCurrentContract returns the id, status and amount of
// engagementID's most recently created Contract row, or a wrapped
// sql.ErrNoRows if none exists yet. The status travels back with the id
// (rather than a separate query) because resolveInvoiceEngagement's only
// caller, PostInvoiceHandler, needs it immediately afterward to check
// contracts.TransitionBill -- #275. amountCents (contracts.amount_cents,
// #967) is the Contract's own real amount -- an Invoice is raised for
// exactly this figure and no other (#947).
func fetchCurrentContract(ctx context.Context, tx *sql.Tx, engagementID string) (id string, status contracts.Status, amountCents int64, err error) {
	var rawStatus string
	err = tx.QueryRowContext(ctx,
		`SELECT id, status, amount_cents FROM contracts WHERE engagement_id = $1 ORDER BY created_at DESC LIMIT 1`, engagementID,
	).Scan(&id, &rawStatus, &amountCents)
	// coverage:ignore reason: the sql.ErrNoRows branch is exercised by unit tests; a non-ErrNoRows DB failure here is not
	if err != nil {
		return "", "", 0, fmt.Errorf("payments: fetch current contract: %w", err)
	}
	return id, contracts.Status(rawStatus), amountCents, nil
}

// fetchConnectAccountID reads practiceID's stored Stripe Connect account
// id, for the Stripe API calls PostInvoiceHandler makes once ClientsCanPay
// has already confirmed card_payments is active -- which the webhook that
// writes that column only ever does by matching an existing
// stripe_connect_account_id (see PostAccountWebhookHandler), so a null
// account id here should be unreachable. Nothing in the schema enforces
// that pairing, though, so this still checks rather than trusting it:
// erroring here is cheap, and the alternative is calling Stripe with an
// empty account id.
func fetchConnectAccountID(ctx context.Context, tx *sql.Tx, practiceID string) (accountID string, err error) {
	var acct sql.NullString
	if err := tx.QueryRowContext(ctx,
		`SELECT stripe_connect_account_id FROM practices WHERE id = $1`, practiceID,
	).Scan(&acct); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", fmt.Errorf("payments: fetch connect account: %w", err)
	}
	if !acct.Valid {
		return "", fmt.Errorf("payments: card_payments active with no connect account linked for practice %s", practiceID)
	}
	return acct.String, nil
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

// listInvoicesQuery and listInvoicesAfterQuery share the same column list
// and JOIN; the only difference is the cursor's WHERE clause and LIMIT
// placeholder position -- mirrors message's listMessagesQuery /
// listMessagesAfterQuery split. Joined through contracts (rather than a
// direct contract_id = $1 filter) so an Invoice created against a
// since-voided Contract still lists under the Engagement that Contract
// belonged to.
const listInvoicesQuery = `SELECT i.id, i.contract_id, i.status, i.amount_cents, i.currency, i.created_at, i.paid_at, i.reference, i.stripe_invoice_id
	FROM invoices i
	JOIN contracts c ON c.id = i.contract_id
	WHERE c.engagement_id = $1
	ORDER BY i.created_at DESC, i.id DESC LIMIT $2`

const listInvoicesAfterQuery = `SELECT i.id, i.contract_id, i.status, i.amount_cents, i.currency, i.created_at, i.paid_at, i.reference, i.stripe_invoice_id
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
		var stripeInvoiceID sql.NullString
		if err := rows.Scan(&it.ID, &it.ContractID, &it.Status, &it.AmountCents, &it.Currency, &it.CreatedAt, &paidAt, &it.Reference, &stripeInvoiceID); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, false, fmt.Errorf("payments: scan invoice row: %w", err)
		}
		if paidAt.Valid {
			it.PaidAt = &paidAt.Time
		}
		it.BillingMode = billingModeOf(stripeInvoiceID)
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
