/**
 * The one billing period both halves of the sync read against.
 *
 * {@link startOfBillingPeriod} and {@link billingPeriodMonth} are the only
 * two things either query needs from a clock, and both derive from the same
 * zone below — so a usage figure and a cost figure can never disagree about
 * which month they cover.
 */

/**
 * The zone GCP assigns `invoice.month` in — not UTC.
 *
 * Checked against the project's own billing export on 2026-09-10, over the
 * Aug/Sep boundary: for every service with rows on both sides, `202608` ran
 * through `usage_start_time` 2026-09-01T06:00:00Z and `202609` began at
 * 07:00:00Z — midnight Pacific, not midnight UTC. Re-verified live on
 * 2026-09-12 with a per-service query against the same export: BigQuery,
 * Cloud Logging, Cloud Run, Cloud Scheduler and Identity Platform all cut
 * over at exactly 07:00:00Z that day. (A few flat-rate SKUs — Artifact
 * Registry, Cloud SQL, Secret Manager — carry an earlier `usage_start_time`
 * on their first `202609` row; that is a daily-aggregation quirk in those
 * SKUs' export rows, not a different invoice boundary.) The same boundary
 * shows in Cloud Monitoring: Firebase Hosting's `network/monthly_sent`
 * gauge, a month-to-date total, resets between 07:07:59Z and 07:16:59Z on
 * the same day. See #1174.
 */
export const GCP_INVOICE_TIMEZONE = 'America/Los_Angeles';

const pacificDateFormatter = new Intl.DateTimeFormat('en-US', {
	timeZone: GCP_INVOICE_TIMEZONE,
	year: 'numeric',
	month: '2-digit',
	day: '2-digit'
});

const pacificClockFormatter = new Intl.DateTimeFormat('en-US', {
	timeZone: GCP_INVOICE_TIMEZONE,
	hourCycle: 'h23',
	year: 'numeric',
	month: '2-digit',
	day: '2-digit',
	hour: '2-digit',
	minute: '2-digit',
	second: '2-digit'
});

interface YearMonth {
	readonly year: number;
	readonly month: number;
}

/**
 * `at`'s calendar year and month in {@link GCP_INVOICE_TIMEZONE}.
 */
function pacificYearMonth(at: Date): YearMonth {
	const fields = Object.fromEntries(
		pacificDateFormatter.formatToParts(at).map((field) => [field.type, field.value])
	);

	return { year: Number(fields.year), month: Number(fields.month) };
}

/**
 * How far {@link GCP_INVOICE_TIMEZONE} sits from UTC at `instant`, in
 * minutes — negative, since Pacific is west of UTC. This is -480 for the
 * part of the year the zone keeps standard time and -420 the rest, never a
 * fixed offset.
 */
function pacificOffsetMinutesAt(instant: number): number {
	const fields = Object.fromEntries(
		pacificClockFormatter.formatToParts(instant).map((field) => [field.type, field.value])
	);
	const wallClockReadAsUtc = Date.UTC(
		Number(fields.year),
		Number(fields.month) - 1,
		Number(fields.day),
		Number(fields.hour),
		Number(fields.minute),
		Number(fields.second)
	);

	return (wallClockReadAsUtc - instant) / 60_000;
}

const MILLISECONDS_PER_MINUTE = 60_000;
const CONVERGENCE_PASSES = 2;

/**
 * The UTC instant of midnight, `year`-`month`-01, in
 * {@link GCP_INVOICE_TIMEZONE}.
 *
 * Converting a wall-clock instant to UTC needs the zone's offset, but the
 * offset itself depends on the instant being converted. Two passes settle
 * it: each reads the offset at the previous pass's guess and refines the
 * instant from it. That converges wherever the guess and the true instant
 * land on the same side of a daylight-saving change, which midnight always
 * does — every US transition happens at 2 a.m. local, hours after the
 * guess a month start ever lands on.
 */
function pacificMonthStartUtc(year: number, month: number): Date {
	const wallClockReadAsUtc = Date.UTC(year, month - 1, 1);
	let instant = wallClockReadAsUtc;

	for (let pass = 0; pass < CONVERGENCE_PASSES; pass += 1) {
		const offsetMinutes = pacificOffsetMinutesAt(instant);
		instant = wallClockReadAsUtc - offsetMinutes * MILLISECONDS_PER_MINUTE;
	}

	return new Date(instant);
}

/**
 * First instant of the billing period `now` falls in.
 *
 * The period is the calendar month in {@link GCP_INVOICE_TIMEZONE}, the zone
 * GCP assigns `invoice.month` in. {@link billingPeriodMonth} reads the same
 * period's `invoice.month` off the same zone, so the two cannot drift apart.
 */
export function startOfBillingPeriod(now: Date): Date {
	const { year, month } = pacificYearMonth(now);

	return pacificMonthStartUtc(year, month);
}

/**
 * `invoice.month` (`YYYYMM`) for the billing period `now` falls in — the
 * calendar month in {@link GCP_INVOICE_TIMEZONE}, read the same way
 * {@link startOfBillingPeriod} reads the period's start.
 */
export function billingPeriodMonth(now: Date): string {
	const { year, month } = pacificYearMonth(now);

	return `${year}${String(month).padStart(2, '0')}`;
}
