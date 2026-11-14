/**
 * Who is on call, and where the holes are (#1093).
 *
 * The load, the URL shape and the words a reader meets, decoupled from
 * SvelteKit and the DOM so each can be unit-tested directly -- the split
 * `visitSchedule.ts` already draws for the Practice-wide schedule.
 *
 * Everything here is in **calendar days**, not instants, because that is
 * what an on-call window is: the BFF derives it from a due date and a
 * gestational week, both of them days in the Practice's own timezone
 * (ADR-0036). A coverage gap is the one thing with a clock on it -- "a
 * few drinks on Saturday night" -- so it alone carries instants.
 */
import type { Fetcher } from './fetcher.js';
import { formatCalendarWeekdayDay } from './dates.js';
import {
	THIS_WEEK,
	daysOf,
	daysSpan,
	stepDays,
	type Days,
	type DaysStep,
	type DaysView,
	type RangeErrors,
	type RangeLimits
} from './dayRange.js';
import { parseRefusal, refusalError } from './formErrors.js';
import type { OnCallSettings } from './onCallRule.js';
import { dayIn } from './zonedTime.js';

/** One live coverage gap, as the roster and the Engagement panel show
 * it. Mirrors the Go BFF's oncall.Gap. The roster sends no `reason`. */
export interface CoverageGap {
	id: string;
	engagementId: string;
	staffId: string;
	staffName: string;
	startsAt: string;
	endsAt: string;
	reason?: string;
	coveringStaffId?: string;
	coveringStaffName?: string;
}

/**
 * Two calendar days, both inclusive.
 */
export interface DayRange {
	start: string;
	end: string;
}

/**
 * Another birth a Doula holds on the same days, and the days both are hers
 * in the range.
 */
export interface AtOnceBirth {
	engagementId: string;
	clientName: string;
	days: DayRange;
}

/**
 * One Doula and the part of a window she is on call for.
 */
export interface RosterOnCall {
	staffId: string;
	name: string;
	from: string;
	to: string;
	narrowed: boolean;
	atOnce?: AtOnceBirth[];
}

/**
 * Who holds a birth tonight: the Doula whose days include today, or the one
 * covering her gap.
 */
export interface TonightHolder {
	staffId: string;
	name: string;
	coveringFor?: string;
}

export interface Tonight {
	holders: TonightHolder[];
	isNobody: boolean;
}

/**
 * One birth with on-call days in the range, or with none yet because no
 * Doula is on it.
 */
export interface RosterWindow {
	engagementId: string;
	clientName: string;
	dueDate?: string | null;
	window: DayRange;
	ownStart?: { startRule: OnCallSettings['startRule']; startWeek?: number };
	noDoula?: boolean;
	onCall: RosterOnCall[];
	gaps: CoverageGap[];
	unstaffedDays: DayRange[];
	uncovered: boolean;
	tonight?: Tonight;
}

/**
 * A birth that would have a window but cannot say when it is.
 */
export interface NoWindowRow {
	engagementId: string;
	clientName: string;
	reason: string;
	doulas?: { staffId: string; name: string }[];
}

export interface UnreachableGap {
	gapId: string;
	engagementId: string;
	clientName: string;
	startsAt: string;
	endsAt: string;
	isCovered: boolean;
}

/**
 * One Doula on the Practice's roster: the births she is on call for or
 * covers a gap on, the births whose days she shares, and the gaps of hers
 * in force. `atOnce` is absent where the reader may not hold it, a
 * contractor reading about a colleague.
 */
export interface RosterDoula {
	staffId: string;
	name: string;
	births: { engagementId: string; clientName: string; covering: boolean }[];
	atOnce?: { days: DayRange; engagementIds: string[] }[];
	unreachable: UnreachableGap[];
	isOnCallTonight: boolean;
}

export interface Roster {
	from: string;
	to: string;
	today: string;
	rule: OnCallSettings;
	windows: RosterWindow[];
	noWindow: NoWindowRow[];
	doulas: RosterDoula[];
}

