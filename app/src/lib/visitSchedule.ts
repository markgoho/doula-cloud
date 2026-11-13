/**
 * The Practice-wide schedule (#263): every scheduled Visit at the
 * Practice, across all Clients and all Doulas, soonest first.
 *
 * This module holds the narrowing, the URL shape and the load, decoupled
 * from SvelteKit and the DOM so each can be unit-tested directly --
 * mirrors contract.ts's own Practice-wide roll-up half.
 *
 * The one thing it owns that no other list module does is the difference
 * between a **calendar day** and an **instant**. A person narrows a
 * schedule by day ("the 12th to the 19th"), and a Visit happens at an
 * instant. The URL therefore carries plain `YYYY-MM-DD` days -- readable,
 * and the same link means the same days to whoever opens it -- and this
 * module turns them into the half-open instant range the BFF takes, in
 * the reader's own zone. Sending instants in the URL instead would make a
 * shared link mean a different week in a different zone, and reading the
 * day as UTC midnight would move it by one for every reader behind UTC
 * (dates.ts's own opening note).
 *
 * A bare address is the next 30 days; `?days=7` is the next seven, counted
 * from the day the link is opened; `?from=&to=` is two chosen days.
 */
import type { Fetcher } from './fetcher.js';
import type { CursorPage } from './paginatedList.svelte.js';
import { apiErrorMessage } from './apiErrorMessage.js';
import { formatCalendarWeekdayDay } from './dates.js';

/** One row of the Practice-wide schedule -- mirrors the Go BFF's
 * ScheduledVisit (api/internal/visit/practice_list.go). `scheduledAt` is
 * always present: the endpoint's own predicate excludes a Visit with no
 * scheduled instant, which is why this is not optional the way an
 * Engagement page's Visit row is. */
export interface ScheduledVisit {
	visitId: string;
	engagementId: string;
	clientName: string;
	staffId: string;
	staffName: string;
	scheduledAt: string;
	/**
	A booked Visit whose Doula was taken off its Engagement (#2596).
	*/
	isOffEngagement: boolean;
}

/**
 * One page of the schedule. `visitCount` (#2867) is how many Visits the
 * days and Doula chosen hold in all, on the first page only; a page that
 * lacks it says no total.
 */
export interface SchedulePage extends CursorPage<ScheduledVisit> {
	visitCount?: number;
}

/** How many days ahead the screen looks when the reader has named no
 * range, and the window On call opens on. */
export const DEFAULT_SCHEDULE_DAYS = 30;

/**
The other preset the "Days" choice offers.
*/
export const SHORT_SCHEDULE_DAYS = 7;

export type ScheduleDaysChoice = '7' | '30' | 'choose';

function padTwoDigits(n: number): string {
	return n.toString().padStart(2, '0');
}

/** `<input type="date">`'s own value shape ("YYYY-MM-DD") for a Date, in
 * the reader's zone. Local getters, not `toISOString`, which would name
 * yesterday for anybody behind UTC after their evening. */
export function toDateInputValue(date: Date): string {
	return `${date.getFullYear()}-${padTwoDigits(date.getMonth() + 1)}-${padTwoDigits(date.getDate())}`;
}

/** The calendar day `days` after the given one, as another
 * `YYYY-MM-DD`. Calendar arithmetic on a local Date, not milliseconds: a
 * local day is 25 hours long when daylight saving ends and 23 when it
 * starts, so adding fixed 24-hour blocks to a midnight lands a day off.
 * `setDate` also carries month and year ends correctly. */
export function addDays(day: string, days: number): string {
	const date = startOfLocalDay(day);
	date.setDate(date.getDate() + days);
	return toDateInputValue(date);
}

/** Midnight at the start of a `YYYY-MM-DD` day, in the reader's zone --
 * the day's own parts, never `new Date(day)`, which parses a bare date
 * string as UTC. */
function startOfLocalDay(day: string): Date {
	const [year, month, date] = day.split('-').map(Number);
	return new Date(year, month - 1, date);
}

