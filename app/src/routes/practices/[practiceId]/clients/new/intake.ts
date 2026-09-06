/**
 * What every step of intake needs and none of them owns (#466).
 *
 * The sequence is one question per route, so the things that are true of
 * the whole journey -- where it starts, what it is called, what the
 * Client is called on the page after the one that named her, and what
 * "save" means -- would otherwise be written out eight times.
 */

import { goto } from '$app/navigation';
import { resolve } from '$app/paths';
import { apiFetchWithSession } from '#lib/api.js';
import { createClient } from '#lib/client.js';
import { SERVICE_PROBLEM, type FormError } from '#lib/formErrors.js';
import { intakeDraft } from '#lib/intakeDraft.svelte.js';
import { CHANGE_PARAMETER, CHANGE_VALUE } from '#lib/intakeJourney.js';

/** What `page.url.searchParams` hands out: a `URLSearchParams` with its
 * mutators removed (`svelte/prefer-svelte-reactivity` bans the mutable
 * one in reactive code). Read-only is all any of this needs. */
type ReadonlyURLSearchParameters = Pick<URLSearchParams, 'get'>;

/**
Names the rail's landmark on every page of the sequence.
*/
export const JOURNEY = 'Adding a Client';

/**
The one control on the whole sequence a refusal can point at.
*/
export const GIVEN_NAME_ID = 'intake-given-name';

/**
 * ADR-0017's one requirement, asked wherever a save can start.
 *
 * A Client record needs a given name and nothing else, and
 * `CreateHandler` refuses without one -- so asking here is the
 * difference between a message beside the field and a message from the
 * server several pages later. It was written out in two routes with two
 * different wordings until a review of this ticket noticed.
 *
 * `askedFrom` decides whether the summary's entry is a link. On the name
 * page the field is right there; from the summary it is a page away, and
 * GOV.UK renders an entry with nowhere useful to send the reader as
 * plain text rather than as a link that goes nowhere.
 */
export function givenNameRefusal(
	askedFrom: 'this-page' | 'the-summary' = 'this-page'
): FormError[] {
	if (intakeDraft.hasGivenName) return [];
	return askedFrom === 'this-page'
		? [{ message: "Enter the Client's given name", targetId: GIVEN_NAME_ID }]
		: [{ message: "Enter the Client's given name on the Name step" }];
}

export function basePath(practiceId: string): string {
	return resolve('/practices/[practiceId]/clients/new', { practiceId });
}

export function searchHref(practiceId: string): string {
	return resolve('/practices/[practiceId]/clients/search', { practiceId });
}

export function detailHref(practiceId: string, clientId: string): string {
	return resolve('/practices/[practiceId]/clients/[clientId]', { practiceId, clientId });
}

/**
 * What the Client is called on this page.
 *
 * #463's rule with no pronoun in it: her preferred name if she has one,
 * her given name otherwise, and the domain noun before either exists --
 * which is only ever the first question, since that is the one that asks
 * for the name.
 */
export function knownAs(): string {
	const answers = intakeDraft.answers;
	return answers.preferredName.trim() || answers.givenName.trim() || 'the Client';
}

/** Whether this page was reached by a Change link from check-answers,
 * which is what decides where both Back and Continue go. */
export function isFromCheck(search: ReadonlyURLSearchParameters): boolean {
	return search.get(CHANGE_PARAMETER) === CHANGE_VALUE;
}

/**
 * The summary, or wherever the reader would otherwise have gone.
 *
 * One function for both directions rather than a `continueHref` and a
 * `backHref` with identical bodies (a review of this ticket caught the
 * pair): on a Change round trip the reader came from the summary and is
 * going back to it, so BOTH ends of the page point there, and the only
 * thing that differs is the `otherwise` each caller passes.
 */
export function checkOr(
	search: ReadonlyURLSearchParameters,
	practiceId: string,
	otherwise: string
): string {
	return isFromCheck(search) ? `${basePath(practiceId)}/check` : otherwise;
}

/**
 * Saves what has been typed, whenever it is asked for.
 *
 * ADR-0017 makes the save free: only a given name is required, and #466
 * removed #497's wait for all four match keys. So this is reachable from
 * every page's "Save and come back later" and from the summary's own
 * button, and does the same thing at each.
 *
 * A refused save with matches is not a failure -- it is the duplicate
 * check, which is a page of its own -- so it navigates there rather than
 * returning an error. Returns the errors to show when the save genuinely
 * could not happen, and undefined when the reader has been sent
 * somewhere.
 */
export async function saveIntake(
	practiceId: string,
	shouldOverride: boolean
): Promise<FormError[] | undefined> {
	try {
		const result = await createClient(
			apiFetchWithSession,
			practiceId,
			intakeDraft.answers,
			shouldOverride
		);
		if (result.conflict) {
			intakeDraft.matches = result.matches;
			await goto(`${basePath(practiceId)}/duplicate`);
			return undefined;
		}
		const clientId = result.record.id;
		intakeDraft.clear();
		await goto(detailHref(practiceId, clientId));
		return undefined;
	} catch (error) {
		return [{ message: error instanceof Error && error.message ? error.message : SERVICE_PROBLEM }];
	}
}
