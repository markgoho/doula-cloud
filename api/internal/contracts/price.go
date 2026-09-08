package contracts

import (
	"context"
	"database/sql"
	"fmt"
	"maps"
	"slices"

	"github.com/dustin/go-humanize"
)

// priceMergeKey is the reserved merge-field key #967 gives a Contract's
// amount -- resolved by the product the same way clientNameMergeKey and
// practiceNameMergeKey are, but never stored: see withResolvedPrice.
// Matches the default seeded Contract Template (staffauth/signup.go) and
// app/src/lib/contractTemplate.ts's MERGE_FIELDS entry, so a Practice
// that keeps the default prose gets it resolved with no template edit.
const priceMergeKey = "price"

// formatCentsAsDollars renders amountCents as a dollar string with a
// thousands separator and two decimal places (e.g. 150000 -> "$1,500.00"),
// the form the "price" merge field renders into a Contract's prose.
// amountCents is always non-negative here: practicerate.PutRateHandler
// and PutContractAmountHandler both refuse a non-positive amount before
// it ever reaches a column this reads from.
func formatCentsAsDollars(amountCents int64) string {
	dollars := amountCents / 100
	cents := amountCents % 100
	return fmt.Sprintf("$%s.%02d", humanize.Comma(dollars), cents)
}

// withResolvedPrice returns values with priceMergeKey resolved from
// amountCents, when mergeFields (parsed from the Contract's prose) asks
// for it -- the read/render-time counterpart of resolveMergeFieldValues,
// which resolves client_name and practice_name once, at creation, into
// the persisted merge_field_values blob. price is never persisted there:
// ADR-0008's amendment ("Money stops being a merge field") makes the
// real amount_cents column the only place the price lives, so every
// caller that builds a response or renders prose (fillProse) calls this
// once, right after fetchContract, rather than trusting a stored copy
// that PutContractHandler could otherwise be asked to overwrite. Returns
// values unchanged (including a nil map) when the prose never asked for
// a price at all.
func withResolvedPrice(mergeFields []string, values MergeFieldValues, amountCents int64) MergeFieldValues {
	if !slices.Contains(mergeFields, priceMergeKey) {
		return values
	}
	out := make(MergeFieldValues, len(values)+1)
	maps.Copy(out, values)
	out[priceMergeKey] = formatCentsAsDollars(amountCents)
	return out
}

// resolveContractAmount reads engagementID's Engagement kind and looks up
// the Practice's rate_card (practice_rates, 00096_practice_rates.sql) for
// it, via a LEFT JOIN rather than two round trips -- kind is returned
// even when hasRate is false, so PostContractHandler's refusal can name
// exactly which kind has no rate set (#967's AC: "fails in a way a
// person can act on, naming the missing rate").
func resolveContractAmount(ctx context.Context, tx *sql.Tx, engagementID string) (amountCents int64, kind string, hasRate bool, err error) {
	var amount sql.NullInt64
	if err := tx.QueryRowContext(ctx,
		`SELECT e.kind::text, r.amount_cents
		 FROM engagements e
		 LEFT JOIN practice_rates r ON r.practice_id = e.practice_id AND r.kind = e.kind
		 WHERE e.id = $1`,
		engagementID,
	).Scan(&kind, &amount); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests -- resolveContractRequest already proved the Engagement exists
		return 0, "", false, fmt.Errorf("contracts: resolve contract amount: %w", err)
	}
	if !amount.Valid {
		return 0, kind, false, nil
	}
	return amount.Int64, kind, true, nil
}

// noRateSetMsg is PostContractHandler's refusal when the Practice has no
// rate set for the Engagement's kind (#967's AC) -- named so the
// override endpoint's own tests and PostContractHandler's tests can
// assert on the same string rather than each hardcoding it.
func noRateSetMsg(kind string) string {
	return fmt.Sprintf("this practice has no rate set for %s engagements -- set one on the rate card before creating a contract", kind)
}
