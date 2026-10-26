/**
 * Setting up a Practice, as a short journey of question pages (#2049).
 *
 * Signup was one page of eight decisions. The founder decided on
 * 2026-10-26 that it is three question pages and a check page, the
 * shape of a Client's details journey: You (name, email, password),
 * Your Practice (its name), Where you work (state and timezone), then
 * Check your answers with the agreement and the one button that
 * creates the Practice. Nothing is sent before that button.
 *
 * ## Where the draft lives, and why
 *
 * In this module's state, mirrored to `sessionStorage` (ADR-0059's
 * second tier: a reload in the same tab keeps what was typed). Not a
 * server draft: there is no account and no Practice to hang one off
 * until the last press, and #1715's server draft is for intake. The
 * mirror is the same guarded shape `intakeDraft.svelte.ts` uses: every
 * read and write is in a try, so a browser that refuses storage gets a
 * journey that works one navigation at a time.
 *
 * **The password is never written to storage.** It stays in module
 * state, so it survives a move between the pages and not a reload. A
 * reload costs her one box, and the check page's button asks for it
 * again on her first page; a password at rest in the tab's storage
 * would cost more than that.
 */

import { resolve } from '$app/paths';
import type { JourneyStep } from '#lib/components/organisms/StepRail.svelte';
import type { FormError } from '#lib/formSubmission.svelte.js';
import { CHANGE_QUERY } from '#lib/intakeJourney.js';
import { passwordRefusal } from '#lib/passwordRule.js';
import { TIMEZONE_NEEDED, detectTimezone } from '#lib/timezones.js';

/**
 * Every control's id, one table for the pages, the check page's links and
 * the BFF's Details keys (docs/api-design.md section 7). The Email and
 * Password are not BFF fields: Identity Platform takes them, and the BFF
 * reads the address off the ID token (#614).
 */
export const FIELD_IDS = {
	firstName: 'signup-first-name',
	lastName: 'signup-last-name',
	email: 'signup-email',
	password: 'signup-password',
	practiceName: 'signup-practice-name',
	workState: 'signup-work-state',
	timezone: 'signup-timezone'
} as const;

/**
 * POST /api/staff/signup's Details, keyed by the DTO's own JSON names.
 */
export const BFF_FIELD_IDS = {
	firstName: FIELD_IDS.firstName,
	lastName: FIELD_IDS.lastName,
	practiceName: FIELD_IDS.practiceName,
	workState: FIELD_IDS.workState,
	timezone: FIELD_IDS.timezone
};

export type SignupStepId = 'you' | 'practice' | 'where';

export interface SignupStep {
	id: SignupStepId;
	label: string;
	href: string;
	fieldIds: readonly string[];
}

/**
Names the rail's landmark on every page.
*/
export const JOURNEY_LABEL = 'Setting up your Practice';

/**
GOV.UK's section caption above each question.
*/
export const JOURNEY_CAPTION = 'Set up your Practice';

/**
 * The three question pages, in order. `/signup` is the first, so every
 * door that says "Set up a Practice" opens the journey at its start.
 */
export const SIGNUP_STEPS: readonly SignupStep[] = [
	{
		id: 'you',
		label: 'You',
		href: resolve('/(signed-out)/signup'),
		fieldIds: [FIELD_IDS.firstName, FIELD_IDS.lastName, FIELD_IDS.email, FIELD_IDS.password]
	},
	{
		id: 'practice',
		label: 'Your Practice',
		href: resolve('/(signed-out)/signup/practice'),
		fieldIds: [FIELD_IDS.practiceName]
	},
	{
		id: 'where',
		label: 'Where you work',
		href: resolve('/(signed-out)/signup/where'),
		fieldIds: [FIELD_IDS.workState, FIELD_IDS.timezone]
	}
];

export const CHECK_HREF = resolve('/(signed-out)/signup/check');

export interface SignupAnswers {
	firstName: string;
	lastName: string;
	email: string;
	practiceName: string;
	workStateName: string;
	timezone: string;
}

function blankAnswers(): SignupAnswers {
	return {
		firstName: '',
		lastName: '',
		email: '',
		practiceName: '',
		workStateName: '',
		// #1166: opened on the zone the browser reports, which GOV.UK's
		// Select guidance allows for something the service already knows.
		timezone: detectTimezone()
	};
}

const STORAGE_KEY = 'signup-draft';

interface Stored {
	answers: SignupAnswers;
	visited: SignupStepId[];
}

function readStored(): Stored | undefined {
	try {
		const raw = sessionStorage.getItem(STORAGE_KEY);
		if (raw === null) return undefined;
		const stored = JSON.parse(raw) as Partial<Stored>;
		return {
			answers: { ...blankAnswers(), ...stored.answers },
			visited: Array.isArray(stored.visited) ? stored.visited : []
		};
	} catch {
		return undefined;
	}
}

function writeStored(value: Stored): void {
	try {
		sessionStorage.setItem(STORAGE_KEY, JSON.stringify(value));
	} catch {
		// Nothing to recover: the draft still lives in module state.
	}
}

function clearStored(): void {
	try {
		sessionStorage.removeItem(STORAGE_KEY);
	} catch {
		// As above.
	}
}