/**
Whether a string is a real calendar day in `YYYY-MM-DD` form.
*/
export function isCalendarDay(value: string): boolean {
	return /^\d{4}-\d{2}-\d{2}$/.test(value) && toDateInputValue(startOfLocalDay(value)) === value;
}

/** The instants the BFF's half-open range takes, for a pair of calendar
 * days. `from` is the start of its day and `to` is the start of the day
 * **after** the one named, so a reader who asks for "the 12th to the
 * 19th" gets the whole of the 19th rather than only its first instant. */
export function toInstantRange(from: string, to: string): { from: string; to: string } {
	return {
		from: startOfLocalDay(from).toISOString(),
		to: startOfLocalDay(addDays(to, 1)).toISOString()
	};
}

/** What the address says, before any day is worked out: a preset or two
 * typed days, and whose Visits. */
export interface ScheduleView {
	/**
	The preset, used while no days are chosen.
	*/
	preset: typeof SHORT_SCHEDULE_DAYS | typeof DEFAULT_SCHEDULE_DAYS;
	/**
	The days the address carries, as typed: either may be empty or not a day.
	*/
	chosen?: { from: string; to: string };
	/**
	The reader's own Visits.
	*/
	isMine: boolean;
	/**
	One Doula's Visits: a Staff id, or empty.
	*/
	doula: string;
}

/**
The two days a view reads, once they are both days and in order.
*/
export interface ScheduleDays {
	from: string;
	to: string;
}

export interface ScheduleRangeErrors {
	from?: string;
	to?: string;
}

export const ENTER_FROM_DAY = 'Enter a From day';
export const ENTER_TO_DAY = 'Enter a To day';
export const TO_BEFORE_FROM = 'Choose a To day that is the same as the From day or later.';

/**
Why two typed days cannot be a range, in the words each field says.
*/
export function scheduleRangeErrors(from: string, to: string): ScheduleRangeErrors {
	const errors: ScheduleRangeErrors = {};
	if (!isCalendarDay(from)) errors.from = ENTER_FROM_DAY;
	if (!isCalendarDay(to)) errors.to = ENTER_TO_DAY;
	else if (!errors.from && to < from) errors.to = TO_BEFORE_FROM;
	return errors;
}

/** Reads the narrowing off a URL's search parameters. `canMine` is whether
 * this reader is offered "Mine" at all; an address cannot give her what
 * the screen does not. Days beside `from` or `to` are ignored, and a
 * `days` that is not one of the presets is the default. A reader who is not
 * offered the Doula picker (`withoutPicker`, carrying her own Staff id) has
 * no roster to name anyone: her own id in `doula` is "Mine", any other is
 * ignored. */
export function scheduleViewFromParameters(
	parameters: Pick<URLSearchParams, 'get' | 'has'>,
	canMine: boolean,
	withoutPicker?: { staffId: string | undefined }
): ScheduleView {
	const hasChosen = parameters.has('from') || parameters.has('to');
	const doula = parameters.get('doula') ?? '';
	const isOwnDoula = withoutPicker !== undefined && doula !== '' && doula === withoutPicker.staffId;
	return {
		preset: parameters.get('days') === String(SHORT_SCHEDULE_DAYS) ? SHORT_SCHEDULE_DAYS : DEFAULT_SCHEDULE_DAYS,
		chosen: hasChosen ? { from: parameters.get('from') ?? '', to: parameters.get('to') ?? '' } : undefined,
		isMine: canMine && (parameters.get('mine') === 'true' || isOwnDoula),
		doula: withoutPicker ? '' : doula
	};
}

/** The days a view reads: a preset of N days is today through today plus
 * N - 1, the last day included. Chosen days that are not a range carry
 * their errors instead. */
export function scheduleDaysOf(
	view: ScheduleView,
	now: Date = new Date()
): { days: ScheduleDays; errors?: undefined } | { days?: undefined; errors: ScheduleRangeErrors } {
	if (view.chosen) {
		const errors = scheduleRangeErrors(view.chosen.from, view.chosen.to);
		return Object.keys(errors).length > 0 ? { errors } : { days: view.chosen };
	}
	const from = toDateInputValue(now);
	return { days: { from, to: addDays(from, view.preset - 1) } };
}

