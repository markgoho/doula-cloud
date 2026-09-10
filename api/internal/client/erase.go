package client

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/clientkey"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
)

// ErasedGivenName is what a Client's given_name becomes on erasure.
// The column is NOT NULL and always has been -- a Client with no name at
// all is not a shape this schema allows -- so erasure replaces it rather
// than nulling it, and the replacement says plainly what happened rather
// than inventing a person. Every other identifying column goes to NULL,
// which is a state they already have for a Client whose intake was
// half-typed on a phone.
const ErasedGivenName = "Erased Client"

// erasureAct names one act client_erasure_outbox carries -- the Go side
// of the client_erasure_act enum. Each is somebody else's API, which is
// why none of them happens inside the erasure transaction.
type erasureAct string

const (
	actStripeCustomerDelete erasureAct = "stripe_customer_delete"
	actStripeRedactionJob   erasureAct = "stripe_redaction_job"
)

// StripeRedactionFloor is how long Stripe makes a platform wait before
// most transactions can be redacted (ADR-0027). It is Stripe's number,
// not a Doula Cloud policy, and erasure schedules the redaction job for
// this far past the Client's newest invoice rather than issuing one it
// already knows will fail validation.
const StripeRedactionFloor = 90 * 24 * time.Hour

// ErasureResponse is what the Owner gets back: when the erasure ran, and
// -- if any of her Stripe transactions is still inside Stripe's 90-day
// floor -- the date the Stripe half finishes. It is the same pair the
// detail read carries afterwards, answered here so the screen that ran
// the act does not have to re-fetch to say what happened.
type ErasureResponse struct {
	ErasedAt                  time.Time  `json:"erasedAt"`
	StripeRedactionEligibleAt *time.Time `json:"stripeRedactionEligibleAt,omitempty"`
	// StripeCustomersQueued counts every Customer of hers this erasure
	// queued for deletion, on her surviving record and on every record
	// merged into it (#813). Those cannot move at a merge --
	// client_stripe_customers carries no UPDATE grant -- so an Owner who
	// saw only the survivor's count would be told a smaller number than
	// the act actually covered.
	StripeCustomersQueued int `json:"stripeCustomersQueued"`
	// PortalAccountQueued says her portal link at this Practice was
	// removed -- deliberately not whether the Portal Account row itself
	// was deleted (#830). The login survives when another Practice's
	// un-erased Client still reaches it, and reporting that difference
	// here would tell this Practice that another Practice serves her,
	// which is the fact ADR-0015 keeps inside the portal.
	PortalAccountQueued bool `json:"portalAccountQueued"`
}

// erasureScope is the plaintext diff on the 'erased' activity row: what
// the act actually covered, in a form that stays readable forever. It
// deliberately names counts and dates, never a value that was erased --
// this row survives the shredding of her history precisely because it
// describes the act rather than her.
type erasureScope struct {
	Contracts       int `json:"contracts"`
	StripeCustomers int `json:"stripeCustomers"`
	// PortalAccount reads the same way ErasureResponse's own
	// PortalAccountQueued does: her portal link at this Practice was
	// removed, never whether the login itself survived because another
	// Practice still reaches it (#830).
	PortalAccount bool `json:"portalAccount"`
	// There is deliberately no session count here (#830). A session is
	// the person's, not a Practice's -- one reaches every Client she has
	// -- so erasure ends her sessions only when it is also taking the
	// login, and a count would therefore read 0 exactly when another
	// Practice still reaches her. An Owner who knows she was signed in an
	// hour ago could read that 0 as "somebody else serves her", which is
	// the fact ADR-0015 keeps inside the portal. What the sessions did is
	// a consequence of the login's fate, and the login's fate is not this
	// Practice's to be told.
	StripeRedactionEligibleAt *time.Time `json:"stripeRedactionEligibleAt,omitempty"`
	// PaymentNotes counts manually recorded Payments (#271) whose note
	// this erasure emptied -- the same "count, never the value" rule
	// erasureScope already applies to Contracts.
	PaymentNotes int `json:"paymentNotes"`
	// PaymentReversalReasons counts reversal rows (#945) whose reason this
	// erasure emptied -- a second personal-data surface on the same
	// payments table, counted separately from PaymentNotes because the two
	// columns are set on different kinds of row (note on 'manual', reason
	// on 'reversal') and neither implies the other.
	PaymentReversalReasons int `json:"paymentReversalReasons"`
	// AbsorbedRecords counts the tombstoned records this erasure reached
	// through her merged_into chain (#813): each one redacted, its Stripe
	// Customers queued for deletion, and its own data key shredded. It is
	// the plaintext answer to the one question a merged Client's erasure
	// otherwise could not answer -- "was the other record erased too?" --
	// asked at exactly the moment both audit trails become unreadable.
	AbsorbedRecords int `json:"absorbedRecords"`
}

