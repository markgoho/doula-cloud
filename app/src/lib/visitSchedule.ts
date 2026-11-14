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
 * the Practice's zone (ADR-0036). Sending instants in the URL instead would make a
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
import { visitTypeLabel } from './engagementDetail.js';
import {
	addDays,
	daysOf,
	THIS_WEEK,
	daysSpan,
	daysTag,
	daysWords,
	rangeErrors,
	stepDays,
	type Days,
	type DaysStep,
	type RangeErrors
} from './dayRange.js';
import { dayIn, startOfDayIn } from './zonedTime.js';

export { addDays, ENTER_FROM_DAY, ENTER_TO_DAY, isCalendarDay, TO_BEFORE_FROM } from './dayRange.js';

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
	/**
	prenatal, birth or postpartum: the BFF's one derivation (ADR-0036).
	*/
	type: string;
}

/**
 * One page of the schedule. `visitCount` (#2867) is how many Visits the
 * days and Doula chosen hold in all, on the first page only; a page that
 * lacks it says no total.
 */
export interface SchedulePage extends CursorPage<ScheduledVisit> {
	visitCount?: number;
	/**
	How many Visits under the same other narrowings wait for a new Doula (#2863): on the first page of a read not already narrowed to them, for an Owner or Admin.
	*/
	offEngagementCount?: number;
}

/** How many days ahead the screen looks when the reader has named no
 * range, and the window On call opens on. */
export const DEFAULT_SCHEDULE_DAYS = 30;

/**
The other preset the "Days" choice offers.
*/
export const SHORT_SCHEDULE_DAYS = 7;

const THIS_WEEK_PARAMETER = 'week';

export type ScheduleDaysChoice = '7' | 'week' | '30' | 'choose';

export type ScheduleDaysPreset = typeof SHORT_SCHEDULE_DAYS | typeof THIS_WEEK | typeof DEFAULT_SCHEDULE_DAYS;

const PRESET_OF_CHOICE = { '7': SHORT_SCHEDULE_DAYS, week: THIS_WEEK, '30': DEFAULT_SCHEDULE_DAYS } as const;

/**
The preset a Days radio stands for; "Choose days" is none and reads as the default.
*/
export function schedulePresetOfChoice(choice: ScheduleDaysChoice): ScheduleDaysPreset {
	return choice === 'choose' ? DEFAULT_SCHEDULE_DAYS : PRESET_OF_CHOICE[choice];
}

/**
The Days radio a preset is drawn as.
*/
export function scheduleChoiceOfPreset(preset: ScheduleDaysPreset): ScheduleDaysChoice {
	if (preset === THIS_WEEK) return 'week';
	return preset === SHORT_SCHEDULE_DAYS ? '7' : '30';
}

/** The instants the BFF's half-open range takes, for a pair of calendar
 * days in the Practice's zone. `from` is the start of its day and `to` is
 * the start of the day **after** the one named, so a reader who asks for
 * "the 12th to the 19th" gets the whole of the 19th rather than only its
 * first instant. */
export function toInstantRange(from: string, to: string, timeZone: string): { from: string; to: string } {
	return {
		from: startOfDayIn(from, timeZone).toISOString(),
		to: startOfDayIn(addDays(to, 1), timeZone).toISOString()
	};
}

/** What the address says, before any day is worked out: a preset or two
 * typed days, and whose Visits. */
export interface ScheduleView {
	/**
	The preset, used while no days are chosen.
	*/
	preset: ScheduleDaysPreset;
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
	/**
	Only the Visits waiting for a new Doula (#2863).
	*/
	needsDoula: boolean;
	/**
	One kind of Visit, or empty for every kind.
	*/
	type: ScheduleType;
	/**
	The order the reader chose: empty is soonest first, the list's own.
	*/
	order: ScheduleOrder;
}

/**
The Visit types the Schedule narrows to (#2888): the empty one is every type.
*/
export type ScheduleType = '' | 'prenatal' | 'birth' | 'postpartum';

export const ANY_SCHEDULE_TYPE = 'Any type';

const SCHEDULE_TYPES = ['prenatal', 'birth', 'postpartum'] as const;

