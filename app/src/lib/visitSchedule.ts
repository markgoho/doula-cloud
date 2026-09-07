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
 */
import type { Fetcher } from './fetcher.js';
import type { CursorPage } from './paginatedList.svelte.js';
import { apiErrorMessage } from './apiErrorMessage.js';

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
}

/** The narrowing a reader chose, in the shape the URL carries it: two
 * calendar days and, optionally, one Doula. Every field is optional
 * because a bare URL is a valid one -- `defaultScheduleRange` supplies
 * the days the screen then shows in its own controls. */
export interface ScheduleFilters {
	from?: string;
	to?: string;
	staffId?: string;
}

/** How many days ahead the screen looks when the reader has named no
 * range -- the same window the BFF defaults to, named here so the two
 * cannot silently disagree about what "no filter" shows. */
export const DEFAULT_SCHEDULE_DAYS = 30;

const DAY_MS = 24 * 60 * 60 * 1000;

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
 * `YYYY-MM-DD`. Built through a local Date so month and year ends carry
 * correctly. */
export function addDays(day: string, days: number): string {
	return toDateInputValue(new Date(startOfLocalDay(day).getTime() + days * DAY_MS));
}

/** The days the screen shows when the URL names none: today through
 * today plus the default window. `now` is injectable so a test does not
 * have to reason about the wall clock. */
export function defaultScheduleRange(now: Date = new Date()): { from: string; to: string } {
	const from = toDateInputValue(now);
	return { from, to: addDays(from, DEFAULT_SCHEDULE_DAYS) };
}

/** Midnight at the start of a `YYYY-MM-DD` day, in the reader's zone --
 * the day's own parts, never `new Date(day)`, which parses a bare date
 * string as UTC. */
function startOfLocalDay(day: string): Date {
	const [year, month, date] = day.split('-').map(Number);
	return new Date(year, month - 1, date);
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

/** Reads the narrowing off a URL's search parameters, falling back to
 * the default window for a day the URL does not name. The screen's
 * controls and its fetch both read this, so what is shown and what was
 * asked for can never drift apart. */
export function scheduleFiltersFromParameters(
	parameters: URLSearchParams,
	now: Date = new Date()
): ResolvedScheduleFilters {
	const fallback = defaultScheduleRange(now);
	return {
		from: parameters.get('from') ?? fallback.from,
		to: parameters.get('to') ?? fallback.to,
		staffId: parameters.get('staffId') ?? undefined
	};
}

/** The narrowing once the default window has filled the gaps a bare URL
 * leaves: two calendar days that are always present, and the Doula only
 * if one was chosen. The href builder, the BFF path and the loader all
 * take this one shape, so none of them can be handed a half-decided
 * range. */
export type ResolvedScheduleFilters = Required<Pick<ScheduleFilters, 'from' | 'to'>> &
	Pick<ScheduleFilters, 'staffId'>;

/** The screen's own URL for a narrowing -- what the browser's address bar
 * holds, and what a shared link carries. Only the parts a reader actually
 * chose are written, so an unnarrowed schedule stays a clean path. */
export function scheduleHref(
	basePath: string,
	filters: ScheduleFilters,
	now: Date = new Date()
): string {
	const fallback = defaultScheduleRange(now);
	const parameters = new URLSearchParams();
	if (filters.from && filters.from !== fallback.from) parameters.set('from', filters.from);
	if (filters.to && filters.to !== fallback.to) parameters.set('to', filters.to);
	if (filters.staffId) parameters.set('staffId', filters.staffId);
	const query = parameters.toString();
	return query ? `${basePath}?${query}` : basePath;
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
 * body text on a non-2xx response; the route's `load` maps status codes
 * to SvelteKit errors before calling this, so a throw here is only ever
 * an unexpected failure. */
export async function loadPracticeSchedule(
	fetcher: Fetcher,
	practiceId: string,
	filters: ResolvedScheduleFilters,
	cursor = ''
): Promise<CursorPage<ScheduledVisit>> {
	const response = await fetcher(practiceSchedulePath(practiceId, filters, cursor));
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

/** What the result count says out loud, for the live region that
 * announces it. Words rather than a bare number, because "0" alone is
 * the thing a screen reader reads as nothing having happened. The
 * endpoint answers `hasMore` rather than a total, so this never claims a
 * total it was not given. */
export function scheduleResultSummary(count: number, hasMore: boolean): string {
	if (count === 0) return 'No scheduled visits in this range.';
	const visits = count === 1 ? '1 scheduled visit' : `${count} scheduled visits`;
	return hasMore ? `Showing ${visits} so far. More remain.` : `Showing ${visits}.`;
}