/**
 * One Engagement's on-call panel.
 */
export interface EngagementOnCall {
	window?: DayRange;
	noWindowReason?: string;
	rule: OnCallSettings & { overridden: boolean };
	doulas: EngagementDoula[];
	gaps: CoverageGap[];
}

/** One Doula on a birth, with her narrowing and the days it leaves her
 * on call for. */
export interface EngagementDoula {
	staffId: string;
	name: string;
	from?: string;
	to?: string;
	onCallFrom?: string;
	onCallTo?: string;
}

export const TONIGHT_DAYS = 1;
export const SHORT_ON_CALL_DAYS = 7;

/** How many days ahead the roster looks when the address names none: today
 * through today plus 29, the Schedule's own 30 (#2026). */
export const DEFAULT_ON_CALL_DAYS = 30;

/**
 * The longest range the BFF serves.
 */
export const MOST_ON_CALL_DAYS = 92;

const PRESETS = [TONIGHT_DAYS, SHORT_ON_CALL_DAYS, THIS_WEEK, DEFAULT_ON_CALL_DAYS] as const;

const PARAMETER_OF_PRESET = new Map<number, string>([
	[TONIGHT_DAYS, 'tonight'],
	[SHORT_ON_CALL_DAYS, String(SHORT_ON_CALL_DAYS)],
	[THIS_WEEK, 'week']
]);
const PRESET_OF_PARAMETER = new Map([...PARAMETER_OF_PRESET].map(([preset, parameter]) => [parameter, preset]));

export const FROM_BEFORE_TODAY = 'Choose today or a later day. On call shows only births still to come.';
export const TOO_MANY_DAYS = `Choose a range of ${MOST_ON_CALL_DAYS} days or fewer.`;

export type OnCallDaysChoice = 'tonight' | '7' | 'week' | '30' | 'choose';

/**
 * The orders the roster is read in: the empty one is tonight's births first,
 * then by due date.
 */
export type OnCallOrder = '' | 'cover';

export const COVER_FIRST: OnCallOrder = 'cover';

export const ON_CALL_ORDER_OPTIONS: readonly { value: OnCallOrder; label: string }[] = [
	{ value: '', label: 'Tonight first' },
	{ value: COVER_FIRST, label: 'Needing cover first' }
];

/** What the address says, before any day is worked out: a preset or two
 * typed days, and which births. */
export interface OnCallView extends DaysView {
	/**
	The reader's own births.
	*/
	isMine: boolean;
	/**
	One Doula's births: a Staff id, or empty.
	*/
	doula: string;
	/**
	Only the births that need cover.
	*/
	needsCover: boolean;
	order: OnCallOrder;
}

/** Reads the narrowing off a URL's search parameters. `canMine` is whether
 * this reader is offered "Mine"; a reader not offered the Doula picker
 * (`withoutPicker`, carrying her own Staff id) has no roster to name anyone,
 * so her own id in `doula` is "Mine" and any other is ignored. */
export function onCallViewFromParameters(
	parameters: Pick<URLSearchParams, 'get' | 'has'>,
	canMine: boolean,
	withoutPicker?: { staffId: string | undefined }
): OnCallView {
	const hasChosen = parameters.has('from') || parameters.has('to');
	const doula = parameters.get('doula') ?? '';
	const isOwnDoula = withoutPicker !== undefined && doula !== '' && doula === withoutPicker.staffId;
	const days = parameters.get('days');
	return {
		preset: PRESET_OF_PARAMETER.get(days ?? '') ?? DEFAULT_ON_CALL_DAYS,
		chosen: hasChosen ? { from: parameters.get('from') ?? '', to: parameters.get('to') ?? '' } : undefined,
		isMine: canMine && (parameters.get('mine') === 'true' || isOwnDoula),
		doula: withoutPicker ? '' : doula,
		needsCover: !withoutPicker && parameters.get('cover') === 'needed',
		order: parameters.get('order') === COVER_FIRST ? COVER_FIRST : ''
	};
}