export const SCHEDULE_TYPE_OPTIONS: readonly { value: ScheduleType; label: string }[] = [
	{ value: '', label: ANY_SCHEDULE_TYPE },
	...SCHEDULE_TYPES.map((type) => ({ value: type, label: visitTypeLabel(type) }))
];

function scheduleTypeOf(value: string | null): ScheduleType {
	return SCHEDULE_TYPES.find((type) => type === value) ?? '';
}

/**
The orders the Schedule is read in (#2869): the empty one is soonest first.
*/
export type ScheduleOrder = '' | 'latest';

export const LATEST_ORDER: ScheduleOrder = 'latest';

export const SCHEDULE_ORDER_OPTIONS: readonly { value: ScheduleOrder; label: string }[] = [
	{ value: '', label: 'Soonest first' },
	{ value: LATEST_ORDER, label: 'Latest first' }
];

export type ScheduleDays = Days;

export type ScheduleRangeErrors = RangeErrors;

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
	const days = parameters.get('days');
	const isOwnDoula = withoutPicker !== undefined && doula !== '' && doula === withoutPicker.staffId;
	return {
		preset: days === '7' || days === THIS_WEEK_PARAMETER ? PRESET_OF_CHOICE[days] : DEFAULT_SCHEDULE_DAYS,
		chosen: hasChosen ? { from: parameters.get('from') ?? '', to: parameters.get('to') ?? '' } : undefined,
		isMine: canMine && (parameters.get('mine') === 'true' || isOwnDoula),
		doula: withoutPicker ? '' : doula,
		needsDoula: !withoutPicker && parameters.get('needsDoula') === 'true',
		type: scheduleTypeOf(parameters.get('type')),
		order: parameters.get('order') === LATEST_ORDER ? LATEST_ORDER : ''
	};
}

/**
The days a view reads, or why two chosen days are not a range.
*/
export function scheduleDaysOf(view: ScheduleView, timeZone: string, now: Date = new Date()) {
	return daysOf(view, timeZone, now);
}

/**
Why two typed days cannot be a range, in the words each field says.
*/
export function scheduleRangeErrors(from: string, to: string): ScheduleRangeErrors {
	return rangeErrors(from, to);
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
	} else if (view.preset === THIS_WEEK) {
		parameters.set('days', THIS_WEEK_PARAMETER);
	} else if (view.preset === SHORT_SCHEDULE_DAYS) {
		parameters.set('days', String(SHORT_SCHEDULE_DAYS));
	}
	if (view.isMine) parameters.set('mine', 'true');
	if (view.doula) parameters.set('doula', view.doula);
	if (view.needsDoula) parameters.set('needsDoula', 'true');
	if (view.type) parameters.set('type', view.type);
	if (view.order) parameters.set('order', view.order);
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
	return view.preset !== DEFAULT_SCHEDULE_DAYS || view.chosen !== undefined || view.isMine || view.doula !== '' || view.needsDoula || view.type !== '' || view.order !== '';
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
 * range. `timeZone` is the Practice's, the zone the two days are in. */
export interface ResolvedScheduleFilters {
	from: string;
	to: string;
	timeZone: string;
	staffId?: string;
	order?: ScheduleOrder;
	/**
	Only the Visits waiting for a new Doula (#2863).
	*/
	needsDoula?: boolean;
	/**
	Only one kind of Visit (#2888).
	*/
	type?: ScheduleType;
	/**
	The Client's name a reader typed (#2870). It is in no address (#1829), so a read that has one is a POST.
	*/
	search?: string;
}

/**
 * The narrowing a view reads under, and whether there is anything to read:
 * days that are not a range, and "Mine" beside a different Doula, are
 * answered with an empty list and no read, so the BFF is never sent a range
 * that can never match. `load` and a read the screen owns (a search, which
 * `load` never sees) both resolve a view here.
 */
