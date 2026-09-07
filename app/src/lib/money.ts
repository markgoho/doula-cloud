/**
 * The one money formatter for `app/` (#285). `invoice.ts`'s `formatAmount`
 * and `offer.ts`'s `formatFee` each held their own copy of this same
 * cents-to-currency-string conversion, hardcoded to USD and 'en-US' --
 * both now delegate here, and the Billing screen and a future marketing
 * surface (#284) read the same currency code Stripe itself returns rather
 * than assuming USD.
 */

/**
 * Renders `unitAmountCents` (a minor-unit integer, e.g. Stripe's own
 * `unit_amount`) in `currency` (an ISO 4217 code, e.g. Stripe's own
 * `currency`), in the reader's own locale -- "$20.00", not a fixed
 * "USD 20.00" every reader sees alike.
 *
 * The divisor is read back off `Intl.NumberFormat` itself rather than
 * assumed to be 100: a zero-decimal currency (e.g. JPY) has no minor
 * unit at all, and a three-decimal one (e.g. BHD) has a thousandth, so
 * only the formatter's own `maximumFractionDigits` says how many places
 * the integer actually carries.
 */
export function formatMoney(unitAmountCents: number, currency: string): string {
	const formatter = new Intl.NumberFormat(undefined, { style: 'currency', currency: currency.toUpperCase() });
	// TypeScript's Intl.ResolvedNumberFormatOptions types this optional
	// generically (it doesn't apply to every `style`), but currency style
	// always sets it -- MDN's own currency example reads it unconditionally.
	/* v8 ignore next -- currency style always sets maximumFractionDigits; the ?? 2 is a type-narrowing fallback, not a reachable case */
	const fractionDigits = formatter.resolvedOptions().maximumFractionDigits ?? 2;
	return formatter.format(unitAmountCents / 10 ** fractionDigits);
}