/** The screen's own URL for a view -- what the browser's address bar
 * holds, and what a shared link carries. Only what a reader chose is
 * written, so the default stays a clean path and `days=30` is never
 * written. */
export function scheduleHref(basePath: string, view: ScheduleView): string {
	const parameters = new URLSearchParams();
	if (view.chosen) {
		parameters.set('from', view.chosen.from);
		parameters.set('to', view.chosen.to);
	} else if (view.preset === SHORT_SCHEDULE_DAYS) {
		parameters.set('days', String(SHORT_SCHEDULE_DAYS));
	}
	if (view.isMine) parameters.set('mine', 'true');
	if (view.doula) parameters.set('doula', view.doula);
	const query = parameters.toString();
	return query ? `${basePath}?${query}` : basePath;
}

/** The view with nothing narrowed beyond what is named, for the default
 * days: what a "Show the next 30 days" link goes to. */
export function withDefaultDays(view: ScheduleView): ScheduleView {
	return { ...view, preset: DEFAULT_SCHEDULE_DAYS, chosen: undefined };
}

/**
Whether the address names anything beyond the default days.
*/
export function isScheduleNarrowed(view: ScheduleView): boolean {
	return view.preset !== DEFAULT_SCHEDULE_DAYS || view.chosen !== undefined || view.isMine || view.doula !== '';
}

/** Whose Visits the BFF is asked for. It takes one Staff id; "Mine" and a
 * different Doula are two narrowings that no Visit meets, so that is an
 * empty list and no read. */
export function scheduleStaffId(
	view: ScheduleView,
	ownStaffId: string | undefined
): { staffId?: string; isEmpty: boolean } {
	if (!view.isMine) return { staffId: view.doula || undefined, isEmpty: false };
	if (view.doula && view.doula !== ownStaffId) return { isEmpty: true };
	return { staffId: ownStaffId, isEmpty: false };
}

/** The narrowing once the days are known: two calendar days that are
 * always present, and the Doula only if one was chosen. The BFF path and
 * the loader take this one shape, so neither can be handed a half-decided
 * range. */
export interface ResolvedScheduleFilters {
	from: string;
	to: string;
	staffId?: string;
}

/** The BFF path for one page of the schedule, with the calendar days
 * already turned into the instants the endpoint takes. */
export function practiceSchedulePath(
	practiceId: string,
	filters: ResolvedScheduleFilters,
	cursor = ''
): string {
	const range = toInstantRange(filters.from, filters.to);
	const parameters = new URLSearchParams({ from: range.from, to: range.to });
	if (filters.staffId) parameters.set('staffId', filters.staffId);
	if (cursor) parameters.set('cursor', cursor);
	return `/api/practices/${practiceId}/visits?${parameters.toString()}`;
}

/** Loads one page of the Practice's schedule. Throws with the response
 * body text on a non-2xx response. */