export function resolveScheduleRead(
	view: ScheduleView,
	timeZone: string,
	ownStaffId: string | undefined,
	now: Date,
	search = ''
): { filters: ResolvedScheduleFilters; errors?: ScheduleRangeErrors; isReadable: boolean } {
	const read = scheduleDaysOf(view, timeZone, now);
	const whose = scheduleStaffId(view, ownStaffId);
	const filters: ResolvedScheduleFilters = read.days
		? { ...read.days, timeZone, staffId: whose.staffId, order: view.order }
		: { from: '', to: '', timeZone };
	if (search) filters.search = search;
	if (view.needsDoula) filters.needsDoula = true;
	if (view.type) filters.type = view.type;
	return { filters, errors: read.errors, isReadable: read.days !== undefined && !whose.isEmpty };
}

/** The BFF path for one page of the schedule, with the calendar days
 * already turned into the instants the endpoint takes. */
export function practiceSchedulePath(
	practiceId: string,
	filters: ResolvedScheduleFilters,
	cursor = ''
): string {
	const range = toInstantRange(filters.from, filters.to, filters.timeZone);
	const parameters = new URLSearchParams({ from: range.from, to: range.to });
	if (filters.staffId) parameters.set('staffId', filters.staffId);
	if (filters.order) parameters.set('order', filters.order);
	if (filters.needsDoula) parameters.set('offEngagement', 'true');
	if (filters.type) parameters.set('type', filters.type);
	if (cursor) parameters.set('cursor', cursor);
	return `/api/practices/${practiceId}/visits?${parameters.toString()}`;
}

/** Loads one page of the Practice's schedule. A search -- a Client's name --
 * makes it a POST with the narrowing and the cursor in the body (#2870): a
 * name is in no URL, so in no log (#1829). Throws with the response body
 * text on a non-2xx response. */
export async function loadPracticeSchedule(
	fetcher: Fetcher,
	practiceId: string,
	filters: ResolvedScheduleFilters,
	cursor = ''
): Promise<SchedulePage> {
	let response: Response;
	if (filters.search) {
		const range = toInstantRange(filters.from, filters.to, filters.timeZone);
		response = await fetcher(`/api/practices/${practiceId}/visits/list`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				from: range.from,
				to: range.to,
				staffId: filters.staffId ?? '',
				order: filters.order ?? '',
				search: filters.search,
				offEngagement: filters.needsDoula === true,
				type: filters.type ?? '',
				cursor
			})
		});
	} else {
		response = await fetcher(practiceSchedulePath(practiceId, filters, cursor));
	}
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

/**
The days as a reader says them: "Fri Nov 6 to Thu Nov 12", or one day alone.
*/
export const scheduleDaysWords = daysWords;

/**
The applied tag for chosen days: "Days: Tue Nov 10 to Thu Nov 19".
*/
export const scheduleDaysTag = daysTag;

export type ScheduleStep = DaysStep<ScheduleView>;

/**
 * The window one step earlier (`-1`) or later (`1`): the days the reader sees
 * moved by their own length, 7 for the next 7 days, 30 for the next 30, and
 * to - from + 1 for chosen days. Nothing where the days are not a range.
 */
export function scheduleStep(
	view: ScheduleView,
	direction: -1 | 1,
	timeZone: string,
	now: Date
): ScheduleStep | undefined {
	return stepDays(view, direction, timeZone, now, [SHORT_SCHEDULE_DAYS, THIS_WEEK, DEFAULT_SCHEDULE_DAYS]);
}

/**
 * The day heading a Visit sits under, in the Practice's zone: "Today, Fri Nov
 * 13", "Tomorrow, Sat Nov 14", then the day as a row writes it, "Sun Nov 15".
 * `key` is the calendar day, so a day that spans two pages is one heading.
 */
export function scheduleDayGroup(scheduledAt: string, timeZone: string, now: Date): { key: string; label: string } {
	const key = dayIn(new Date(scheduledAt), timeZone);
	const today = dayIn(now, timeZone);
	const day = formatCalendarWeekdayDay(key, timeZone, now);
	if (key === today) return { key, label: `Today, ${day}` };
	if (key === addDays(today, 1)) return { key, label: `Tomorrow, ${day}` };
	return { key, label: day };
}

