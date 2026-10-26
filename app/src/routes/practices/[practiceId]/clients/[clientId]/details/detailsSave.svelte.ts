/**
 * The one save of a Client's details journey (#1610, #2048), shared by
 * the check page and by a question a Change on her record opened.
 *
 * One edit, through the edit path's full-record `PUT` (`edit.go`), so
 * ADR-0017's amendment's two gates run on it as on every edit, and the
 * one `updated` entry on her history says who saved it and when.
 *
 * - Gate one, a name substitution (`substitution: true`): her new name is
 *   exactly another Client's. Answered by the app's one confirmation
 *   mechanism (#473), the dialog `details/+layout.svelte` draws from this
 *   state: "Yes, a different person" retries with `override: true`.
 * - Gate two, a possible duplicate (`substitution: false`): the journey's
 *   own `duplicate` page asks "Is this the same person?", the only door
 *   to combining two records, with Back to the page the save was made on.
 *
 * Module state, because the page that saves and the layout that draws
 * the dialog are two components -- the same reason `editMergeDraft`
 * gives for the duplicate page.
 */

import { goto } from '$app/navigation';
import { page } from '#lib/appState.svelte.js';
import { apiFetchWithSession } from '#lib/api.js';
import { editClient, type ClientEditFields, type CollisionMatch } from '#lib/client.js';
import { clientDetails } from '#lib/clientDetailsFlow.svelte.js';
import { detailsToSave } from '#lib/clientDetailsJourney.js';
import { editMergeDraft } from '#lib/editMergeDraft.svelte.js';
import { errorsFromCause } from '#lib/formErrors.js';
import { detailsSavedMessage, type IntakeSection } from '#lib/intakeJourney.js';
import { gotoWithOutcome } from '#lib/outcome.js';

/**
 * What the save needs to know about the journey: where it lives, where
 * it lands, and the Practice sections it asks.
 */
export interface SavedJourney {
	readonly basePath: string;
	readonly exitHref: string;
	readonly sections: IntakeSection[];
}

/**
What the save sends: her record, with every answer laid over it.
*/
function fieldsToSave(journey: SavedJourney): ClientEditFields {
	const askedFieldIds = journey.sections.flatMap((section) => section.fields.map((field) => field.id));
	return detailsToSave(clientDetails.record!, clientDetails.draft.answers, askedFieldIds);
}

function ids(): { practiceId: string; clientId: string } {
	return { practiceId: page.params.practiceId ?? '', clientId: page.params.clientId ?? '' };
}

export class DetailsSave {
	/**
	The journey the dialog's override saves, set by `save`.
	*/
	#journey: SavedJourney | undefined;
	/**
	The Clients gate one named, while its dialog is open.
	*/
	matches = $state<CollisionMatch[]>([]);
	isConflictOpen = $state(false);
	/**
	A refused override, shown inside the dialog (#804's contract).
	*/
	overrideError = $state('');

	/** The screen the journey was opened from says the save happened
	 * (#1710): the page that saved is gone once the save leaves it. */
	async #finish(journey: SavedJourney, fields: ClientEditFields): Promise<void> {
		const href = journey.exitHref;
		clientDetails.draft.clear();
		await gotoWithOutcome(href, detailsSavedMessage(fields));
	}

	/**
	 * Saves, or stops at whichever gate refused. `returnHref` is the page
	 * the save was made on, where the duplicate page's Back goes. A
	 * refusal that is not a gate is thrown, for the caller's
	 * `FormSubmission`.
	 */
	async save(journey: SavedJourney, returnHref: string): Promise<void> {
		this.#journey = journey;
		const { practiceId, clientId } = ids();
		const fields = fieldsToSave(journey);
		const result = await editClient(apiFetchWithSession, practiceId, clientId, fields, false);
		if (!result.conflict) {
			await this.#finish(journey, fields);
			return;
		}
		if (result.substitution) {
			this.matches = result.matches;
			this.isConflictOpen = true;
			return;
		}
		editMergeDraft.open(clientId, fields, result.matches, returnHref);
		await goto(`${journey.basePath}/duplicate`);
	}

	/** The single deliberate override, reached only by the dialog's named
	 * button. A rejection keeps the dialog open over its own message. */
	async confirmOverride(): Promise<void> {
		const journey = this.#journey!;
		const fields = fieldsToSave(journey);
		this.overrideError = '';
		try {
			const { practiceId, clientId } = ids();
			const result = await editClient(apiFetchWithSession, practiceId, clientId, fields, true);
			// override: true skips the match query entirely (edit.go), so a
			// conflict here means something else refused the write.
			if (result.conflict) throw new Error('The Client record could not be saved.');
		} catch (error) {
			this.overrideError = errorsFromCause(error, {})
				.map((entry) => entry.message)
				.join(' ');
			throw new Error(this.overrideError, { cause: error });
		}
		this.matches = [];
		await this.#finish(journey, fields);
	}

	cancel(): void {
		this.matches = [];
		this.overrideError = '';
	}
}

/**
The one save the `details` layout and its pages share.
*/
export const detailsSave = new DetailsSave();
