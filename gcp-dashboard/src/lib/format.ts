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
const BYTES_PER_GIBIBYTE = 1024 ** 3;
const gibibytes = new Intl.NumberFormat('en-US', { maximumFractionDigits: 1 });

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

/**
 * A count of bytes as gibibytes, e.g. `10464022528` renders as `9.7`. Cloud
 * Monitoring reports a disk quota in bytes; a Cloud SQL bill is read in GiB.
 * A metric Monitoring did not report renders as {@link NOT_REPORTED}.
 */
export function formatGibibytes(bytes: number | undefined): string {
	return bytes === undefined ? NOT_REPORTED : gibibytes.format(bytes / BYTES_PER_GIBIBYTE);
}

/**
 * The binary units a byte count is read in, smallest first. A dashboard that
 * reports every count in GiB writes `0` for half a megabyte, which reads as
 * no usage rather than a little.
 */
const BYTE_UNITS = ['B', 'KiB', 'MiB', 'GiB', 'TiB'] as const;
const BYTES_PER_UNIT = 1024;
const scaledBytes = new Intl.NumberFormat('en-US', { maximumFractionDigits: 1 });

/**
 * A byte count and the unit it is worth reading in: `567149` renders as
 * `{ value: '553.9', unit: 'KiB' }`, `0` as `{ value: '0', unit: 'B' }`.
 *
 * The unit comes back beside the figure rather than inside it, because the
 * stat grid draws a unit smaller than the number it belongs to. A metric
 * Cloud Monitoring did not report renders as {@link NOT_REPORTED} with no
 * unit at all.
 */
export function formatBytes(bytes: number | undefined): { value: string; unit: string } {
	if (bytes === undefined) return { value: NOT_REPORTED, unit: '' };

	let scaled = bytes;
	let unit = 0;
	while (scaled >= BYTES_PER_UNIT && unit < BYTE_UNITS.length - 1) {
		scaled /= BYTES_PER_UNIT;
		unit += 1;
	}

	return { value: scaledBytes.format(scaled), unit: BYTE_UNITS[unit] };
}
