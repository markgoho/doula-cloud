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
import { apiErrorMessage } from './apiErrorMessage.js';
import { refusalError } from './formErrors.js';

/** One live coverage gap, as the roster and the Engagement panel show
 * it. Mirrors the Go BFF's oncall.Gap. */
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
 * One Doula and the part of a window she is on call for.
 */
export interface RosterOnCall {
	staffId: string;
	name: string;
	from: string;
	to: string;
	narrowed: boolean;
}

/**
 * One birth with a live window in the range.
 */
export interface RosterWindow {
	engagementId: string;
	clientName: string;
	dueDate?: string;
	window: DayRange;
	onCall: RosterOnCall[];
	gaps: CoverageGap[];
	unstaffedDays: DayRange[];
	uncovered: boolean;
}

/**
 * A birth that would have a window but cannot say when it is.
 */
export interface NoWindowRow {
	engagementId: string;
	clientName: string;
	reason: string;
}

/** One Doula on the Practice's roster. `concurrentWindows` is absent
 * where the reader may not hold it -- a contractor reading about a
 * colleague. */
export interface RosterDoula {
	staffId: string;
	name: string;
	available: boolean;
	concurrentWindows?: number;
}

export interface Roster {
	from: string;
	to: string;
	windows: RosterWindow[];
	noWindow: NoWindowRow[];
	doulas: RosterDoula[];
}

/**
 * A Practice's on-call rule.
 */
export interface OnCallSettings {
	startRule: 'gestational_week' | 'attachment_granted';
	startWeek: number;
	graceDays: number;
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

/**
 * The days a reader chose, as the URL carries them.
 */
export interface RosterRange {
	from: string;
	to: string;
}

function padTwoDigits(n: number): string {
	return n.toString().padStart(2, '0');
}

/** `<input type="date">`'s own value shape for a Date, in the reader's
 * zone -- local getters, never `toISOString`, which names yesterday for
 * anybody behind UTC after their evening. */
export function toDayValue(date: Date): string {
	return `${date.getFullYear()}-${padTwoDigits(date.getMonth() + 1)}-${padTwoDigits(date.getDate())}`;
}

/** The days the screen opens on when the URL names none: today, and
 * only today. "Who is on call tonight" is the question the roster
 * exists for, and a reader widens it from there. */
export function defaultRange(now: Date = new Date()): RosterRange {
	const today = toDayValue(now);
	return { from: today, to: today };
}

/** Reads the range off a URL's search parameters, falling back to today
 * for a day the URL does not name. */
export function rangeFromParameters(
	parameters: URLSearchParams,
	now: Date = new Date()
): RosterRange {
	const fallback = defaultRange(now);
	return {
		from: parameters.get('from') ?? fallback.from,
		to: parameters.get('to') ?? fallback.to,
	};
}

/** The screen's own URL for a range. Only a day the reader actually
 * chose is written, so the unnarrowed roster stays a clean path. */
export function rosterHref(
	basePath: string,
	range: RosterRange,
	now: Date = new Date()
): string {
	const fallback = defaultRange(now);
	const parameters = new URLSearchParams();
	if (range.from && range.from !== fallback.from)
		parameters.set('from', range.from);
	if (range.to && range.to !== fallback.to) parameters.set('to', range.to);
	const query = parameters.toString();
	return query ? `${basePath}?${query}` : basePath;
}

export function rosterPath(practiceId: string, range: RosterRange): string {
	const parameters = new URLSearchParams({ from: range.from, to: range.to });
	return `/api/practices/${practiceId}/on-call?${parameters.toString()}`;
}

export function engagementOnCallPath(
	practiceId: string,
	engagementId: string
): string {
	return `/api/practices/${practiceId}/engagements/${engagementId}/on-call`;
}

export function settingsPath(practiceId: string): string {
	return `/api/practices/${practiceId}/on-call-settings`;
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

/** Why a birth has no window, said to the person reading the roster
 * rather than in the model's own words. Every reason the BFF can give
 * has a sentence here, and an unknown one is a plain statement rather
 * than a thrown error: a roster that refuses to render because the BFF
 * learned a new reason is worse than one that says less about one row.
 */
export function describeNoWindow(reason: string): string {
	switch (reason) {
		case 'no_due_date': {
			return 'No due date recorded, so there is no window to work out.';
		}
		case 'nobody_attached': {
			return 'Nobody is on this birth yet.';
		}
		case 'postpartum': {
			return 'Postpartum care is not on-call work.';
		}
		case 'not_active': {
			return 'Care has not started, or has ended.';
		}
		case 'ended_before_start': {
			return 'The baby arrived before the window would have opened.';
		}
		default: {
			return 'There is no on-call window for this birth.';
		}
	}
}

/** What one window's state says out loud, for the reader scanning the
 * roster and for the live region that announces a narrowing change. */
export function describeCoverage(window: RosterWindow): string {
	if (!window.uncovered) {
		return window.onCall.length === 1
			? 'Covered'
			: `Covered by ${window.onCall.length}`;
	}
	const uncoveredGaps = window.gaps.filter(
		(gap) => gap.coveringStaffId === undefined
	).length;
	if (uncoveredGaps > 0) {
		return uncoveredGaps === 1
			? 'A gap needs cover'
			: `${uncoveredGaps} gaps need cover`;
	}
	return 'Nobody on call for part of this';
}

/** The roster's own summary line: what the reader is looking at, in
 * words, because a bare count reads as nothing having happened. */
export function rosterSummary(roster: Roster): string {
	if (roster.windows.length === 0) {
		return 'No births are on call in these days.';
	}
	const births =
		roster.windows.length === 1 ? '1 birth' : `${roster.windows.length} births`;
	const uncovered = roster.windows.filter((window) => window.uncovered).length;
	if (uncovered === 0) {
		return `${births} on call, all covered.`;
	}
	return `${births} on call, ${uncovered} needing cover.`;
}

/** Every Doula carrying more than one live window in the range, which
 * is the "is anyone covering two births at once" half of the screen. */
export function doublyBooked(roster: Roster): RosterDoula[] {
	return roster.doulas.filter((doula) => (doula.concurrentWindows ?? 0) > 1);
}

/** Loads the roster for a range. Throws with the response body's own
 * message; the route's `load` maps status codes to SvelteKit errors
 * before calling this. */
export async function loadRoster(
	fetcher: Fetcher,
	practiceId: string,
	range: RosterRange
): Promise<Roster> {
	const response = await fetcher(rosterPath(practiceId, range));
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
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
 * Reads the Practice's on-call rule.
 */
export async function loadOnCallSettings(
	fetcher: Fetcher,
	practiceId: string
): Promise<OnCallSettings> {
	const response = await fetcher(settingsPath(practiceId));
	if (!response.ok) {
		throw await refusalError(response);
	}
	return response.json();
}

/** States the Practice's on-call rule. A RefusalError, so the screen can
 * put the BFF's own sentence on the control it named. */
export async function saveOnCallSettings(
	fetcher: Fetcher,
	practiceId: string,
	settings: OnCallSettings
): Promise<OnCallSettings> {
	const response = await fetcher(settingsPath(practiceId), {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(settings),
	});
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