/** The screen's own URL for a view. Only what a reader chose is written, so
 * the default stays a clean path and `days=30` is never written. */
export function onCallHref(basePath: string, view: OnCallView): string {
	const parameters = new URLSearchParams();
	if (view.chosen) {
		parameters.set('from', view.chosen.from);
		parameters.set('to', view.chosen.to);
	} else {
		const days = PARAMETER_OF_PRESET.get(view.preset);
		if (days) parameters.set('days', days);
	}
	if (view.isMine) parameters.set('mine', 'true');
	if (view.doula) parameters.set('doula', view.doula);
	if (view.needsCover) parameters.set('cover', 'needed');
	if (view.order) parameters.set('order', view.order);
	const query = parameters.toString();
	return query ? `${basePath}?${query}` : basePath;
}

/**
 * The view on the days the screen opens on, keeping the rest.
 */
export function withDefaultDays(view: OnCallView): OnCallView {
	return { ...view, preset: DEFAULT_ON_CALL_DAYS, chosen: undefined };
}

/**
Whether the address names anything beyond the default days.
*/
export function isOnCallNarrowed(view: OnCallView): boolean {
	return view.preset !== DEFAULT_ON_CALL_DAYS || view.chosen !== undefined || view.isMine || view.doula !== '' || view.needsCover || view.order !== '';
}

/**
 * The refusals a chosen range meets before the BFF is asked: From before
 * today, and more days than it serves. Said on the fields in the words the
 * BFF uses.
 */
export function onCallLimits(today: string): RangeLimits {
	return {
		earliest: { day: today, message: FROM_BEFORE_TODAY },
		longest: { days: MOST_ON_CALL_DAYS, message: TOO_MANY_DAYS }
	};
}

/** What the BFF is asked: two calendar days that are always present, and
 * whose births. */
export interface OnCallFilters extends Days {
	staffId?: string;
	needsCover?: boolean;
	order?: OnCallOrder;
}

/**
 * The days a view reads. The roster offers no day before today, so a week
 * that began before today starts on it.
 */
export function onCallDaysOf(
	view: DaysView,
	timeZone: string,
	now: Date
): { days: Days; errors?: undefined } | { days?: undefined; errors: RangeErrors } {
	const today = dayIn(now, timeZone);
	const read = daysOf(view, timeZone, now, onCallLimits(today));
	if (!read.days || view.chosen || view.preset !== THIS_WEEK || read.days.from >= today) return read;
	return { days: { ...read.days, from: today } };
}

/**
 * The narrowing a view reads under, and whether there is anything to read.
 * Days that are not a range are answered in place with their errors and no
 * read. "Mine" beside a different Doula is read as "Mine" and then shown
 * empty (`isEmpty`), so the BFF is never sent a range that can never match.
 */
export function resolveOnCallRead(
	view: OnCallView,
	timeZone: string,
	ownStaffId: string | undefined,
	now: Date
): { days?: Days; errors?: RangeErrors; filters?: OnCallFilters; isEmpty: boolean } {
	const read = onCallDaysOf(view, timeZone, now);
	if (!read.days) return { errors: read.errors, isEmpty: false };
	const staffId = view.isMine ? ownStaffId : view.doula || undefined;
	const filters: OnCallFilters = { ...read.days };
	if (staffId) filters.staffId = staffId;
	if (view.needsCover) filters.needsCover = true;
	if (view.order) filters.order = view.order;
	return { days: read.days, filters, isEmpty: view.isMine && view.doula !== '' && view.doula !== ownStaffId };
}

export function rosterPath(practiceId: string, filters: OnCallFilters): string {
	const parameters = new URLSearchParams({ from: filters.from, to: filters.to });
	if (filters.staffId) parameters.set('staffId', filters.staffId);
	if (filters.needsCover) parameters.set('cover', 'needed');
	if (filters.order) parameters.set('order', filters.order);
	return `/api/practices/${practiceId}/on-call?${parameters.toString()}`;
}

