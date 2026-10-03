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
import { page } from '#lib/appState.svelte.js';
import { createClient } from '#lib/client.js';
import { errorsFromCause, type FormError } from '#lib/formErrors.js';
import { dateFieldId } from '#lib/intakeDate.js';
import { intakeDraft } from '#lib/intakeDraft.svelte.js';
import { intakeFlow } from '#lib/intakeFlow.svelte.js';
import { knownAsFrom, originQuery, type IntakeOrigin } from '#lib/intakeJourney.js';
import { DATE_OF_BIRTH_GROUP, type QuestionJourney } from '../questions/questionJourney.js';

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

/**
 * The BFF's own field names (`client.Record`'s json tags) mapped onto
 * the controls of the step being saved from (#488).
 *
 * Per-step, because intake is a sequence of pages and only one of them
 * is showing at a time: a `givenName` refusal saved from the name step
 * has a control right there, and the same refusal saved from the summary
 * is a page away. An entry with nowhere useful to send the reader is
 * rendered as plain text rather than as a link that goes nowhere, which
 * is what an empty map here produces -- the same rule `givenNameRefusal`
 * follows.
 *
 * The date group's first box is the target, which is GOV.UK's rule for a
 * group: the group itself is a `<fieldset>` and is not focusable. The
 * two ids are written here rather than in each page so the summary and
 * the control cannot drift apart.
 */
export function intakeFieldIds(stepId: string): Record<string, string> {
	switch (stepId) {
		case 'name': {
			return { givenName: GIVEN_NAME_ID };
		}
		case 'date-of-birth': {
			return { dateOfBirth: dateFieldId(DATE_OF_BIRTH_GROUP, 'day') };
		}
		default: {
			return {};
		}
	}
}

export function basePath(practiceId: string): string {
	return resolve('/practices/[practiceId]/clients/new', { practiceId });
}

/**
 * The link that opens the name question directly, with no search in front
 * of it (#1609), naming the screen it is on so Back returns there. Only
 * the overview and the Clients list of a Practice that holds no Client
 * record show one.
 */
export function startHref(practiceId: string, origin: IntakeOrigin): string {
	return `${basePath(practiceId)}?${originQuery(origin)}`;
}

/**
 * Where the name question's Back goes: the screen that opened it (#1609).
 * The overview and the Clients list open it directly while the Practice
 * holds no Client record; from the first Client on, the search does, and
 * the search is also where Back goes when the origin is not known.
 */
export function exitHref(practiceId: string, origin: IntakeOrigin | undefined): string {
	switch (origin) {
		case 'overview': {
			return resolve('/practices/[practiceId]', { practiceId });
		}
		case 'clients': {
			return resolve('/practices/[practiceId]/clients', { practiceId });
		}
		default: {
			return resolve('/practices/[practiceId]/clients/search', { practiceId });
		}
	}
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
	return knownAsFrom(intakeDraft.answers);
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
	shouldOverride: boolean,
	fieldIds: Record<string, string> = {}
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
		// A server refusal that names fields becomes one entry each
		// (#488), each pointed at the control the caller says holds it.
		// Callers away from the field pass nothing, and those entries
		// stay untargeted for the reason `givenNameRefusal` gives: GOV.UK
		// renders an entry with nowhere useful to send the reader as
		// plain text rather than as a link that goes nowhere.
		return errorsFromCause(error, fieldIds);
	}
}

function currentPracticeId(): string {
	return page.params.practiceId ?? '';
}

/**
 * Intake, as the shared question pages see it (#1610): its own draft,
 * the Practice's template, Back to whichever screen opened it, and the
 * free save on every page.
 */
export const intakeQuestions: QuestionJourney = {
	label: JOURNEY,
	draft: intakeDraft,
	get steps() {
		return intakeFlow.steps;
	},
	get sections() {
		return intakeFlow.sections;
	},
	get isReady() {
		return intakeFlow.status === 'ready';
	},
	get basePath() {
		return basePath(currentPracticeId());
	},
	get exitHref() {
		return exitHref(currentPracticeId(), intakeDraft.origin);
	},
	get knownAs() {
		return knownAs();
	},
	saveForLater: (stepId) => saveIntake(currentPracticeId(), false, intakeFieldIds(stepId))
};
