import { formatInstant } from './dates.js';

/**
 * Who did a thing and when (#2104): a Contract's send or signature, a
 * Plan's last save. Mirrors the Go BFF's `activitypage.Stamp`
 * (api/internal/activitypage/latest.go): `by` is already the name the
 * Activity ledger prints for the same person.
 */
export interface Stamp {
	at: string;
	by: string;
}

/**
A Stamp as a summary-list row reads it: "Oct 25, 2026 by Priya Raman".
*/
export function stampText(stamp: Stamp): string {
	return `${formatInstant(stamp.at)} by ${stamp.by}`;
}