/**
 * Why the BFF refused a roster's days, per field, from a 400's `details`.
 * Nothing for a refusal that is not about the days.
 */
export async function rosterRangeErrors(response: Response): Promise<RangeErrors | undefined> {
	if (response.status !== 400) return undefined;
	const { details } = await parseRefusal(response, { opaque5xx: false });
	const errors: RangeErrors = {};
	if (details?.from) errors.from = details.from;
	if (details?.to) errors.to = details.to;
	return Object.keys(errors).length > 0 ? errors : undefined;
}

export type OnCallStep = DaysStep<OnCallView>;

/**
 * The days one step earlier (`-1`) or later (`1`): the days the reader sees
 * moved by their own length. Earlier stops at today, because the roster
 * shows only births still to come.
 */
export function onCallStep(view: OnCallView, direction: -1 | 1, timeZone: string, now: Date): OnCallStep | undefined {
	const today = dayIn(now, timeZone);
	return stepDays(view, direction, timeZone, now, PRESETS, today, onCallLimits(today));
}

/**
 * The action an empty result offers, by what narrowed it: the days the
 * screen opens on first, then every Doula's births, then every birth.
 */
export function onCallEmptyNextStep(view: OnCallView): { label: string; view: OnCallView } | undefined {
	if (view.preset !== DEFAULT_ON_CALL_DAYS || view.chosen) {
		return { label: `Show the next ${DEFAULT_ON_CALL_DAYS} days`, view: withDefaultDays(view) };
	}
	if (view.isMine || view.doula) {
		return { label: "Show every Doula's births", view: { ...view, isMine: false, doula: '' } };
	}
	if (view.needsCover) return { label: 'Show every birth', view: { ...view, needsCover: false } };
	return undefined;
}

export function engagementOnCallPath(
	practiceId: string,
	engagementId: string
): string {
	return `/api/practices/${practiceId}/engagements/${engagementId}/on-call`;
}

export function gapsPath(practiceId: string, engagementId: string): string {
	return `/api/practices/${practiceId}/engagements/${engagementId}/coverage-gaps`;
}

export function narrowingPath(
	practiceId: string,
	engagementId: string,
	staffId: string
): string {
	return `/api/practices/${practiceId}/engagements/${engagementId}/attachments/${staffId}/on-call`;
}

/** Why a birth has no on-call days, said to the person reading the roster
 * rather than in the model's own words. Every reason the BFF can give
 * has a sentence here, and an unknown one is a plain statement rather
 * than a thrown error: a roster that refuses to render because the BFF
 * learned a new reason is worse than one that says less about one row.
 */
export function describeNoWindow(reason: string): string {
	switch (reason) {
		case 'no_due_date': {
			return 'No due date yet, so there are no on-call days.';
		}
		case 'no_due_date_no_doula': {
			return 'No due date and no Doula yet.';
		}
		case 'nobody_attached': {
			return 'No Doula is on this birth yet.';
		}
		case 'postpartum': {
			return 'Postpartum care is not on-call work.';
		}
		case 'completed': {
			return 'Care has ended.';
		}
		case 'ended_before_start': {
			// The BFF gives this reason after a loss and after an early live
			// birth alike (#2220), so it says what is true of both.
			return 'The pregnancy ended before on call would have started.';
		}
		default: {
			return 'There are no on-call days for this work.';
		}
	}
}

/** What one birth's cover says out loud. The lines under it, what is
 * missing, are the screen's. */
export function describeCoverage(window: Pick<RosterWindow, 'uncovered'>): string {
	return window.uncovered ? 'Needs cover' : 'Covered';
}

/**
 * The rule in force, said where the days are read.
 */
