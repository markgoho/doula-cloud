/**
 * The act by which a Doula says once that she cannot be reached, and a
 * Coverage gap is written on each of her births on call during that time
 * (#2934). The paths, the two BFF calls, and the words a reader meets,
 * decoupled from SvelteKit so each can be unit-tested directly.
 */
import { formatGapTimes } from './coverageGap.js';
import type { Fetcher } from './fetcher.js';
import { refusalError } from './formErrors.js';
import { addDays } from './dayRange.js';
import {
	MOST_ON_CALL_DAYS,
	onCallHref,
	onCallViewFromParameters,
	rosterPath,
	type CoverageGap,
	type Roster
} from './onCall.js';

/**
The answer "Nobody yet" for a birth's cover, in a draft and a radio group.
*/
export const NOBODY_YET = 'nobody';

export const BIRTHS_CHANGED = 'The births on call changed since you checked. Check them again.';
export const NOT_ON_CALL_THEN = 'You are not on call for any birth then.';
export const NOTHING_TO_RECORD = 'There is nothing more to record.';

export interface CoverOption {
	staffId: string;
	name: string;
}

export interface PreviewBirth {
	engagementId: string;
	clientName: string;
	startsAt: string;
	endsAt: string;
	coverOptions: CoverOption[];
	existingGap?: CoverageGap;
}

export interface GapPreview {
	births: PreviewBirth[];
}

export type JourneyStepSlug = 'doula' | 'when' | 'cover' | 'check';

export const STEP_LABELS: Record<Exclude<JourneyStepSlug, 'check'>, string> = {
	doula: 'Doula',
	when: 'When',
	cover: 'Cover'
};

export function cannotBeReachedPath(practiceId: string, step: JourneyStepSlug = 'doula'): string {
	const base = `/practices/${practiceId}/on-call/cannot-be-reached`;
	return step === 'doula' ? base : `${base}/${step}`;
}

/**
What the journey's addresses carry between pages: the roster's own query to come back to, and the Doula the roster was narrowed to.
*/
export interface JourneyAddress {
	back: string;
	doula: string;
}

export function journeyAddressFrom(parameters: Pick<URLSearchParams, 'get'>): JourneyAddress {
	return { back: parameters.get('back') ?? '', doula: parameters.get('doula') ?? '' };
}

export function journeyHref(practiceId: string, step: JourneyStepSlug, address: JourneyAddress, extra: Record<string, string> = {}): string {
	const parameters = new URLSearchParams();
	if (address.back) parameters.set('back', address.back);
	if (address.doula) parameters.set('doula', address.doula);
	for (const [key, value] of Object.entries(extra)) parameters.set(key, value);
	const query = parameters.toString();
	const path = cannotBeReachedPath(practiceId, step);
	return query ? `${path}?${query}` : path;
}

/**
 * The roster the act was started from. `back` holds the roster's own query;
 * it is read back through the roster's own parser and written by its own
 * writer, so nothing the address held is followed raw.
 */
export function rosterHrefFor(
	practiceId: string,
	back: string,
	canMine: boolean,
	withoutPicker?: { staffId: string | undefined }
): string {
	const base = `/practices/${practiceId}/on-call`;
	return onCallHref(base, onCallViewFromParameters(new URLSearchParams(back), canMine, withoutPicker));
}

export function firstName(fullName: string): string {
	return fullName.trim().split(/\s+/, 1)[0]!;
}

export function whenQuestion(isOwnerView: boolean, doulaName: string): string {
	return isOwnerView && doulaName ? `When can ${firstName(doulaName)} not be reached?` : 'When can you not be reached?';
}

export function entryLabel(isOwnerView: boolean): string {
	return isOwnerView ? 'Record a time a Doula cannot be reached' : 'Record a time you cannot be reached';
}

export function birthLegend(birth: PreviewBirth, timeZone: string, now: Date): string {
	return `${birth.clientName}, ${formatGapTimes(birth.startsAt, birth.endsAt, timeZone, now)}`;
}

export function coverRequired(clientName: string): string {
	return `Choose who is covering ${clientName}'s birth, or Nobody yet`;
}

export function nobodyElse(clientName: string): string {
	return `Nobody else is on ${clientName}'s birth, so nobody can cover.`;
}

export function recordButton(births: number): string {
	return `Record it on ${births} ${births === 1 ? 'birth' : 'births'}`;
}

export function coverAnswer(coveringName: string | undefined): string {
	return coveringName ? `${coveringName} covering` : 'Nobody covering';
}

/**
What the roster says after the act: how many births, and who is told.
*/
export function recordedMessage(subject: string, births: number, uncovered: number): string {
	const recorded = `The time ${subject} cannot be reached is recorded on ${births} ${births === 1 ? 'birth' : 'births'}.`;
	if (uncovered === 0) return recorded;
	const told = births === 1 ? 'it has nobody covering' : `of the ${uncovered} with nobody covering`;
	return `${recorded} Every Owner and Admin is told ${told}.`;
}

/**
Whether a birth takes an answer: another Doula is on it, and no gap of hers is there already.
*/
export function isAsked(birth: PreviewBirth): boolean {
	return birth.existingGap === undefined && birth.coverOptions.length > 0;
}

export function isWritable(birth: PreviewBirth): boolean {
	return birth.existingGap === undefined;
}

/**
The cover each writable birth will be recorded with: the chosen Doula, or the empty string for nobody. Undefined where a birth that takes an answer has none.
*/
export function coverFor(birth: PreviewBirth, answers: Record<string, string>): string | undefined {
	if (!isAsked(birth)) return '';
	const answer = answers[birth.engagementId];
	if (answer === undefined || answer === '') return undefined;
	if (answer === NOBODY_YET) return '';
	return birth.coverOptions.some((option) => option.staffId === answer) ? answer : undefined;
}

export function coverageGapPreviewPath(practiceId: string, staffId: string, startsAt: Date, endsAt: Date): string {
	const parameters = new URLSearchParams({ staffId, startsAt: startsAt.toISOString(), endsAt: endsAt.toISOString() });
	return `/api/practices/${practiceId}/coverage-gaps/preview?${parameters.toString()}`;
}

export async function previewGaps(fetcher: Fetcher, practiceId: string, staffId: string, startsAt: Date, endsAt: Date): Promise<GapPreview> {
	const response = await fetcher(coverageGapPreviewPath(practiceId, staffId, startsAt, endsAt));
	if (!response.ok) throw await refusalError(response);
	return response.json();
}

/**
 * The Doulas an Owner or Admin may name: those on a birth with days in the
 * roster's longest range from `today`, read from the roster itself.
 */
export async function loadDoulaChoices(fetcher: Fetcher, practiceId: string, today: string): Promise<CoverOption[]> {
	const response = await fetcher(rosterPath(practiceId, { from: today, to: addDays(today, MOST_ON_CALL_DAYS - 1) }));
	if (!response.ok) throw await refusalError(response);
	const roster = (await response.json()) as Roster;
	return roster.doulas.filter((doula) => doula.births.length > 0).map((doula) => ({ staffId: doula.staffId, name: doula.name }));
}

export interface RecordGapsRequest {
	staffId: string;
	startsAt: string;
	endsAt: string;
	reason?: string;
	births: { engagementId: string; coveringStaffId?: string }[];
}

export async function recordGaps(fetcher: Fetcher, practiceId: string, body: RecordGapsRequest): Promise<CoverageGap[]> {
	const response = await fetcher(`/api/practices/${practiceId}/coverage-gaps`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
	if (!response.ok) throw await refusalError(response);
	return ((await response.json()) as { gaps: CoverageGap[] }).gaps;
}
