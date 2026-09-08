/**
 * Display formatting. Everything here is American English, USD, and pure.
 */

const usd = new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' });
const share = new Intl.NumberFormat('en-US', { style: 'percent', maximumFractionDigits: 1 });
const day = new Intl.DateTimeFormat('en-US', { month: 'short', day: 'numeric' });
const clock = new Intl.DateTimeFormat('en-US', { hour: '2-digit', minute: '2-digit' });

/**
 * Stands in for a value the billing export did not report.
 */
export const NOT_REPORTED = '—';

/**
 * `6.6726` renders as `$6.67`.
 */
export function formatUsd(amount: number): string {
	return usd.format(amount);
}

/**
 * `0.385` renders as `38.5%`.
 */
export function formatShare(fraction: number): string {
	return share.format(fraction);
}

/**
 * An ISO-8601 instant as a short day, e.g. `Sep 7`. A missing instant renders
 * as {@link NOT_REPORTED}.
 */
export function formatDay(instant: string | undefined): string {
	return instant === undefined ? NOT_REPORTED : day.format(new Date(instant));
}

/**
 * Wall-clock time of a sync, from epoch milliseconds, e.g. `06:12 AM`.
 */
export function formatClock(at: number): string {
	return clock.format(at);
}