// EraseHandler erases one Client's personal data at the Owner's
// instruction -- ADR-0027, the whole act, in one transaction plus an
// outbox for the three parts that are somebody else's API.
//
// Owner-only: not Owner-or-Admin, which gates most things that reshape a
// Practice. Erasure is the one act in the product that destroys a fact,
// so it sits in the same seat as the MFA switch and the vouch.
//
// Refused, with 409 and the invoice ids named, while any of her invoices
// is still draft or open: a non-terminal transaction cannot be redacted
// by Stripe, and deleting her Customer underneath an unpaid invoice
// would leave the Practice unable to collect on work it did. Refused,
// also with 409, if she is already erased -- the act is irreversible and
// running it twice is a mistake worth naming rather than absorbing.
//
// The two refusals share a status and are told apart by their code, not
// by their prose (#692). Already erased is CONFLICT: the resource is
// already in the state the caller asked for. Unsettled invoices is
// FAILED_PRECONDITION: nothing about the Client conflicts, a condition
// on other resources is unmet -- the same reading payments/connect
// already gives a 409 that is not a resource conflict. Both codes are
// written explicitly rather than derived from the status, so neither can
// quietly collapse back onto the other.
//
// Must be mounted behind staffauth.Middleware.
//
// Owner-only is declared at the mount, not checked here (#1016,
// following #970's and #990's own move): the handler no longer calls
// staffauth.RequireOwner, because an Admin or a Doula is refused by the
// gate before this runs. Widening or narrowing this route means editing
// its role list in mount.go.
func EraseHandler(enq tasknudge.Enqueuer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		clientID := r.PathValue("clientId")
		if !staffauth.ParseUUID(w, "client", clientID) {
			return
		}

		erasedAt, ok := lookupErasedAt(w, r, tx, clientID, practiceID, true)
		if !ok {
			return
		}
		if erasedAt != nil {
			apierr.Write(w, http.StatusConflict, apierr.CodeConflict,
				"this client's data has already been erased", nil)
			return
		}

		unsettled, err := unsettledInvoiceSummaries(r.Context(), tx, clientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if len(unsettled) > 0 {
			ids := make([]string, len(unsettled))
			for i, s := range unsettled {
				ids[i] = s.stripeInvoiceID
			}
			apierr.Write(w, http.StatusConflict, apierr.CodeFailedPrecondition,
				"cannot erase a client with unsettled invoices: settle or void "+strings.Join(ids, ", ")+" first",
				nil)
			return
		}

		staffID, _ := staffauth.StaffID(r.Context())
		out, err := Erase(r.Context(), tx, practiceID, clientID, activity.StaffActor(staffID), time.Now().UTC())
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		tasknudge.Register(r.Context(), tasknudge.Fire(enq, tasknudge.ClientErasure))

		apierr.WriteJSON(w, http.StatusOK, out)
	})
}

// lookupErasedAt reads clientID's erased_at, scoped to practiceID, and
// writes the response itself on a miss or a query failure -- the same
// ok-bool idiom staffauth.RequireOwner uses, so a caller just does
// `if !ok { return }`. forUpdate locks the row: EraseHandler needs the
// lock before it acts on what it reads; EraseEligibilityHandler's
// precheck acts on nothing, so it reads plain. Shared so the two
// handlers' identical not-found/error handling lives in one place.
func lookupErasedAt(w http.ResponseWriter, r *http.Request, tx *sql.Tx, clientID, practiceID string, forUpdate bool) (erasedAt *time.Time, ok bool) {
	query := `SELECT erased_at FROM clients WHERE id = $1 AND practice_id = $2`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var nullable sql.NullTime
	err := tx.QueryRowContext(r.Context(), query, clientID, practiceID).Scan(&nullable)
	if errors.Is(err, sql.ErrNoRows) {
		apierr.WriteError(w, "client not found", http.StatusNotFound)
		return nil, false
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return nil, false
	}
	if !nullable.Valid {
		return nil, true
	}
	return &nullable.Time, true
}

// EraseEligibility is what an Owner reads before she ever reaches #691's
// confirmation step: whether this Client is already erased, and -- if
// not -- the same unsettled-invoice fact EraseHandler's own 409 would
// otherwise be the only way to learn. UnsettledInvoices is always a
// (possibly empty) slice, never nil, so the frontend can check its
// length without a null guard.
type EraseEligibility struct {
	ErasedAt          *time.Time                `json:"erasedAt,omitempty"`
	UnsettledInvoices []UnsettledInvoiceSummary `json:"unsettledInvoices"`
}

