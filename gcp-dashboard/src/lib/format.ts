/**
 * Display formatting. Everything here is American English, USD, and pure.
 */

const usd = new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' });
const share = new Intl.NumberFormat('en-US', { style: 'percent', maximumFractionDigits: 1 });
const day = new Intl.DateTimeFormat('en-US', { month: 'short', day: 'numeric' });
const clock = new Intl.DateTimeFormat('en-US', { hour: '2-digit', minute: '2-digit' });
const hours = new Intl.NumberFormat('en-US', { maximumFractionDigits: 1 });
const compact = new Intl.NumberFormat('en-US', { notation: 'compact', maximumFractionDigits: 2 });

const SECONDS_PER_HOUR = 3600;

/**
 * Stands in for a value the billing export did not report.
 */
export const NOT_REPORTED = '—';

/**
 * `6.6726` renders as `$6.67`. A cost the billing export did not report
 * renders as {@link NOT_REPORTED}.
 */
export function formatUsd(amount: number | undefined): string {
	return amount === undefined ? NOT_REPORTED : usd.format(amount);
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

/**
 * A duration in seconds as hours, e.g. `620750` renders as `172.4`. Cloud
 * Monitoring reports instance time in seconds; a bill is read in hours. A
 * metric Monitoring did not report renders as {@link NOT_REPORTED}.
 */
export function formatHours(seconds: number | undefined): string {
	return seconds === undefined ? NOT_REPORTED : hours.format(seconds / SECONDS_PER_HOUR);
}

/**
 * A large count at a glance: `184300` renders as `184.3K`. A metric Cloud
 * Monitoring did not report renders as {@link NOT_REPORTED}.
 */
export function formatCompact(value: number | undefined): string {
	return value === undefined ? NOT_REPORTED : compact.format(value);
}
