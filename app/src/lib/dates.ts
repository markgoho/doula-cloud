/**
 * The one distinction every screen printing a date has to get right: an
 * instant and a calendar day are not the same kind of value, and reading
 * one as the other moves it by a day.
 *
 * These lived in `engagementRequest.ts` until the Client portal's
 * Engagement hub became their first caller from outside the staff-side
 * Request flow (#505). A Client-facing route reaching into a module about
 * approvals is a module boundary in the wrong place, so the two
 * formatters moved here and the domain vocabulary (`kindLabel`) stayed
 * where it belongs.
 */

/**
 * A timestamp is an instant, and reads correctly in the reader's own
 * zone.
 */
export function formatInstant(value: string): string {
	return new Date(value).toLocaleDateString(undefined, {
		year: 'numeric',
		month: 'short',
		day: 'numeric'
	});
}

/**
 * A due date is a calendar day, not an instant: it must not shift by one
 * when the reader's zone is behind UTC, so the day is built from its own
 * parts rather than parsed as UTC midnight.
 */
export function formatCalendarDay(value: string): string {
	const [year, month, day] = value.split('-').map(Number);
	return new Date(year!, month! - 1, day!).toLocaleDateString(undefined, {
		year: 'numeric',
		month: 'short',
		day: 'numeric'
	});
}
