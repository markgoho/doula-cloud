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

/**
 * Who last changed the rule and when.
 */
export interface OnCallLastChanged {
	staffName: string;
	at: string;
}

/**
 * The rule as the settings read and save answer it: with who last changed
 * it, or `null` when it has never changed.
 */
export interface OnCallSettingsRead extends OnCallSettings {
	lastChanged: OnCallLastChanged | null;
}

export interface OnCallEffectBirth {
	engagementId: string;
	clientName: string;
}

/**
 * The births the typed rule would move on call today, against the rule the
 * Practice holds now.
 */
export interface OnCallEffect {
	stopToday: OnCallEffectBirth[];
	startToday: OnCallEffectBirth[];
	keepOwnStart: OnCallEffectBirth[];
}

const START_WEEK_FROM = 20;
const START_WEEK_TO = 42;
const GRACE_DAYS_FROM = 0;
const GRACE_DAYS_TO = 42;

export function settingsPath(practiceId: string): string {
	return `/api/practices/${practiceId}/on-call-settings`;
}

function isWholeNumberFrom(value: string, from: number, to: number): boolean {
	return /^\d{1,3}$/.test(value.trim()) && Number(value) >= from && Number(value) <= to;
}

/** Whether what was typed is a rule the BFF would accept, so the preview
 * asks only about a rule that could be saved. The BFF still says the bounds
 * in its own words when a save is refused. */
export function isTypedRuleSavable(
	startRule: OnCallSettings['startRule'],
	startWeek: string,
	graceDays: string
): boolean {
	return (
		(startRule === 'attachment_granted' ||
			isWholeNumberFrom(startWeek, START_WEEK_FROM, START_WEEK_TO)) &&
		isWholeNumberFrom(graceDays, GRACE_DAYS_FROM, GRACE_DAYS_TO)
	);
}

/** What the On-call rule screen says once a rule is saved (#2291): the
 * rule as saved, and which births follow it. */
export function describeSavedOnCall(settings: OnCallSettings): string {
	const opens =
		settings.startRule === 'gestational_week'
			? `at week ${settings.startWeek} of pregnancy`
			: 'on the day a Doula takes the birth';
	const runs =
		settings.graceDays === 0
			? 'ends on the due date'
			: `runs ${settings.graceDays} ${settings.graceDays === 1 ? 'day' : 'days'} past the due date`;
	return `On-call rule saved. On call now opens ${opens} and ${runs}, for every birth without its own start.`;
}

function names(births: readonly OnCallEffectBirth[]): string {
	const clients = births.map((birth) => birth.clientName);
	return clients.length < 2
		? clients.join('')
		: `${clients.slice(0, -1).join(', ')} and ${clients.at(-1)}`;
}

function births(count: number): string {
	return `${count} ${count === 1 ? 'birth' : 'births'}`;
}

/** The lines the preview says before the press: the births saving would
 * take off call today, the births it would put on, or that it changes none,
 * and the births with a start of their own that keep it. */
export function describeOnCallEffect(effect: OnCallEffect): string[] {
	const lines: string[] = [];
	if (effect.stopToday.length > 0) {
		lines.push(
			`Saving this takes ${births(effect.stopToday.length)} off call today: ${names(effect.stopToday)}.`
		);
	}
	if (effect.startToday.length > 0) {
		lines.push(
			`Saving this puts ${births(effect.startToday.length)} on call today: ${names(effect.startToday)}.`
		);
	}
	if (lines.length === 0) {
		lines.push('Saving this changes no birth on call today.');
	}
	if (effect.keepOwnStart.length === 1) {
		lines.push(`1 birth keeps the start set on that birth: ${names(effect.keepOwnStart)}.`);
	} else if (effect.keepOwnStart.length > 1) {
		lines.push(
			`${births(effect.keepOwnStart.length)} keep the start set on each of them: ${names(effect.keepOwnStart)}.`
		);
	}
	return lines;
}

/**
 * Reads the Practice's on-call rule.
 */
export async function loadOnCallSettings(
	fetcher: Fetcher,
	practiceId: string
): Promise<OnCallSettingsRead> {
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
): Promise<OnCallSettingsRead> {
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
 * Reads which births saving the typed rule would move on call today. Owner
 * and Admin only, and read only: nothing is saved. `signal` lets a newer
 * typed value drop the answer to an older one.
 */
export async function loadOnCallEffect(
	fetcher: Fetcher,
	practiceId: string,
	rule: OnCallSettings,
	signal?: AbortSignal
): Promise<OnCallEffect> {
	const query = new URLSearchParams({
		startRule: rule.startRule,
		startWeek: String(rule.startWeek),
		graceDays: String(rule.graceDays)
	});
	const response = await fetcher(`${settingsPath(practiceId)}/effect?${query}`, { signal });
	if (!response.ok) {
		throw await refusalError(response);
	}
	return response.json();
}