export function onCallRuleSentence(rule: OnCallSettings): string {
	const opens =
		rule.startRule === 'gestational_week'
			? `On call opens at week ${rule.startWeek} of pregnancy`
			: 'On call opens on the day a Doula takes the birth';
	if (rule.graceDays === 0) return `${opens} and ends on the due date.`;
	return `${opens} and runs ${rule.graceDays} ${rule.graceDays === 1 ? 'day' : 'days'} past the due date.`;
}

/**
 * How a birth's days are said: "Until Fri Dec 18" when they began on or
 * before today, else the two days.
 */
export type DaysOnCall = { until: string } | { day: string } | { from: string; to: string };

export function daysOnCall(range: DayRange, today: string): DaysOnCall {
	if (range.start <= today) return { until: range.end };
	return range.start === range.end ? { day: range.start } : { from: range.start, to: range.end };
}

/**
 * The same words as text: "Until Fri Dec 18", "Mon Nov 23 to Fri Dec 18".
 */
export function daysOnCallText(range: DayRange, today: string, timeZone: string, now: Date): string {
	const say = (day: string) => formatCalendarWeekdayDay(day, timeZone, now);
	const days = daysOnCall(range, today);
	if ('until' in days) return `Until ${say(days.until)}`;
	if ('day' in days) return say(days.day);
	return `${say(days.from)} to ${say(days.to)}`;
}

/**
 * Where the group headings go: only when the days include today. By default
 * the births on call tonight and then the ones starting later; under
 * "Needing cover first" the births that need cover and then the covered.
 */
export function onCallGroup(
	window: RosterWindow,
	order: OnCallOrder,
	today: string,
	timeZone: string,
	now: Date
): { key: string; label: string } {
	if (order === COVER_FIRST) {
		return window.uncovered ? { key: 'cover', label: 'Needing cover' } : { key: 'covered', label: 'Covered' };
	}
	return window.tonight
		? { key: 'tonight', label: `Tonight, ${formatCalendarWeekdayDay(today, timeZone, now)}` }
		: { key: 'later', label: 'Starting later' };
}

/**
 * The reader is "you" wherever she is named; anyone else is her name.
 */
export function personWords(staffId: string, name: string, ownStaffId: string | undefined, isLead = false): string {
	if (staffId !== ownStaffId) return name;
	return isLead ? 'You' : 'you';
}

/**
 * Who holds the birth tonight, the first line of the On call cell on every
 * birth that is on call tonight: the Doula holding it, the covering Doula and
 * whom she covers, or nobody.
 */
export function tonightLine(window: RosterWindow, ownStaffId?: string): string | undefined {
	const tonight = window.tonight;
	if (!tonight) return undefined;
	if (tonight.isNobody) return 'Tonight: nobody';
	const who = tonight.holders.map((holder) => {
		const name = personWords(holder.staffId, holder.name, ownStaffId);
		if (!holder.coveringFor) return name;
		const covered = window.gaps.find(
			(gap) => gap.coveringStaffId === holder.staffId && gap.staffName === holder.coveringFor
		);
		return `${name}, covering for ${covered ? personWords(covered.staffId, covered.staffName, ownStaffId) : holder.coveringFor}`;
	});
	return `Tonight: ${who.join('; ')}`;
}

/**
 * Who covers a gap, in one set of words for the birth's line and the Doulas
 * table's: the covering Doula by name or "you", "covered" when her name is
 * not in what the reader received, "nobody covering" when nobody does.
 */
export function coveringWords(
	isCovered: boolean,
	covering: { staffId?: string; name?: string } | undefined,
	ownStaffId: string | undefined
): string {
	if (!isCovered) return 'nobody covering';
	if (!covering?.staffId || !covering.name) return 'covered';
	return `${personWords(covering.staffId, covering.name, ownStaffId)} covering`;
}

export type SharedDays = { from: 'today' | string; to: string } | { day: string } | { isToday: true };

/**
 * The days two births share, said for an "Also" line: "today to Sun Nov 22"
 * when they began on or before today, else both days.
 */