/**
 * The action an empty result offers, the way back to the days the screen
 * opens on first, keeping Mine, the Doula and the type; with those days on,
 * a type answers with every type of Visit, then Mine or a Doula answers with
 * every Doula's Visits on the same days. Nothing where the opening days are
 * on and nothing is narrowed.
 */
export function scheduleEmptyNextStep(
	view: ScheduleView
): { label: string; view: ScheduleView } | undefined {
	if (view.preset !== DEFAULT_SCHEDULE_DAYS || view.chosen) {
		return { label: `Show the next ${DEFAULT_SCHEDULE_DAYS} days`, view: withDefaultDays(view) };
	}
	if (view.type) {
		return { label: 'Show every type of Visit', view: { ...view, type: '' } };
	}
	if (view.isMine || view.doula) {
		return { label: "Show every Doula's Visits", view: { ...view, isMine: false, doula: '' } };
	}
	return undefined;
}

/**
 * The action an empty search offers (#2870): the same search over a year
 * back and a year ahead, today less 365 days through today plus 364 days in
 * the Practice's zone, keeping Mine, the Doula, the type and the order.
 * Nothing where the days already reach that far.
 */
export function scheduleSearchNextStep(
	view: ScheduleView,
	search: string,
	timeZone: string,
	now: Date
): { label: string; view: ScheduleView } | undefined {
	const today = dayIn(now, timeZone);
	const from = addDays(today, -365);
	const to = addDays(today, 364);
	const days = scheduleDaysOf(view, timeZone, now).days;
	if (days && days.from <= from && days.to >= to) return undefined;
	return { label: `Look for "${search}" a year back and a year ahead`, view: { ...view, chosen: { from, to } } };
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
	/**
	The order the rows are in; the line ends on its words.
	*/
	order?: ScheduleOrder;
	/**
	The Client's name the rows were searched for.
	*/
	search?: string;
	/**
	The rows are only the Visits whose Doula is no longer on the work (#2863).
	*/
	needsDoula?: boolean;
	/**
	The rows are only one kind of Visit (#2888).
	*/
	type?: ScheduleType;
	/**
	How many Visits have a Doula no longer on the work, said after the line when the rows are not narrowed to them.
	*/
	waitingCount?: number;
}

/**
The words a result line ends on for an order.
*/
export function scheduleOrderWords(order: ScheduleOrder = ''): string {
	return order === LATEST_ORDER ? 'latest first' : 'soonest first';
}

// Said as the badge on a Visit's row says it (`NO_LONGER_ON_THIS_ENGAGEMENT`), so the box, the tag and the line name one condition once.
const NO_LONGER_ON_THIS_WORK = 'no longer on this work';

/** What the result line says: the count, and while more exist how many there
 * are in all (`total`, the BFF's `visitCount`; a page without one says no
 * "of how many"), the days, whose, and the order. */
export function scheduleResultSummary(
	count: number,
	hasMore: boolean,
	days: ScheduleDays,
	timeZone: string,
	who: ScheduleWho = {},
	now: Date = new Date(),
	total?: number
): string {
	const line = scheduleRowsSummary(count, hasMore, days, timeZone, who, now, total);
	const waiting = who.waitingCount ?? 0;
	// The count of Visits with such a Doula is the second sentence of a line that names rows, never of one that narrowed to them.
	if (count === 0 || waiting === 0 || who.needsDoula) return line;
	return `${line} ${waiting} ${waiting === 1 ? 'has' : 'have'} a Doula ${NO_LONGER_ON_THIS_WORK}.`;
}