// EraseEligibilityHandler answers #691's precheck: can this Client be
// erased right now, and if not, which invoices are in the way. Gated the
// same as EraseHandler itself (Owner-only) -- the invoice amounts it
// names are exactly what ADR-0008's read table reserves for Owner and
// Admin, and this is a precheck of an Owner-only act, not a wider read.
// Must be mounted behind staffauth.Middleware.
func EraseEligibilityHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireOwner(w, r)
		if !ok {
			// coverage:ignore reason: belt-and-braces -- client.Mount's own
			// OwnerOnly declaration (g.Get) already refuses a non-owner caller
			// before this handler runs, so !ok is unreachable through the real
			// mount. Since #1016 the erasure POST is declared the same way
			// (ir.ExemptGated with staffauth.OwnerOnly), so this in-handler
			// call is the redundant half of a pair, not the last line of
			// defense it once was.
			return
		}
		clientID := r.PathValue("clientId")
		if !staffauth.ParseUUID(w, "client", clientID) {
			return
		}

		erasedAt, ok := lookupErasedAt(w, r, tx, clientID, practiceID, false)
		if !ok {
			return
		}

		out := EraseEligibility{UnsettledInvoices: []UnsettledInvoiceSummary{}}
		if erasedAt != nil {
			out.ErasedAt = erasedAt
		} else {
			unsettled, err := unsettledInvoiceSummaries(r.Context(), tx, clientID)
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
			out.UnsettledInvoices = unsettled
		}

		apierr.WriteJSON(w, http.StatusOK, out)
	})
}

// Erase is the act itself, in the order the pieces depend on each other:
// the record is redacted, the outside-world work is enqueued, the
// erasure is recorded in plaintext, and only then is her key destroyed.
// The key goes last because recordErasure's own row is written through
// activity.Record directly rather than recordEvent -- but the ordering
// still matters for any future write site that seals, and reads more
// honestly: everything that needed her key has already happened.
//
// Exported so a Practice-deletion cascade (#871) can erase every Client
// under a Practice with the same act a single Owner-initiated erasure
// runs, passing activity.SystemActor() for a day-30 finalization rather
// than a Staff member's own id -- erase.go's caller (EraseHandler) is
// the only place a request-scoped Staff actor exists.
func Erase(ctx context.Context, tx *sql.Tx, practiceID, clientID string, actor activity.Actor, now time.Time) (ErasureResponse, error) {
	if err := redactRecord(ctx, tx, clientID, now); err != nil {
		// coverage:ignore reason: every step below fails only on a DB query failure, not exercised by unit tests
		return ErasureResponse{}, err
	}
	contracts, err := redactContractMergeFields(ctx, tx, clientID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return ErasureResponse{}, err
	}
	paymentNotes, err := redactPaymentNotes(ctx, tx, clientID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return ErasureResponse{}, err
	}
	paymentReversalReasons, err := redactPaymentReversalReasons(ctx, tx, clientID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return ErasureResponse{}, err
	}

	customers, eligibleAt, err := enqueueStripeErasure(ctx, tx, practiceID, clientID, now)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return ErasureResponse{}, err
	}
	absorbed, absorbedCustomers, absorbedEligibleAt, err := eraseAbsorbedRecords(ctx, tx, practiceID, clientID, now)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return ErasureResponse{}, err
	}
	if absorbedEligibleAt != nil && (eligibleAt == nil || absorbedEligibleAt.After(*eligibleAt)) {
		eligibleAt = absorbedEligibleAt
	}
	portalQueued, err := enqueuePortalErasure(ctx, tx, clientID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return ErasureResponse{}, err
	}

	scope := erasureScope{
		Contracts:                 contracts,
		StripeCustomers:           len(customers) + absorbedCustomers,
		PortalAccount:             portalQueued,
		StripeRedactionEligibleAt: eligibleAt,
		PaymentNotes:              paymentNotes,
		PaymentReversalReasons:    paymentReversalReasons,
		AbsorbedRecords:           absorbed,
	}
	if err := recordErasure(ctx, tx, practiceID, clientID, actor, scope); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return ErasureResponse{}, err
	}

	// The shredding. Every diff sealed under this key becomes permanently
	// unreadable here, and not one row of activity was touched to do it.
	if err := clientkey.Destroy(ctx, tx, clientID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return ErasureResponse{}, fmt.Errorf("client: destroy client key: %w", err)
	}

	return ErasureResponse{
		ErasedAt:                  now,
		StripeRedactionEligibleAt: eligibleAt,
		StripeCustomersQueued:     len(customers) + absorbedCustomers,
		PortalAccountQueued:       portalQueued,
	}, nil
}