export function sharedDays(range: DayRange, today: string): SharedDays {
	if (range.start === range.end) return range.start <= today ? { isToday: true } : { day: range.start };
	return { from: range.start <= today ? 'today' : range.start, to: range.end };
}

/**
 * What a birth's own start says against the Practice's rule: "On call from
 * week 38 for this birth, not week 37."
 */
export function ownStartWords(own: NonNullable<RosterWindow['ownStart']>, rule: OnCallSettings): string {
	const isOwnWeek = own.startRule === 'gestational_week';
	const instead =
		rule.startRule === 'gestational_week'
			? `${isOwnWeek ? '' : 'from '}week ${rule.startWeek}`
			: 'from the day a Doula took it';
	return isOwnWeek
		? `On call from week ${own.startWeek} for this birth, not ${instead}.`
		: `On call from the day a Doula took this birth, not ${instead}.`;
}

/**
 * The Order control's first option and the result line say the same words:
 * tonight first while today is in the days shown, soonest due first when it
 * is not.
 */
export function onCallOrderOptions(isTonightShown: boolean): readonly { value: OnCallOrder; label: string }[] {
	return [{ value: '', label: isTonightShown ? 'Tonight first' : 'Soonest due first' }, ON_CALL_ORDER_OPTIONS[1]!];
}

/**
 * The cover line's subject: "You" for the reader's own gap, else her name.
 */
export function gapSubject(gap: Pick<CoverageGap, 'staffId' | 'staffName'>, ownStaffId: string | undefined): string {
	return personWords(gap.staffId, gap.staffName, ownStaffId, true);
}

/**
 * How many births a Doula holds on one day, as the badge says it.
 */
export function atOnceLabel(others: number): string {
	return `On ${others + 1} births at once`;
}

export interface OnCallWho {
	/**
	The reader's own births are asked for.
	*/
	isMine?: boolean;
	/**
	The picked Doula's name, when one is.
	*/
	doulaName?: string;
	/**
	The Staff id whose births are asked for, for the one line that counts Doulas.
	*/
	staffId?: string;
	/**
	Only the births that need cover.
	*/
	needsCover?: boolean;
	order?: OnCallOrder;
}

interface SummaryCounts {
	births: number;
	needCover: number;
	tonightNobody: number;
	atOnce: number;
	isTonightShown: boolean;
}

function plural(count: number, one: string, many: string): string {
	return count === 1 ? one : many;
}

function summaryOf(counts: SummaryCounts, span: string, who: OnCallWho): string {
	if (counts.births === 0) {
		if (who.needsCover) return `No births need cover ${span}.`;
		if (who.isMine) return `You are on call for no births ${span}.`;
		if (who.doulaName) return `No births for ${who.doulaName} ${span}.`;
		return `No births on call ${span}.`;
	}
	const whose = who.isMine ? 'your ' : (who.doulaName ? `${who.doulaName}'s ` : '');
	const ordered =
		who.order === COVER_FIRST ? 'needing cover first' : (counts.isTonightShown ? 'tonight first' : 'soonest due first');
	const what = `${counts.births} ${plural(counts.births, 'birth', 'births')}`;
	const state = who.needsCover ? ` that ${plural(counts.births, 'needs', 'need')} cover` : ' on call';
	const sentences = [`Showing ${whose}${what}${state} ${span}, ${ordered}.`];
	if (!who.needsCover) {
		if (counts.needCover === 0) sentences.push('Every one is covered.');
		else if (counts.tonightNobody === 0) {
			sentences.push(`${counts.needCover} ${plural(counts.needCover, 'needs', 'need')} cover.`);
		} else if (counts.needCover === 1) sentences.push('1 needs cover, tonight.');
		else sentences.push(`${counts.needCover} need cover, ${counts.tonightNobody} of them tonight.`);
	}
	if (counts.atOnce > 0) {
		if (who.isMine) sentences.push('You are on two births at once.');
		else if (who.doulaName) sentences.push(`${who.doulaName} is on two births at once.`);
		else sentences.push(`${counts.atOnce} ${plural(counts.atOnce, 'Doula', 'Doulas')} on two births at once.`);
	}
	return sentences.join(' ');
}

