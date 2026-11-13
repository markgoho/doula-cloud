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

/*
 * Every screen that prints a date prints it through this file (#1958):
 * `dates.usage.spec.ts` fails the build on a `toLocaleDateString`,
 * `toLocaleString` or `toLocaleTimeString` anywhere else in `app/src`.
 * Each formatter below reads month-day ("Oct 25, 2026"), the order
 * ADR-0022's 2026-10-24 amendment (#1940) sets for every absolute day the
 * product prints, whatever locale the reader's browser reports.
 */

/**
 * A timestamp is an instant, and reads correctly in the reader's own
 * zone: "Oct 25, 2026".
 */
export function formatInstant(value: string): string {
	return formatDay(new Date(value));
}

/**
 * An instant to the minute, in the reader's own zone: "Oct 25, 2026,
 * 3:04pm" -- the same day and the same 12-hour clock the Activity
 * ledger's absolute branch prints (ADR-0022), for a value that is never
 * shown relative (a message's own sent time, a ledger row).
 */
export function formatInstantWithClock(value: string): string {
	const date = new Date(value);
	return `${formatDay(date)}, ${formatClock(date)}`;
}

/**
 * An instant to the second, in the reader's own zone: "Oct 25, 2026,
 * 3:04:09pm". For the one place the minute is not enough to tell two
 * things apart: two Visits added in the same minute (#2666).
 */
export function formatInstantWithSeconds(value: string): string {
	const date = new Date(value);
	const clock = formatClock(date);
	const seconds = date.getSeconds().toString().padStart(2, '0');
	return `${formatDay(date)}, ${clock.slice(0, -2)}:${seconds}${clock.slice(-2)}`;
}

/**
 * A due date is a calendar day, not an instant: it must not shift by one
 * when the reader's zone is behind UTC, so the day is built from its own
 * parts rather than parsed as UTC midnight. `value` is the date-only
 * `YYYY-MM-DD` form a `type="date"` control posts, and the form the BFF
 * sends an Invoice's `dueOn` and `paidOn` in (#1965): days it has already
 * read in the Practice's own timezone, so the reader's zone must not move
 * them again.
 */
export function formatCalendarDay(value: string): string {
	const [year, month, day] = value.split('-').map(Number);
	return formatDay(new Date(year!, month! - 1, day!));
}

/**
 * How long ago a birth was, in whole weeks: "3 weeks postpartum", and under
 * fourteen days in days, "5 days postpartum". `birthDay` is a calendar day,
 * `YYYY-MM-DD`, counted against the reader's calendar day so a zone cannot
 * move it. For a live birth only: a pregnancy that ended in a loss, or in an
 * outcome nobody learned, is never called postpartum (`docs/personas/loss-client.md`).
 * `now` defaults to the real clock and is a parameter only so a test can pin it.
 */
export function formatPostpartum(birthDay: string, now: Date = new Date()): string {
	const [year, month, day] = birthDay.split('-').map(Number);
	const born = Date.UTC(year!, month! - 1, day!);
	const today = Date.UTC(now.getFullYear(), now.getMonth(), now.getDate());
	const days = Math.max(0, Math.round((today - born) / DAY_MS));
	if (days < 14) return `${days} day${days === 1 ? '' : 's'} postpartum`;
	return `${Math.floor(days / 7)} weeks postpartum`;
}

/**
 * A calendar month, `YYYY-MM`, built from its parts for the same reason
 * as `formatCalendarDay`: "Feb 2027".
 */
export function formatCalendarMonth(value: string): string {
	const [year, month] = value.split('-').map(Number);
	return new Date(year!, month! - 1, 1).toLocaleDateString('en-US', { month: 'short', year: 'numeric' });
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
	return formatInstantWithClock(value);
}

/**
 * A Visit's own Date column and per-row summary (#250): "Not yet
 * scheduled" -- plain text, so it is readable at 320px and announced to a
 * screen reader the same as any other cell -- when `value` is absent, or
 * the weekday-led form ADR-0022's 2026-10-24 amendment writes ("Thu Sep 4,
 * 2:00pm"), the same one `formatPortalVisit` prints for a Visit still to
 * come. The year joins only when the Visit is not in `now`'s year, so a
 * Visit from last winter or next spring still says which. Not
 * relative-under-a-week like `formatActivityTimestamp`'s recent-past
 * branches: a Visit's scheduled instant is as often ahead as behind, and
 * "in 3 days" reads oddly beside "Yesterday" for a thing that has not
 * happened yet. `now` defaults to the real clock and is a parameter only
 * so a test can pin it.
 */