// eraseAbsorbedRecords erases every record that was merged into
// survivorID (#813). A merge leaves the absorbed row standing -- a
// clients row is never deleted (ADR-0027) -- still holding her name, her
// email, her address, her own data key, and any Stripe Customer that was
// allocated against it before the merge. Erasing the woman has to reach
// all of it, and the survivor's own erasure is the only act that ever
// will: a tombstone has no screen of its own and no endpoint that
// accepts it.
//
// The work goes through redact_absorbed_client (00112) rather than
// redactRecord, because clients_update's USING clause (00080) carries
// merged_into IS NULL: app_runtime cannot write to a tombstone at all.
// That function is preconditioned on the destination already being
// erased, which redactRecord has done by the time Erase calls this.
//
// Her Stripe Customers stay where they are and are deleted from there.
// client_stripe_customers carries no UPDATE grant (00076: repointing a
// Stripe Customer is not an operation this product has), so the merge
// could not move them and did not pretend to.
//
// Both data keys are destroyed, which is the whole reason the merge left
// the second one alone. ADR-0022's activity table is append-only and
// ADR-0027 seals each Client's diffs under her own key, so the absorbed
// woman's history could never be re-sealed under the survivor's -- it
// stays a second sealed trail under one name until this shreds it.
func eraseAbsorbedRecords(ctx context.Context, tx *sql.Tx, practiceID, survivorID string, now time.Time) (records, customers int, eligibleAt *time.Time, err error) {
	ids, err := absorbedRecordIDs(ctx, tx, survivorID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return 0, 0, nil, err
	}
	for _, id := range ids {
		// It takes the record and nothing else. The replacement name and
		// the timestamp are the function's own, not this caller's: it is
		// the one door that can write a tombstone at all, so letting a
		// caller choose either value would be a door onto a write
		// app_runtime is otherwise forbidden to make.
		if _, err := tx.ExecContext(ctx,
			`SELECT redact_absorbed_client($1)`, id,
		); err != nil {
			// coverage:ignore reason: the function's own refusal needs a caller that redacts a tombstone whose destination is not erased, which Erase's ordering and absorbedRecordIDs' own ordering make unreachable
			return 0, 0, nil, fmt.Errorf("client: redact absorbed record: %w", err)
		}
		absorbedCustomers, absorbedEligibleAt, err := enqueueStripeErasure(ctx, tx, practiceID, id, now)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return 0, 0, nil, err
		}
		customers += len(absorbedCustomers)
		if absorbedEligibleAt != nil && (eligibleAt == nil || absorbedEligibleAt.After(*eligibleAt)) {
			eligibleAt = absorbedEligibleAt
		}
		if err := clientkey.Destroy(ctx, tx, id); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return 0, 0, nil, fmt.Errorf("client: destroy absorbed client key: %w", err)
		}
	}
	return len(ids), customers, eligibleAt, nil
}

// absorbedRecordIDs walks survivorID's merged_into chain outward,
// nearest first. Nothing refuses absorbing B into C while A is already
// merged into B, so a chain is a state the endpoint can produce and this
// has to be able to read.
//
// The order is what makes redact_absorbed_client's precondition work:
// that function admits a row only when the record it was merged into is
// already erased, so B (merged into the just-erased C) is admitted
// first, and redacting B stamps its own erased_at, which is what then
// admits A.
func absorbedRecordIDs(ctx context.Context, tx *sql.Tx, survivorID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx,
		absorbedChainCTE+`SELECT id FROM absorbed ORDER BY depth, id`,
		survivorID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("client: list absorbed records: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("client: scan absorbed record: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("client: iterate absorbed records: %w", err)
	}
	return ids, nil
}

// isErased reports whether clientID has already been erased -- the gate
// every other write on a Client reads.
func isErased(ctx context.Context, tx *sql.Tx, clientID string) (bool, error) {
	var erasedAt sql.NullTime
	if err := tx.QueryRowContext(ctx, `SELECT erased_at FROM clients WHERE id = $1`, clientID).Scan(&erasedAt); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests -- the row was already read by the caller
		return false, fmt.Errorf("client: read erased_at: %w", err)
	}
	return erasedAt.Valid, nil
}

