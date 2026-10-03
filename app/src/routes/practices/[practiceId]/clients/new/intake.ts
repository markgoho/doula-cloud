/**
 * What intake's two pages share and neither owns (#466, #1611).
 *
 * Intake for a new Client is one question, her name, and a save that
 * opens the Start work form (#1611, ADR-0017's amendment of 2026-10-02).
 * The duplicate page is the only other page, reached when the save finds
 * a match. The other details are added later from her record (#1610).
 */

import { goto } from '$app/navigation';
import { resolve } from '$app/paths';
import { apiFetchWithSession } from '#lib/api.js';
import { createClient } from '#lib/client.js';
import type { JourneyStep } from '#lib/components/organisms/StepRail.svelte';
import { errorsFromCause, type FormError } from '#lib/formErrors.js';
import { intakeDraft } from '#lib/intakeDraft.svelte.js';
import { clientSavedMessage, knownAsFrom, originQuery, type IntakeOrigin } from '#lib/intakeJourney.js';
import { gotoWithOutcome } from '#lib/outcome.js';

/**
Names the journey a question page is in.
*/
export const JOURNEY = 'Adding a Client';

/**
The one control in intake a refusal can point at.
*/
export const GIVEN_NAME_ID = 'intake-given-name';

/**
 * The BFF's own field names (`client.Record`'s json tags) mapped onto
 * the name question's controls (#488). The given name is the only
 * column `CreateHandler` refuses on that has a control in intake.
 */
export const INTAKE_FIELD_IDS: Record<string, string> = { givenName: GIVEN_NAME_ID };

/**
 * ADR-0017's one requirement, asked before the save.
 *
 * A Client record needs a given name and nothing else, and
 * `CreateHandler` refuses without one -- so asking here is the
 * difference between a message beside the field and a message from the
 * server.
 */
export function givenNameRefusal(): FormError[] {
	if (intakeDraft.hasGivenName) return [];
	return [{ message: "Enter the Client's given name", targetId: GIVEN_NAME_ID }];
}

export function basePath(practiceId: string): string {
	return resolve('/practices/[practiceId]/clients/new', { practiceId });
}

/**
 * Intake as the journey `QuestionPage` is given: one step. The Template
 * draws no rail for it (#1611), so no page says "Step 1 of 1".
 */
export function intakeSteps(practiceId: string): JourneyStep[] {
	return [{ label: 'Name', href: `${basePath(practiceId)}/name`, status: 'current' }];
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
 * Where a saved new Client goes next: the Start work form (#1611). The
 * save starts nothing (ADR-0017), so the form is the next act of the
 * same flow, and its second action goes to her record instead.
 */
export function startWorkHref(practiceId: string, clientId: string): string {
	return resolve('/practices/[practiceId]/clients/[clientId]/engagement-requests/new', {
		practiceId,
		clientId
	});
}

/**
 * What the Client is called on this page.
 *
 * #463's rule with no pronoun in it: her preferred name if she has one,
 * her given name otherwise, and the domain noun before either exists.
 */
export function knownAs(): string {
	return knownAsFrom(intakeDraft.answers);
}

/**
 * Saves the new Client and opens the Start work form (#1611), which says
 * the Client is saved (#1710).
 *
 * ADR-0017 makes the save free: only a given name is required. What the
 * search carried (date of birth, email, phone) is in the draft and is
 * saved with the name, so the collision check has those keys too.
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
		const message = clientSavedMessage(intakeDraft.answers);
		intakeDraft.clear();
		await gotoWithOutcome(startWorkHref(practiceId, clientId), message);
		return undefined;
	} catch (error) {
		// A server refusal that names fields becomes one entry each
		// (#488), each pointed at the control the caller says holds it.
		// The duplicate page has no name field and passes nothing, so
		// there the entries stay plain text: GOV.UK renders an entry with
		// nowhere useful to send the reader as text rather than as a link
		// that goes nowhere.
		return errorsFromCause(error, fieldIds);
	}
}