export function formatScheduledVisit(value: string | undefined, now: Date = new Date()): string {
	if (!value) return 'Not yet scheduled';
	const { day, clock } = scheduledVisitParts(value, now);
	return `${day} ${clock}`;
}

/**
 * The two halves of `formatScheduledVisit`'s words, for a cell that sets the
 * day and the clock in tracks of their own: "Thu Sep 4," and "2:00pm". The
 * comma stays with the day, so the halves read as the one phrase when joined
 * by a space.
 */
export function scheduledVisitParts(value: string, now: Date = new Date()): { day: string; clock: string } {
	const date = new Date(value);
	return {
		day: `${formatWeekdayDay(date, { withYear: date.getFullYear() !== now.getFullYear() })},`,
		clock: formatClock(date)
	};
}

/**
 * A calendar day, `YYYY-MM-DD`, with its weekday: "Fri Nov 13", and "Fri Nov
 * 13, 2027" when it is not `now`'s year. The Schedule's result line and its
 * applied days say a day this way, the way a Visit's own day is said.
 */
export function formatCalendarWeekdayDay(value: string, now: Date = new Date()): string {
	const [year, month, day] = value.split('-').map(Number);
	return formatWeekdayDay(new Date(year!, month! - 1, day!), { withYear: year !== now.getFullYear() });
}

/**
 * A Visit as her own portal shows it (#478), and the one place the
 * difference between "Thursday at 2pm" and "she came on August 18" is
 * written. `hasHappened` is the server's own answer (portal.Visit's
 * `hasHappened`, decided against the database clock), never a comparison
 * this module makes against the reader's device: two people looking at
 * the same Visit from two time zones must not disagree about whether it
 * has happened.
 *
 * A Visit still to come reads as a plan -- the weekday first, because
 * that is how a person says it ("when she comes over"), and the clock,
 * because the hour is the part she has to be ready for. One that has
 * happened reads as a record: the calendar day it was on, with no
 * weekday and no clock, since neither is what she is checking a month
 * later ("when Maya came"). The year is on the past form alone -- a
 * scheduled Visit is inside the pregnancy she is living in, a past one
 * may not be. Both are the design source's own two rows on
 * `Your care - Desktop`, read month-day per ADR-0022's 2026-10-24
 * amendment (#1940): "Thu Sep 4, 2:00pm", "Aug 18, 2026", with this
 * file's existing `formatClock` for the hour rather than a second clock
 * format of its own.
 */
export function formatPortalVisit(value: string, hasHappened: boolean): string {
	const date = new Date(value);
	if (hasHappened) return formatDay(date);
	return formatWeekdayDayClock(date, { withYear: false });
}

/**
 * "Thu Sep 4, 2:00pm", or "Thu Sep 4, 2027, 2:00pm" with the year.
 */
function formatWeekdayDayClock(date: Date, { withYear }: { withYear: boolean }): string {
	return `${formatWeekdayDay(date, { withYear })}, ${formatClock(date)}`;
}

/**
 * "Thu Sep 4", or "Thu Sep 4, 2027" with the year.
 */
function formatWeekdayDay(date: Date, { withYear }: { withYear: boolean }): string {
	const weekday = date.toLocaleDateString('en-US', { weekday: 'short' });
	return `${weekday} ${formatDay(date, { withYear })}`;
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

/** "Aug 18, 2026", or "Aug 18" without the year -- the absolute calendar
 * day every formatter here prints, written once. Short month, then day,
 * then year: the order a person in the United States reads and says a
 * date in, which ADR-0022's 2026-10-24 amendment (#1940) sets for the
 * Activity ledger's absolute branch ("Aug 14, 2026, 8:12pm") and every
 * Visit formatter here. */
function formatDay(date: Date, { withYear = true } = {}): string {
	const day = `${date.toLocaleDateString('en-US', { month: 'short' })} ${date.getDate()}`;
	return withYear ? `${day}, ${date.getFullYear()}` : day;
}

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