// redactRecord empties every identifying column on her clients row and
// stamps erased_at. The row and its id survive untouched, so every
// record that names her by client_id keeps resolving -- the whole point
// of redacting in place rather than deleting (ADR-0027).
func redactRecord(ctx context.Context, tx *sql.Tx, clientID string, now time.Time) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE clients SET
			given_name = $2, family_name = NULL, preferred_name = NULL, email = NULL, phone = NULL,
			address_line1 = NULL, address_line2 = NULL, address_locality = NULL,
			address_region = NULL, address_postal_code = NULL, date_of_birth = NULL,
			field_values = '{}'::jsonb, erased_at = $3
		 WHERE id = $1`,
		clientID, ErasedGivenName, now,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("client: redact record: %w", err)
	}
	return nil
}

// redactContractMergeFields empties the structured values a Contract
// captured about her at render time -- the same shape as her own record,
// captured a second time, and so covered by the same in-place-redaction
// rule. The Contract's prose is deliberately left exactly as signed:
// ADR-0027's free-text limitation.
func redactContractMergeFields(ctx context.Context, tx *sql.Tx, clientID string) (int, error) {
	res, err := tx.ExecContext(ctx,
		`UPDATE contracts SET merge_field_values = '{}'::jsonb
		 WHERE engagement_id IN (SELECT id FROM engagements WHERE client_id = $1)
		   AND merge_field_values <> '{}'::jsonb`,
		clientID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return 0, fmt.Errorf("client: redact contract merge fields: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		// coverage:ignore reason: lib/pq always reports RowsAffected, not exercised by unit tests
		return 0, fmt.Errorf("client: count redacted contracts: %w", err)
	}
	return int(n), nil
}

// redactPaymentNotes empties the free-text note on every manually
// recorded Payment (#271) against one of clientID's Invoices -- a Staff
// member can type anything in there, including a check number that
// carries her name or address, so it is a personal-data surface the same
// way a Contract's merge fields are. Only manual rows ever carry a note
// (payments_method_matches_kind's own CHECK), so this reaches nothing a
// Stripe-webhook-written row holds.
func redactPaymentNotes(ctx context.Context, tx *sql.Tx, clientID string) (int, error) {
	res, err := tx.ExecContext(ctx,
		`UPDATE payments SET note = NULL
		 WHERE note IS NOT NULL
		   AND invoice_id IN (
		       SELECT i.id FROM invoices i
		       JOIN contracts c ON c.id = i.contract_id
		       JOIN engagements e ON e.id = c.engagement_id
		       WHERE e.client_id = $1
		   )`,
		clientID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return 0, fmt.Errorf("client: redact payment notes: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		// coverage:ignore reason: lib/pq always reports RowsAffected, not exercised by unit tests
		return 0, fmt.Errorf("client: count redacted payment notes: %w", err)
	}
	return int(n), nil
}

// redactPaymentReversalReasons empties the free-text reason on every
// reversal row (#945) against one of clientID's Invoices -- the same
// personal-data surface redactPaymentNotes already covers for a manually
// recorded Payment's own note, on the same table's other free-text
// column. Only a 'reversal' row ever carries a reason
// (payments_reason_matches_kind's own CHECK), so this reaches nothing a
// 'manual' or 'stripe' row holds.
func redactPaymentReversalReasons(ctx context.Context, tx *sql.Tx, clientID string) (int, error) {
	res, err := tx.ExecContext(ctx,
		`UPDATE payments SET reason = NULL
		 WHERE reason IS NOT NULL
		   AND invoice_id IN (
		       SELECT i.id FROM invoices i
		       JOIN contracts c ON c.id = i.contract_id
		       JOIN engagements e ON e.id = c.engagement_id
		       WHERE e.client_id = $1
		   )`,
		clientID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return 0, fmt.Errorf("client: redact payment reversal reasons: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		// coverage:ignore reason: lib/pq always reports RowsAffected, not exercised by unit tests
		return 0, fmt.Errorf("client: count redacted payment reversal reasons: %w", err)
	}
	return int(n), nil
}

// UnsettledInvoiceSummary is one invoice standing between a Client and
// her own erasure -- what #691's eligibility read names so an Owner sees
// which invoices to settle or void before she ever reaches the
// confirmation, rather than only learning it from a failed erasure's
// 409. It carries no more than payments.InvoiceView already exposes
// elsewhere: this is a courtesy read ahead of an Owner-only act, not a
// second Invoice history view.
type UnsettledInvoiceSummary struct {
	InvoiceID   string    `json:"invoiceId"`
	Status      string    `json:"status"`
	AmountCents int64     `json:"amountCents"`
	Currency    string    `json:"currency"`
	CreatedAt   time.Time `json:"createdAt"`
	// stripeInvoiceID is what EraseHandler's own 409 message names --
	// unexported so it never reaches the eligibility read's JSON, which
	// names invoices by InvoiceID (payments.InvoiceView's own id) instead.
	stripeInvoiceID string
}

// unsettledInvoiceSummaries names every invoice of hers that is neither
// paid, void nor uncollectible. Stripe cannot redact a transaction that
// is not in a terminal state, and the Practice cannot collect on one
// whose Customer has been deleted -- so an unsettled invoice refuses the
// whole act rather than being quietly skipped. Shared by EraseHandler's
// own 409 (stripeInvoiceID joined into its message) and the eligibility
// read below, so the fact named ahead of time is exactly the fact the
// endpoint itself checks.
func unsettledInvoiceSummaries(ctx context.Context, tx *sql.Tx, clientID string) ([]UnsettledInvoiceSummary, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT i.id, i.stripe_invoice_id, i.status, i.amount_cents, i.currency, i.created_at FROM invoices i
		  JOIN contracts ct ON ct.id = i.contract_id
		  JOIN engagements e ON e.id = ct.engagement_id
		 WHERE e.client_id = $1 AND i.status IN ('draft', 'open')
		 ORDER BY i.created_at`,
		clientID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("client: list unsettled invoices: %w", err)
	}
	defer func() { _ = rows.Close() }()

	// Initialized empty, not nil: EraseEligibilityHandler encodes this
	// straight into its own UnsettledInvoices field, and a Client with no
	// invoices at all must still read as "unsettledInvoices": [] rather
	// than null -- one shape for the frontend to check .length on, not two.
	out := []UnsettledInvoiceSummary{}
	for rows.Next() {
		var s UnsettledInvoiceSummary
		if err := rows.Scan(&s.InvoiceID, &s.stripeInvoiceID, &s.Status, &s.AmountCents, &s.Currency, &s.CreatedAt); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("client: scan unsettled invoice: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("client: iterate unsettled invoices: %w", err)
	}
	return out, nil
}