function scheduleRowsSummary(
	count: number,
	hasMore: boolean,
	days: ScheduleDays,
	timeZone: string,
	who: ScheduleWho,
	now: Date,
	total?: number
): string {
	const span = daysSpan(days, timeZone, now);
	// The Client's name a reader typed sits right after the Visits it narrows: `2 Visits for "Camille" from ...`.
	const about = who.search ? ` for "${who.search}"` : '';
	// The type sits before the noun, in lowercase as a running sentence says it: `3 postpartum Visits`.
	const kind = who.type ? `${visitTypeLabel(who.type).toLowerCase()} ` : '';
	const visits = `${kind}Visits`;
	const visit = `${kind}Visit`;
	if (count === 0) {
		if (who.needsDoula) {
			if (who.isMine) return `None of your ${visits}${about} has a Doula ${NO_LONGER_ON_THIS_WORK} ${span}.`;
			return `No ${visit}${about}${who.doulaName ? ` for ${who.doulaName}` : ''} has a Doula ${NO_LONGER_ON_THIS_WORK} ${span}.`;
		}
		if (who.isMine && who.doulaName) return `No ${visits}${about} for ${who.doulaName} that are also yours ${span}.`;
		if (who.isMine) return `You have no ${visits}${about} ${span}.`;
		if (who.doulaName) return `No ${visits}${about} for ${who.doulaName} ${span}.`;
		return `No ${visits}${about} ${span}.`;
	}
	const ordered = `, ${scheduleOrderWords(who.order)}`;
	const owner = who.isMine ? 'your' : (who.doulaName ? `${who.doulaName}'s` : '');
	const needing = who.needsDoula ? ` whose Doula is ${NO_LONGER_ON_THIS_WORK},` : '';
	if (count === 1) {
		const one = hasMore ? `first ${visit}` : `1 ${visit}`;
		return `Showing ${owner ? `${owner} ${one}` : (hasMore ? `the ${one}` : one)}${about}${needing} ${span}${ordered}.`;
	}
	const whose = owner ? `${owner} ` : '';
	let what = `${whose}${count} ${visits}`;
	if (hasMore) {
		what =
			total === undefined
				? `the first ${count}${owner ? ` of ${owner}` : ''} ${visits}`
				: `the first ${count} of ${whose}${Math.max(total, count)} ${visits}`;
	}
	return `Showing ${what}${about}${needing} ${span}${ordered}.`;
}

// A Client's name as long as a real one runs: a typed search longer than this is the one state that can move the list.
const SEARCH_RULER = 'Camille Ortiz';

/** The longest wording the result line can say, for the room the filter bar
 * holds for it so the list does not move between results. Days with a year
 * are the longest a day is said. */
export function scheduleResultRuler(longestDoulaName: string): string {
	const days = { from: '2000-09-29', to: '2000-12-29' };
	const now = new Date(1999, 0, 1);
	const postpartum: ScheduleType = 'postpartum';
	return [
		scheduleResultSummary(999, true, days, 'UTC', { doulaName: longestDoulaName }, now, 9999),
		scheduleResultSummary(999, true, days, 'UTC', { doulaName: longestDoulaName, order: LATEST_ORDER }, now, 9999),
		scheduleResultSummary(999, true, days, 'UTC', { isMine: true }, now, 9999),
		scheduleResultSummary(999, true, days, 'UTC', { search: SEARCH_RULER }, now, 9999),
		scheduleResultSummary(0, false, days, 'UTC', { isMine: true, doulaName: longestDoulaName }, now),
		scheduleResultSummary(999, true, days, 'UTC', { doulaName: longestDoulaName, order: LATEST_ORDER, waitingCount: 9999 }, now, 9999),
		scheduleResultSummary(999, true, days, 'UTC', { doulaName: longestDoulaName, order: LATEST_ORDER, needsDoula: true }, now, 9999),
		scheduleResultSummary(999, true, days, 'UTC', { search: SEARCH_RULER, needsDoula: true }, now, 9999),
		scheduleResultSummary(0, false, days, 'UTC', { needsDoula: true, doulaName: longestDoulaName, search: SEARCH_RULER }, now),
		scheduleResultSummary(1, false, days, 'UTC', { needsDoula: true, doulaName: longestDoulaName, search: SEARCH_RULER, order: LATEST_ORDER }, now),
		scheduleResultSummary(999, true, days, 'UTC', { doulaName: longestDoulaName, search: SEARCH_RULER, type: postpartum, order: LATEST_ORDER }, now, 9999),
		scheduleResultSummary(0, false, days, 'UTC', { needsDoula: true, doulaName: longestDoulaName, search: SEARCH_RULER, type: postpartum }, now)
	].toSorted((a, b) => b.length - a.length)[0];
}
