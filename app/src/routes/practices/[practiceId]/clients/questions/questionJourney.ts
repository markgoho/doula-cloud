/**
 * What a question page needs to know about the journey it is in (#1610).
 *
 * The date of birth, the email address, the phone number, the address
 * and a Practice's own sections are a Client's details, added from her
 * record (#1610, ADR-0017's amendment of 2026-10-02). Intake asked them
 * too until #1611 made it one question, the name. Each question is one
 * component in this folder, and the journey hands it one of these.
 *
 * The journey's object reads its own module state: the continuum sweep
 * mounts a route with no layout in front of it, so a route's fixture
 * seeds that state at import time.
 */

import type { IntakeDraft } from '#lib/intakeDraft.svelte.js';
import { CHANGE_PARAMETER, CHANGE_VALUE, type IntakeSection, type IntakeStep } from '#lib/intakeJourney.js';

export interface QuestionJourney {
	/**
	Names the rail's landmark on every page of the journey.
	*/
	readonly label: string;
	/**
	What has been typed so far, and the steps walked.
	*/
	readonly draft: IntakeDraft;
	/**
	Every step of the journey, the Practice's own sections included.
	*/
	readonly steps: IntakeStep[];
	/**
	The Practice's own sections, one page each.
	*/
	readonly sections: IntakeSection[];
	/**
	Whether the steps are known yet.
	*/
	readonly isReady: boolean;
	/**
	The path each step's slug is under.
	*/
	readonly basePath: string;
	/**
	Where the first question's Back goes: the screen that opened the journey.
	*/
	readonly exitHref: string;
	/** What the Client is called on this page (#463's rule, with no
	 * pronoun in it). */
	readonly knownAs: string;
}

/**
 * The `name` the date-of-birth question's three boxes share, which is
 * also what each box's id is built from (`<name>-day`).
 */
export const DATE_OF_BIRTH_GROUP = 'intake-date-of-birth';

/** What `page.url.searchParams` hands out: a `URLSearchParams` with its
 * mutators removed (`svelte/prefer-svelte-reactivity` bans the mutable
 * one in reactive code). Read-only is all any of this needs. */
export type ReadonlyURLSearchParameters = Pick<URLSearchParams, 'get'>;

/** Whether this page was reached by a Change link from the check page,
 * which is what decides where both Back and Continue go. */
export function isFromCheck(search: ReadonlyURLSearchParameters): boolean {
	return search.get(CHANGE_PARAMETER) === CHANGE_VALUE;
}

/**
 * The check page, or wherever the reader would otherwise have gone.
 *
 * One function for both directions rather than a `continueHref` and a
 * `backHref` with identical bodies (a review of #466 caught the pair):
 * on a Change round trip the reader came from the check page and is
 * going back to it, so BOTH ends of the page point there, and the only
 * thing that differs is the `otherwise` each caller passes.
 */
export function checkOr(
	search: ReadonlyURLSearchParameters,
	basePath: string,
	otherwise: string
): string {
	return isFromCheck(search) ? `${basePath}/check` : otherwise;
}