// stripeCustomer is one Stripe Customer of hers and the date its own
// transactions become redactable -- 90 days past the newest invoice
// billed to *that* Customer, not past her newest invoice overall.
//
// The distinction is the whole point. A Client can hold more than one
// Customer with more than one eligibility date, so reading one global
// maximum would hold the older Customer's redaction hostage to the newer
// one turning 90 -- and #394 asks for a job "against every one of her
// invoices/charges that is 90+ days old", each judged on its own age.
//
// She holds more than one for two reasons. Historically, every Invoice
// raised a fresh Customer, so a Client billed six times before #780 has
// six of them recorded on six invoices rows. Since #780 she has one per
// connected account, recorded in client_stripe_customers -- and a
// Practice that re-connects under a new Stripe account gets a second
// mapping row. Erasure has to reach all of them, from both places.
type stripeCustomer struct {
	id         string
	eligibleAt time.Time
}

// enqueueStripeErasure writes one outbox row per Stripe Customer she has
// ever had for the immediate delete, and one more per Customer for the
// redaction job, each deferred to that Customer's own eligibility date.
// It reports the latest of those dates -- when the Stripe half of her
// erasure actually finishes -- or nil when nothing is left to wait for.
//
// A Client with no invoices has no Stripe presence at all: no rows, no
// eligibility date, nothing to wait for.
func enqueueStripeErasure(ctx context.Context, tx *sql.Tx, practiceID, clientID string, now time.Time) ([]string, *time.Time, error) {
	// The union of both places a Customer of hers can be recorded, grouped
	// so an id recorded in both is deleted once, not twice. A mapping row
	// with no invoice behind it still has a date to age from -- when the
	// Customer was made -- so a Customer allocated but never billed is
	// still redactable on a schedule rather than never.
	rows, err := tx.QueryContext(ctx,
		`SELECT customer_id, max(dated_at) FROM (
		     SELECT i.stripe_customer_id AS customer_id, i.created_at AS dated_at
		       FROM invoices i
		       JOIN contracts ct ON ct.id = i.contract_id
		       JOIN engagements e ON e.id = ct.engagement_id
		      WHERE e.client_id = $1 AND i.stripe_customer_id IS NOT NULL
		     UNION ALL
		     SELECT m.stripe_customer_id, m.created_at
		       FROM client_stripe_customers m
		      WHERE m.client_id = $1
		 ) hers
		 GROUP BY customer_id`,
		clientID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, nil, fmt.Errorf("client: list stripe customers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var customers []stripeCustomer
	for rows.Next() {
		var c stripeCustomer
		var newestDate time.Time
		if err := rows.Scan(&c.id, &newestDate); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, nil, fmt.Errorf("client: scan stripe customer: %w", err)
		}
		c.eligibleAt = newestDate.Add(StripeRedactionFloor)
		customers = append(customers, c)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, nil, fmt.Errorf("client: iterate stripe customers: %w", err)
	}
	if len(customers) == 0 {
		return nil, nil, nil
	}

	ids := make([]string, 0, len(customers))
	var latest time.Time
	for _, c := range customers {
		ids = append(ids, c.id)
		if c.eligibleAt.After(latest) {
			latest = c.eligibleAt
		}
		dueAt := c.eligibleAt
		// A Customer whose last invoice is already past the floor waits
		// for nothing -- its job is due immediately.
		if !dueAt.After(now) {
			dueAt = now
		}
		if err := enqueue(ctx, tx, practiceID, clientID, actStripeCustomerDelete, c.id, now, time.Time{}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return nil, nil, err
		}
		if err := enqueue(ctx, tx, practiceID, clientID, actStripeRedactionJob, c.id, dueAt, c.eligibleAt); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return nil, nil, err
		}
	}

	// Nothing to report when every Customer is already redactable: the
	// whole Stripe half runs on the worker's next pass.
	if !latest.After(now) {
		return ids, nil, nil
	}
	return ids, &latest, nil
}

