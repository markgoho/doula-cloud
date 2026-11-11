package payments

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/money"
)

const fieldAmountCents = "amountCents"

const (
	msgAmountNotPositive = "Enter an amount more than $0.00."
	msgNothingToBill     = "Every dollar of this Contract is already billed, so there is nothing left to bill."

	detailAmountNotPositive = "Amount must be more than $0.00"
)

// ContractBilling is how much of a Contract's price its Invoices have
// billed. Billed is the amounts of every Invoice that is not void, so a
// written-off Invoice still counts; UnbilledCents is what remains, and
// never goes below zero.
type ContractBilling struct {
	PriceCents    int64 `json:"priceCents"`
	BilledCents   int64 `json:"billedCents"`
	UnbilledCents int64 `json:"unbilledCents"`
}

// contractBilling reads what contractID has billed so far against its
// price.
func contractBilling(ctx context.Context, tx *sql.Tx, contractID string, priceCents int64) (ContractBilling, error) {
	var billed int64
	err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount_cents), 0)::bigint FROM invoices WHERE contract_id = $1 AND status <> 'void'`, contractID,
	).Scan(&billed)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return ContractBilling{}, fmt.Errorf("payments: sum billed invoices: %w", err)
	}
	return ContractBilling{PriceCents: priceCents, BilledCents: billed, UnbilledCents: max(priceCents-billed, 0)}, nil
}

// lockContractBilling serializes the raises against one Contract for the
// rest of tx, so the figure contractBilling reads next cannot be passed
// by a second raise running at the same moment. An advisory lock rather
// than a row lock: the Contract row's update rules are not every raiser's
// to satisfy.
func lockContractBilling(ctx context.Context, tx *sql.Tx, contractID string) error {
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, contractID); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return fmt.Errorf("payments: lock contract billing: %w", err)
	}
	return nil
}

// invoiceAmount is the amount an Invoice is raised for: the amount the
// request names, or everything not yet billed when it names none. A
// refusal is written and ok is false when nothing is left to bill, or
// when the amount is not more than zero or is more than what is left.
func invoiceAmount(w http.ResponseWriter, req CreateInvoiceRequest, billing ContractBilling) (amountCents int64, ok bool) {
	if billing.UnbilledCents == 0 {
		apierr.Write(w, http.StatusConflict, apierr.CodeFailedPrecondition, msgNothingToBill, nil)
		return 0, false
	}
	if req.AmountCents == nil {
		return billing.UnbilledCents, true
	}
	amountCents = *req.AmountCents
	if amountCents <= 0 {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, msgAmountNotPositive,
			map[string]string{fieldAmountCents: detailAmountNotPositive})
		return 0, false
	}
	if amountCents > billing.UnbilledCents {
		left := money.Dollars(billing.UnbilledCents)
		apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument,
			fmt.Sprintf("That is more than the %s still left to bill on this Contract.", left),
			map[string]string{fieldAmountCents: fmt.Sprintf("Amount cannot be more than %s, what is left to bill", left)})
		return 0, false
	}
	return amountCents, true
}
