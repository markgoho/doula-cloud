/**
 * What gate two (ADR-0017's amendment, #814) needs to carry from the
 * Client edit page to its own `duplicate` sub-route and back -- the same
 * "module state a whole sequence shares" idiom `intakeDraft.svelte.ts`
 * uses, scoped to one edit act rather than to a multi-step sequence.
 *
 * No `sessionStorage` mirror: intake's draft needs one because a doula
 * can spend several page loads walking eight questions before she saves.
 * This draft lives for one edit's conflict-to-answer round trip, and a
 * reader who reloads mid-way has nothing worth recovering -- the guard
 * below sends her back to the edit page instead, the same way intake's
 * own duplicate page sends a matchless reload back to its summary.
 */

import type { ClientEditFields, CollisionMatch } from './client.js';

/** Every structural column but `id`, blank -- what a fresh draft holds
 * before the edit page's 409 fills it in. */
export function blankEditFields(): ClientEditFields {
	return {
		givenName: '',
		familyName: '',
		preferredName: '',
		email: '',
		phone: '',
		addressLine1: '',
		addressLine2: '',
		addressLocality: '',
		addressRegion: '',
		addressPostalCode: '',
		dateOfBirth: '',
		fieldValues: {}
	};
}

export class EditMergeDraft {
	/**
	 * The Client whose edit was refused by gate two.
	 */
	clientId = $state('');
	/** What was typed for `clientId` and not yet saved -- what "This is
	 * her" or the override would write, had gate two not fired. */
	fields = $state<ClientEditFields>(blankEditFields());
	/**
	 * The possible duplicates gate two named.
	 */
	matches = $state<CollisionMatch[]>([]);
	/**
	 * Opens the draft with one edit's refused attempt.
	 *
	 * There was a `mergeOffered` flag here until #813 (ADR-0039), saying
	 * whether "This is her" was available at all. It always is now: two
	 * records that both carry history merge, and attachment only decides
	 * which of them survives -- which each match's own `wouldSurvive`
	 * already says.
	 */
	open(clientId: string, fields: ClientEditFields, matches: CollisionMatch[]): void {
		this.clientId = clientId;
		this.fields = fields;
		this.matches = matches;
	}

	/** Everything the draft holds, gone -- what a save that went through,
	 * in either direction, leaves behind. */
	clear(): void {
		this.clientId = '';
		this.fields = blankEditFields();
		this.matches = [];
	}
}

/**
The one draft the edit page and its `duplicate` sub-route share.
*/
export const editMergeDraft = new EditMergeDraft();