export class SignupDraft {
	answers = $state<SignupAnswers>(blankAnswers());
	/**
	Module state only: see the module comment.
	*/
	password = $state('');
	/**
	The pages she has continued from, which the rail marks completed.
	*/
	visited = $state<SignupStepId[]>([]);
	/**
	 * What the check page's button was refused for, kept so the page each
	 * entry links to can say it beside its own control.
	 */
	refusals = $state<FormError[]>([]);

	constructor() {
		const stored = readStored();
		if (stored) {
			this.answers = stored.answers;
			this.visited = stored.visited;
		}
	}

	#persist(): void {
		writeStored({ answers: this.answers, visited: this.visited });
	}

	update(partial: Partial<SignupAnswers>): void {
		this.answers = { ...this.answers, ...partial };
		this.#persist();
	}

	visit(id: SignupStepId): void {
		if (!this.visited.includes(id)) this.visited = [...this.visited, id];
		this.#persist();
	}

	/**
	 * The refusals that belong to one page's controls, as that page shows
	 * them: linked to its own control, not back to itself.
	 */
	refusalsFor(id: SignupStepId): FormError[] {
		return this.refusals
			.filter((refusal) => isOnStep(refusal, id))
			.map(({ message, targetId }) => ({ message, targetId }));
	}

	/**
	Called when a page continues: its own refusals are answered.
	*/
	settle(id: SignupStepId): void {
		this.refusals = this.refusals.filter((refusal) => !isOnStep(refusal, id));
	}

	/**
	After the Practice is created, and between tests.
	*/
	clear(): void {
		this.answers = blankAnswers();
		this.password = '';
		this.visited = [];
		this.refusals = [];
		clearStored();
	}
}

export const signupDraft = new SignupDraft();

function stepById(id: SignupStepId): SignupStep {
	return SIGNUP_STEPS.find((step) => step.id === id)!;
}

function isOnStep(refusal: FormError, id: SignupStepId): boolean {
	return refusal.targetId !== undefined && stepById(id).fieldIds.includes(refusal.targetId);
}

/**
The rail's data. No `current` on the check page.
*/
export function railSteps(currentId: SignupStepId | undefined, visited: readonly SignupStepId[]): JourneyStep[] {
	return SIGNUP_STEPS.map((step) => ({
		label: step.label,
		href: step.href,
		status: step.id === currentId ? 'current' : (visited.includes(step.id) ? 'completed' : 'todo')
	}));
}

/**
A step's href with the query that sends its Continue back to the check page.
*/
export function changeHref(id: SignupStepId): string {
	return `${stepById(id).href}?${CHANGE_QUERY}`;
}

/**
Where Continue goes: the next page, or the check page after the last.
*/
export function nextHref(id: SignupStepId, isFromCheck: boolean): string {
	if (isFromCheck) return CHECK_HREF;
	const index = SIGNUP_STEPS.findIndex((step) => step.id === id);
	return SIGNUP_STEPS[index + 1]?.href ?? CHECK_HREF;
}

/**
Where Back goes: the previous page, or the landing from the first.
*/
export function backHref(id: SignupStepId, isFromCheck: boolean): string {
	if (isFromCheck) return CHECK_HREF;
	const index = SIGNUP_STEPS.findIndex((step) => step.id === id);
	return SIGNUP_STEPS[index - 1]?.href ?? resolve('/(signed-out)');
}

/*
 * Every message starts with the field's own noun and says what to do,
 * GOV.UK's rule. The Practice name has none: it is optional, and the BFF
 * gives an empty one her own name (#1536).
 */
export async function youRefusals(answers: SignupAnswers, password: string): Promise<FormError[]> {
	const found: FormError[] = [];
	if (answers.firstName.trim() === '')
		found.push({ message: 'Enter your first name', targetId: FIELD_IDS.firstName });
	if (answers.lastName.trim() === '')
		found.push({ message: 'Enter your last name', targetId: FIELD_IDS.lastName });
	if (answers.email.trim() === '')
		found.push({ message: 'Enter your email address', targetId: FIELD_IDS.email });
	if (password === '') {
		found.push({ message: 'Enter a password', targetId: FIELD_IDS.password });
	} else {
		// #1539: the common-password half of the rule is checked here, in
		// the browser, because Identity Platform has no such list.
		const refusal = await passwordRefusal(password);
		if (refusal) found.push({ message: refusal, targetId: FIELD_IDS.password });
	}
	return found;
}

export function whereRefusals(answers: SignupAnswers): FormError[] {
	const found: FormError[] = [];
	if (answers.workStateName === '')
		found.push({ message: 'Choose the state you work from', targetId: FIELD_IDS.workState });
	if (answers.timezone === '') found.push({ message: TIMEZONE_NEEDED, targetId: FIELD_IDS.timezone });
	return found;
}

/**
 * The check page's refusals, each linked to the page that asks the thing
 * refused (#2049). An entry for no control stays plain text.
 */
export function linkedToSteps(errors: FormError[]): FormError[] {
	return errors.map((error) => {
		const step = SIGNUP_STEPS.find((candidate) => error.targetId && candidate.fieldIds.includes(error.targetId));
		return step ? { ...error, href: changeHref(step.id) } : error;
	});
}