export async function loadPracticeSchedule(
	fetcher: Fetcher,
	practiceId: string,
	filters: ResolvedScheduleFilters,
	cursor = ''
): Promise<SchedulePage> {
	const response = await fetcher(practiceSchedulePath(practiceId, filters, cursor));
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

/**
The applied tag for chosen days: "Days: Tue Nov 10 to Thu Nov 19".
*/
export function scheduleDaysTag(days: ScheduleDays, now: Date = new Date()): string {
	const from = formatCalendarWeekdayDay(days.from, now);
	return days.from === days.to ? `Days: ${from}` : `Days: ${from} to ${formatCalendarWeekdayDay(days.to, now)}`;
}

/**
 * The day heading a Visit sits under, in the reader's zone: "Today, Fri Nov
 * 13", "Tomorrow, Sat Nov 14", then the day as a row writes it, "Sun Nov 15".
 * `key` is the calendar day, so a day that spans two pages is one heading.
 */
export function scheduleDayGroup(scheduledAt: string, now: Date): { key: string; label: string } {
	const key = toDateInputValue(new Date(scheduledAt));
	const today = toDateInputValue(now);
	const day = formatCalendarWeekdayDay(key, now);
	if (key === today) return { key, label: `Today, ${day}` };
	if (key === addDays(today, 1)) return { key, label: `Tomorrow, ${day}` };
	return { key, label: day };
}

/**
 * A day held whole on one line: its spaces do not break, so a line wraps
 * between days and never inside one.
 */
function wholeDay(day: string, now: Date): string {
	return formatCalendarWeekdayDay(day, now).replaceAll(' ', '\u{A0}');
}

/**
 * Where a sentence says the days are: "from Fri Nov 13 to Sat Dec 12", or
 * "on Mon Nov 16" when the first day is the last.
 */
function scheduleDaysSpan(days: ScheduleDays, now: Date): string {
	return days.from === days.to
		? `on ${wholeDay(days.from, now)}`
		: `from ${wholeDay(days.from, now)} to ${wholeDay(days.to, now)}`;
}

/**
 * The action an empty result offers, the way back to the days the screen
 * opens on first, keeping Mine and the Doula; with those days on, Mine or a
 * Doula answers with every Doula's Visits on the same days. Nothing where
 * the opening days are on and nothing is narrowed.
 */
export function scheduleEmptyNextStep(
	view: ScheduleView
): { label: string; view: ScheduleView } | undefined {
	if (view.preset !== DEFAULT_SCHEDULE_DAYS || view.chosen) {
		return { label: `Show the next ${DEFAULT_SCHEDULE_DAYS} days`, view: withDefaultDays(view) };
	}
	if (view.isMine || view.doula) {
		return { label: "Show every Doula's Visits", view: { ...view, isMine: false, doula: '' } };
	}
	return undefined;
}

export interface ScheduleWho {
	/**
	The picked Doula's name, when one is.
	*/
	doulaName?: string;
	/**
	The reader's own Visits are asked for.
	*/
	isMine?: boolean;
}

/** What the result line says: the count, and while more exist how many there
 * are in all (`total`, the BFF's `visitCount`; a page without one says no
 * "of how many"), the days, whose, and the order. */
export function scheduleResultSummary(
	count: number,
	hasMore: boolean,
	days: ScheduleDays,
	who: ScheduleWho = {},
	now: Date = new Date(),
	total?: number
): string {
	const span = scheduleDaysSpan(days, now);
	if (count === 0) {
		if (who.isMine && who.doulaName) return `No Visits for ${who.doulaName} that are also yours ${span}.`;
		if (who.isMine) return `You have no Visits ${span}.`;
		if (who.doulaName) return `No Visits for ${who.doulaName} ${span}.`;
		return `No Visits ${span}.`;
	}
	const ordered = hasMore || count > 1 ? ', soonest first' : '';
	if (count === 1) {
		const owner = who.isMine ? 'your' : (who.doulaName ? `${who.doulaName}'s` : '');
		const one = hasMore ? 'first Visit' : '1 Visit';
		return `Showing ${owner ? `${owner} ${one}` : (hasMore ? `the ${one}` : one)} ${span}${ordered}.`;
	}
	const owner = who.isMine ? 'your' : (who.doulaName ? `${who.doulaName}'s` : '');
	const whose = owner ? `${owner} ` : '';
	let what = `${whose}${count} Visits`;
	if (hasMore) {
		what =
			total === undefined
				? `the first ${count}${owner ? ` of ${owner}` : ''} Visits`
				: `the first ${count} of ${whose}${Math.max(total, count)} Visits`;
	}
	return `Showing ${what} ${span}${ordered}.`;
}

/** The longest wording the result line can say, for the room the filter bar
 * holds for it so the list does not move between results. Days with a year
 * are the longest a day is said. */
export function scheduleResultRuler(longestDoulaName: string): string {
	const days = { from: '2000-09-29', to: '2000-12-29' };
	const now = new Date(1999, 0, 1);
	return [
		scheduleResultSummary(999, true, days, { doulaName: longestDoulaName }, now, 9999),
		scheduleResultSummary(999, true, days, { isMine: true }, now, 9999),
		scheduleResultSummary(0, false, days, { isMine: true, doulaName: longestDoulaName }, now)
	].toSorted((a, b) => b.length - a.length)[0];
}