/** What the result line says: how many births, how many need cover and how
 * many of those tonight, who is on two births at once, the days, whose, and
 * the order. */
export function onCallResultSummary(
	roster: Roster,
	days: Days,
	timeZone: string,
	who: OnCallWho = {},
	now: Date = new Date()
): string {
	const doulas = roster.doulas.filter(
		(doula) => (doula.atOnce?.length ?? 0) > 0 && (who.staffId === undefined || doula.staffId === who.staffId)
	);
	return summaryOf(
		{
			births: roster.windows.length,
			needCover: roster.windows.filter((window) => window.uncovered).length,
			tonightNobody: roster.windows.filter((window) => window.tonight?.isNobody).length,
			atOnce: doulas.length,
			isTonightShown: days.from === roster.today
		},
		daysSpan(days, timeZone, now),
		who
	);
}

/** The longest wording the result line can say, for the room the filter bar
 * holds for it so the list does not move between results. */
export function onCallResultRuler(longestDoulaName: string): string {
	const days = { from: '2000-09-29', to: '2000-12-29' };
	const now = new Date(1999, 0, 1);
	const span = daysSpan(days, 'UTC', now);
	const many = { births: 999, needCover: 999, tonightNobody: 999, atOnce: 999, isTonightShown: true };
	return [
		summaryOf(many, span, { doulaName: longestDoulaName }),
		summaryOf(many, span, { doulaName: longestDoulaName, order: COVER_FIRST }),
		summaryOf(many, span, { isMine: true }),
		summaryOf(many, span, {}),
		summaryOf({ ...many, births: 0 }, span, { doulaName: longestDoulaName })
	].toSorted((a, b) => b.length - a.length)[0]!;
}

/**
 * Loads one Engagement's on-call panel.
 */
export async function loadEngagementOnCall(
	fetcher: Fetcher,
	practiceId: string,
	engagementId: string
): Promise<EngagementOnCall> {
	const response = await fetcher(
		engagementOnCallPath(practiceId, engagementId)
	);
	if (!response.ok) {
		throw await refusalError(response);
	}
	return response.json();
}

/**
 * What a coverage gap's form sends.
 */
export interface GapDraft {
	staffId: string;
	startsAt: string;
	endsAt: string;
	reason?: string;
	coveringStaffId?: string;
}

/**
 * Records a coverage gap.
 */
export async function saveGap(
	fetcher: Fetcher,
	practiceId: string,
	engagementId: string,
	draft: GapDraft,
	gapId?: string
): Promise<CoverageGap> {
	const path = gapsPath(practiceId, engagementId);
	const response = await fetcher(gapId ? `${path}/${gapId}` : path, {
		method: gapId ? 'PUT' : 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(draft),
	});
	if (!response.ok) {
		throw await refusalError(response);
	}
	return response.json();
}

/**
 * Clears a coverage gap: she can be reached again.
 */
export async function clearGap(
	fetcher: Fetcher,
	practiceId: string,
	engagementId: string,
	gapId: string
): Promise<void> {
	const response = await fetcher(
		`${gapsPath(practiceId, engagementId)}/${gapId}`,
		{
			method: 'DELETE',
		}
	);
	if (!response.ok) {
		throw await refusalError(response);
	}
}

/**
 * Sets or clears one Doula's narrowing.
 */
export async function saveNarrowing(
	fetcher: Fetcher,
	practiceId: string,
	engagementId: string,
	staffId: string,
	days: { from?: string; to?: string }
): Promise<void> {
	const response = await fetcher(
		narrowingPath(practiceId, engagementId, staffId),
		{
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(days),
		}
	);
	if (!response.ok) {
		throw await refusalError(response);
	}
}
