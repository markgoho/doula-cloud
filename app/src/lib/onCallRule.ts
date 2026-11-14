/**
 * The Practice's on-call rule (#1093, #2291): when on call opens for a
 * birth and how long it runs past the due date. Its read, its save and what
 * a save says, apart from the roster so the settings screen and the
 * Settings summary read the rule without the roster's words.
 */
import type { Fetcher } from './fetcher.js';
import { refusalError } from './formErrors.js';

/**
 * A Practice's on-call rule.
 */
export interface OnCallSettings {
	startRule: 'gestational_week' | 'attachment_granted';
	startWeek: number;
	graceDays: number;
}

export function settingsPath(practiceId: string): string {
	return `/api/practices/${practiceId}/on-call-settings`;
}

/** What the On call settings screen says once a rule is saved (#2291):
 * the rule as saved, and that it reaches every birth the Practice is
 * carrying, because nothing stores a window -- each is worked out on
 * every read from the due date and this rule. */
export function describeSavedOnCall(settings: OnCallSettings): string {
	const opens =
		settings.startRule === 'gestational_week'
			? `at week ${settings.startWeek} of pregnancy`
			: 'on the day a doula takes the birth';
	const runs =
		settings.graceDays === 0
			? 'ends on the due date'
			: `runs ${settings.graceDays} ${settings.graceDays === 1 ? 'day' : 'days'} past the due date`;
	return `On call saved. On call now opens ${opens} and ${runs}, for every birth this Practice is carrying, including births already under way.`;
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