// enqueuePortalErasure removes this Practice's link to her Portal
// Account, and -- when this Practice was the last one holding a live
// Client behind that login -- deletes the Portal Account (#616) outright
// and ends every session she currently holds. A Client has no Identity
// Platform account left to delete (#617, ADR-0026). The sessions are
// deleted here, inside the erasure transaction, rather than left to the
// outbox: deleting the Portal Account does not invalidate a __session
// cookie, which is verified against Postgres, so she would otherwise stay
// signed in to the portal until it expired on its own.
//
// identity_uid is cleared on this Client's row either way, so nothing at
// this Practice points at the login any more. The row itself stays -- it
// is how her portal history resolves.
//
// #830 is why the delete is conditional. ADR-0015 makes a Portal Account
// one person's login reaching many Clients, at most one per Practice,
// and 00081 (#309) made that shape reachable in the schema. The
// unconditional `DELETE FROM portal_accounts WHERE identifier = $1` this
// used to run would then take a login another Practice's un-erased
// Client still reaches, nulling that Practice's own identity_uid through
// the FK cascade and signing her out of a portal session it had nothing
// to do with -- one Practice's erasure request reaching into another
// Practice's relationship with her, which is the thing ADR-0015's "No
// Client fact crosses a Practice" refuses. So the login survives while
// any un-erased Client still reaches it, and the last Practice out takes
// it with it.
//
// The reverse leak is refused just as carefully: whether a sibling
// exists is never reported back. The caller's queued bool means "her
// portal link at this Practice was removed", true in both branches, so
// no Owner can read a Practice she has nothing to do with off her own
// erasure's result.
// Since #813 a Client can be reached by more than one Portal Account --
// two logins, each accepted against one of two records that a merge
// later made one woman. Every one of them is taken here, one at a time,
// so the "last Practice out takes the login" rule below is asked of each
// login separately rather than of whichever row a single read happened
// to return.
func enqueuePortalErasure(ctx context.Context, tx *sql.Tx, clientID string) (queued bool, err error) {
	identityUIDs, hasPending, err := portalIdentityUIDs(ctx, tx, clientID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, err
	}

	if len(identityUIDs) == 0 {
		if !hasPending {
			return false, nil
		}
		// An invitation that was never accepted: there is no Identity
		// Platform account behind it, and revoking the pending invite is
		// enough.
		if _, err := tx.ExecContext(ctx,
			`UPDATE client_portal_users SET invite_token = NULL, invite_token_expires_at = NULL WHERE client_id = $1`, clientID,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return false, fmt.Errorf("client: revoke pending portal invite: %w", err)
		}
		return false, nil
	}

	for _, identityUID := range identityUIDs {
		if err := erasePortalAccount(ctx, tx, identityUID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests -- every branch inside erasePortalAccount is either covered or carries its own justification
			return false, err
		}
	}

	// Runs whatever the loop above did. Where a DELETE ran, the FK
	// cascade has already nulled that row's identity_uid and this clears
	// the invite columns alongside it; where none did, this is the whole
	// act -- every row that named a surviving login from this Practice
	// stops naming it.
	if _, err := tx.ExecContext(ctx,
		`UPDATE client_portal_users SET identity_uid = NULL, invite_token = NULL, invite_token_expires_at = NULL WHERE client_id = $1`, clientID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, fmt.Errorf("client: clear portal identity: %w", err)
	}
	return true, nil
}

