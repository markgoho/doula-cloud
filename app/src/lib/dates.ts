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

/**
 * The activity ledger's own clock (ADR-0022): relative under seven days,
 * absolute beyond it, always on a 12-hour clock with a lowercase am/pm
 * and no periods -- "this product is used in the United States by people
 * who say 'she came at two in the morning'". The exact instant is never
 * lost: it lives in `value` itself, which a caller keeps as the
 * rendered element's machine-readable value (a `<time datetime>`) so a
 * screen reader, a hover or a copy-paste still gets it -- this function
 * only ever answers the display string.
 *
 * `now` defaults to the real clock and is a parameter only so a test can
 * pin it; every threshold below is elapsed real time, not a calendar-day
 * boundary, which is what makes "Yesterday" and the weekday name follow
 * the ADR's own worked examples ("12 minutes ago", "2 hours ago",
 * "Yesterday, 9:31am", "Tuesday, 4:40pm") in the order given rather than
 * flipping at local midnight regardless of how recent the event was.
 */
export function formatActivityTimestamp(value: string, now: Date = new Date()): string {
	const date = new Date(value);
	const diffMs = now.getTime() - date.getTime();
	if (diffMs < MINUTE_MS) return 'just now';
	if (diffMs < HOUR_MS) {
		const minutes = Math.floor(diffMs / MINUTE_MS);
		return `${minutes} minute${minutes === 1 ? '' : 's'} ago`;
	}
	if (diffMs < DAY_MS) {
		const hours = Math.floor(diffMs / HOUR_MS);
		return `${hours} hour${hours === 1 ? '' : 's'} ago`;
	}
	if (diffMs < 2 * DAY_MS) return `Yesterday, ${formatClock(date)}`;
	if (diffMs < WEEK_MS) {
		const weekday = date.toLocaleDateString('en-US', { weekday: 'long' });
		return `${weekday}, ${formatClock(date)}`;
	}
	const day = date.getDate();
	const month = date.toLocaleDateString('en-US', { month: 'short' });
	const year = date.getFullYear();
	return `${day} ${month} ${year}, ${formatClock(date)}`;
}

/**
 * A Visit's own Date column and per-row summary (#250): "Not yet
 * scheduled" -- plain text, so it is readable at 320px and announced to a
 * screen reader the same as any other cell -- when `value` is absent, or
 * the same day-and-clock treatment `formatActivityTimestamp`'s absolute
 * branch already uses (`formatClock`, this file's own) otherwise. Not
 * relative-under-a-week like that function's own recent-past branches: a
 * Visit's scheduled instant is as often ahead as behind, and "in 3 days"
 * reads oddly beside "Yesterday" for a thing that has not happened yet.
 */
export function formatScheduledVisit(value: string | undefined): string {
	if (!value) return 'Not yet scheduled';
	const date = new Date(value);
	const day = date.getDate();
	const month = date.toLocaleDateString('en-US', { month: 'short' });
	const year = date.getFullYear();
	return `${day} ${month} ${year}, ${formatClock(date)}`;
}

/**
 * `<input type="datetime-local">`'s own value shape ("YYYY-MM-DDTHH:mm",
 * local time, no offset) from an ISO instant -- the inverse of what that
 * control hands back on input, so a Visit's existing scheduled value
 * (#250) can prefill the field it is being changed through. `""` for
 * `undefined`/`null`, the value an empty control itself reports.
 */
export function toDatetimeLocalValue(value: string | undefined): string {
	if (!value) return '';
	const date = new Date(value);
	return `${date.getFullYear()}-${padTwoDigits(date.getMonth() + 1)}-${padTwoDigits(date.getDate())}T${padTwoDigits(date.getHours())}:${padTwoDigits(date.getMinutes())}`;
}

function padTwoDigits(n: number): string {
	return n.toString().padStart(2, '0');
}

const MINUTE_MS = 60_000;
const HOUR_MS = 60 * MINUTE_MS;
const DAY_MS = 24 * HOUR_MS;
const WEEK_MS = 7 * DAY_MS;

/** "9:31am" / "12:00pm" -- 12-hour, lowercase am/pm, no periods, minutes
 * always two digits, hour never zero-padded and never 0 (noon and
 * midnight are both 12, per the 12-hour clock's own convention). */
function formatClock(date: Date): string {
	const hours24 = date.getHours();
	const suffix = hours24 < 12 ? 'am' : 'pm';
	const hours = hours24 % 12 === 0 ? 12 : hours24 % 12;
	const minutes = date.getMinutes().toString().padStart(2, '0');
	return `${hours}:${minutes}${suffix}`;
}
