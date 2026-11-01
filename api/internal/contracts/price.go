package contracts

import (
	"context"
	"database/sql"
	"fmt"
	"maps"
	"slices"
	"strings"

	"doula-cloud/api/internal/money"
	"doula-cloud/api/internal/staffauth"
)

// priceMergeKey is the reserved merge-field key #967 gives a Contract's
// amount -- resolved by the product the same way clientNameMergeKey and
// PracticeNameMergeKey are, but never stored: see withResolvedPrice.
// Matches the default seeded Contract Template (staffauth/signup.go) and
// app/src/lib/contractTemplate.ts's MERGE_FIELDS entry, so a Practice
// that keeps the default prose gets it resolved with no template edit.
const priceMergeKey = "price"

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
	out[priceMergeKey] = money.Dollars(amountCents)
	return out
}

// priceForReader applies price resolution the way ADR-0008 (as amended
// on #282, and #969's read-boundary half) entitles reader to it: an
// ambient contractor never reads the Practice's price -- only her own
// agreed fee, which lives elsewhere (an Offer/Attachment fee, not this
// Contract) -- while an Owner, an Admin, or an employee Doula all read
// it resolved the same way withResolvedPrice always did. This is the
// piece #969 could not build (the money_/ContractScope/ContractFull
// split it deleted had no single, reliable key to gate on) and #967
// closes now that price is exactly one reserved, column-backed key
// (contract.go's GetContractHandler doc comment names this the interim
// cost being closed here).
//
// Deletes priceMergeKey from values outright for a contractor, rather
// than merely skipping the resolve -- a pre-#967 fixture or a row
// written before this reserved key existed could otherwise still carry
// a raw stored value under that key, which withResolvedPrice's normal
// no-op-if-absent behavior would let leak straight through. This
// reproduces the deleted ContractScope's own guarantee: no price key
// reachable at all, not just a redacted value.
func priceForReader(reader staffauth.Reader, mergeFields []string, values MergeFieldValues, amountCents int64) MergeFieldValues {
	if !reader.IsAmbientContractor() {
		return withResolvedPrice(mergeFields, values, amountCents)
	}
	if _, present := values[priceMergeKey]; !present {
		return values
	}
	out := make(MergeFieldValues, len(values))
	maps.Copy(out, values)
	delete(out, priceMergeKey)
	return out
}

// withoutPriceLines is prose with every line that names the price merge
// field left out (#2003): a Contract shown to a reader who may not read
// the Practice's price must not render "Price:" with nothing after it,
// which reads as a defect in the document rather than as something kept
// from her. The whole line goes, not only the placeholder, because the
// line exists to state the price. A blank line that the removal would
// leave doubled is dropped with it, so the paragraphs around it keep
// their one blank line between them.
func withoutPriceLines(prose string) string {
	lines := strings.Split(prose, "\n")
	kept := make([]string, 0, len(lines))
	removedLast := false
	for _, line := range lines {
		if namesPrice(line) {
			removedLast = true
			continue
		}
		if removedLast && strings.TrimSpace(line) == "" && (len(kept) == 0 || strings.TrimSpace(kept[len(kept)-1]) == "") {
			continue
		}
		removedLast = false
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// namesPrice reports whether line carries the price merge field.
func namesPrice(line string) bool {
	for _, match := range mergeFieldPattern.FindAllStringSubmatch(line, -1) {
		if match[1] == priceMergeKey {
			return true
		}
	}
	return false
}

// shapeForReader is the last step every Staff-side Contract response
// takes (writeContract calls it), so no route can hand an ambient
// contractor the Practice's price by forgetting to: ADR-0008's money row
// as amended on #282 gives her "her own agreed fee only -- never the
// Practice's price". For her, the price line leaves the prose, "price"
// leaves the merge fields, its value leaves Values, and amountChangedAt
// (the fact that the price moved) is withheld. A request with no Staff
// Reader on it -- the Client portal's own Contract read -- passes
// through unchanged: the Client reads what she agreed to pay.
func shapeForReader(reader staffauth.Reader, hasReader bool, out ContractResponse) ContractResponse {
	if !hasReader || !reader.IsAmbientContractor() {
		return out
	}
	out.Prose = withoutPriceLines(out.Prose)
	out.MergeFields = slices.DeleteFunc(slices.Clone(out.MergeFields), func(key string) bool { return key == priceMergeKey })
	if _, present := out.Values[priceMergeKey]; present {
		values := maps.Clone(out.Values)
		delete(values, priceMergeKey)
		out.Values = values
	}
	out.AmountChangedAt = nil
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
// rate set for the Engagement's kind (#967's AC). #1885: it is a sentence
// a person reads, in the words the Rates settings page uses ("Birth
// rate", "Postpartum rate", a page titled "Rates"). The Engagement page
// withholds "Create Draft Contract" before the click in this state, so
// this text shows only when that check and the server disagree.
func noRateSetMsg(kind string) string {
	return fmt.Sprintf("Your Practice has no %s rate yet. Set one on the Rates page in Settings before you create a Contract.", kind)
}