// portalIdentityUIDs reads every login clientID's portal rows name, and
// whether any of those rows is a pending invitation instead. Its own
// function so the result set is closed before erasePortalAccount runs a
// second query on the same transaction -- opening one while a result set
// is still streaming deadlocks on the connection.
func portalIdentityUIDs(ctx context.Context, tx *sql.Tx, clientID string) (identityUIDs []string, hasPending bool, err error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT identity_uid FROM client_portal_users WHERE client_id = $1 ORDER BY id`, clientID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, false, fmt.Errorf("client: read portal accounts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var identityUID sql.NullString
		if err := rows.Scan(&identityUID); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, false, fmt.Errorf("client: scan portal account: %w", err)
		}
		if identityUID.Valid {
			identityUIDs = append(identityUIDs, identityUID.String)
		} else {
			hasPending = true
		}
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, false, fmt.Errorf("client: iterate portal accounts: %w", err)
	}
	return identityUIDs, hasPending, nil
}

// erasePortalAccount takes one of her logins: it deletes the Portal
// Account and ends her sessions when this Practice was the last one
// holding a live Client behind it, and does nothing when another
// Practice still reaches it. Split out of enqueuePortalErasure so the
// whole rule reads once and runs once per login.
func erasePortalAccount(ctx context.Context, tx *sql.Tx, identityUID string) error {
	// Serialized on the login before the question is asked. Two Practices
	// erasing her at once would otherwise each read the other's Client as
	// still un-erased under READ COMMITTED, each conclude the login is
	// still reached, and both commit -- leaving a portal_accounts row
	// holding her sign-in address that no Client reaches and that no
	// policy can ever again admit to a delete. That orphan is exactly the
	// PII ADR-0027 exists to scrub, and it is permanent.
	//
	// An advisory lock rather than SELECT ... FOR UPDATE: a row lock
	// applies portal_accounts' UPDATE policy as well as its SELECT one,
	// and the only UPDATE policy the table has (00079) admits the row
	// whose identifier equals app.current_identity_uid -- the Client's own
	// session changing her own sign-in address. Inside a Staff
	// transaction that variable never holds a Portal Account identifier,
	// so FOR UPDATE would match no row and lock nothing at all, which
	// looks exactly like a lock that was taken.
	//
	// It is transaction-scoped, so the
	// same COMMIT or ROLLBACK that settles this erasure releases it, and
	// the waiter's next statement takes a fresh snapshot that sees the
	// first one's answer. A hash collision between two unrelated logins
	// costs one of them a short wait and nothing else.
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, identityUID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("client: lock portal account: %w", err)
	}

	// Asked of the database rather than of a read this transaction could
	// do itself: a sibling row lives at another Practice, which the
	// erasing Practice's own RLS hides, so an inline NOT EXISTS would see
	// nothing and answer "safe to delete" every time.
	// portal_account_reaches_a_live_client (00108) is the SECURITY
	// DEFINER door for exactly this one question.
	//
	// redactRecord has already stamped erased_at on the Client being
	// erased by the time Erase calls this, so she is not her own sibling.
	var reachedElsewhere bool
	if err := tx.QueryRowContext(ctx,
		`SELECT portal_account_reaches_a_live_client($1)`, identityUID,
	).Scan(&reachedElsewhere); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("client: check portal account reach: %w", err)
	}

	if !reachedElsewhere {
		// The Portal Account itself (#616) is deleted here, synchronously,
		// rather than through the outbox above: unlike the Identity Platform
		// account, deleting it is a plain Postgres statement with no outside
		// API to fail or retry, and it holds the sign-in address -- the exact
		// PII this erasure exists to scrub. Run before the UPDATE below, not
		// after: portal_accounts_erasure_delete's own USING clause finds the
		// row by joining back through client_portal_users.identity_uid, so
		// that column must still hold the identifier when this DELETE runs.
		// The FK's ON DELETE SET NULL (#616's migration) is what clears it,
		// as this statement's own side effect.
		res, err := tx.ExecContext(ctx, `DELETE FROM portal_accounts WHERE identifier = $1`, identityUID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return fmt.Errorf("client: delete portal account: %w", err)
		}
		// The policy carries the same predicate the function just
		// answered, so a row this transaction believed it could remove and
		// then could not means the two disagreed -- an invitation accepted
		// against this login between the two statements is the reachable
		// way. Refusing the whole erasure is the safe reading: absorbing a
		// zero-row delete would end every session she holds and leave the
		// login standing, which is #830's own harm in a new place.
		affected, err := res.RowsAffected()
		// coverage:ignore reason: pgx always reports a row count for a DELETE, not exercised by unit tests
		if err != nil {
			return fmt.Errorf("client: count deleted portal accounts: %w", err)
		}
		if affected != 1 {
			// coverage:ignore reason: needs a concurrent invitation accept between two statements of one transaction, not reachable from a unit test
			return fmt.Errorf("client: portal account %q was not deleted: %d rows", identityUID, affected)
		}

		// Ended only here, where the login is going with them. A session
		// is hers, not this Practice's -- one reaches every Client she has
		// -- so while the login survives, this Practice's own access is cut
		// by the UPDATE below instead: clientauth recomputes her reachable
		// set from client_portal_users on every request, so a row with no
		// identity_uid is a Client she can no longer address, with or
		// without a live cookie.
		if err := authn.EndAllSessions(ctx, tx, identityUID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return fmt.Errorf("client: end portal sessions: %w", err)
		}
	}

	return nil
}

// recordErasure writes the one activity row that outlives the shredding:
// plaintext, describing the act and its scope, never a value that was
// erased. Without it, a Practice looking at an erased Client's history
// would see only unreadable entries and no explanation of why.
func recordErasure(ctx context.Context, tx *sql.Tx, practiceID, clientID string, actor activity.Actor, scope erasureScope) error {
	diff, err := json.Marshal(scope)
	if err != nil {
		// coverage:ignore reason: a struct of ints, a bool and a time always marshals cleanly, not exercised by unit tests
		return fmt.Errorf("client: marshal erasure scope: %w", err)
	}
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectClient,
		SubjectID:   clientID,
		Action:      string(eventErased),
		Diff:        diff,
		Actor:       actor,
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("client: record erasure: %w", err)
	}
	return nil
}

// enqueue writes one client_erasure_outbox row, due at dueAt. The
// partial unique index makes a repeat enqueue of an act still pending a
// no-op rather than a duplicate call to somebody else's API.
//
// redactableAfter is the zero time for every act but a redaction job.
// Where it is set, it is the durable fact -- when Stripe will first
// allow this Customer's transactions to be redacted -- and it is written
// once here and never touched again. dueAt is scheduling, and a retry
// rewrites it; the two must not be conflated, which is the mistake
// 00065 exists to correct.
func enqueue(ctx context.Context, tx *sql.Tx, practiceID, clientID string, act erasureAct, target string, dueAt, redactableAfter time.Time) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO client_erasure_outbox (client_id, practice_id, act, target, next_attempt_at, redactable_after)
		 VALUES ($1, $2, $3::client_erasure_act, $4, $5, $6)
		 ON CONFLICT (client_id, act, target) WHERE status = 'pending' DO NOTHING`,
		clientID, practiceID, string(act), target, dueAt, nullIfZeroTime(redactableAfter),
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("client: enqueue erasure act: %w", err)
	}
	return nil
}

// nullIfZeroTime turns the zero time into a SQL NULL -- the same "empty
// means not set" convention nullIfEmpty already applies to this
// package's optional text columns.
func nullIfZeroTime(t time.Time) sql.NullTime {
	return sql.NullTime{Time: t, Valid: !t.IsZero()}
}
