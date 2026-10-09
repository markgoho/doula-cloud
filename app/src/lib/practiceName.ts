/**
 * A Practice's own name (#1540): the save half of the settings screen,
 * decoupled from SvelteKit and the DOM so it can be unit-tested directly --
 * mirrors practiceTimezone.ts.
 *
 * There is no read half. The name rides on the Practice session every
 * screen under `practices/[practiceId]` already resolves, so the screen
 * opens on it and a second fetch would only be a second answer that could
 * disagree with the first.
 *
 * Nothing here checks that the name is not empty. The BFF refuses it with
 * the sentence the Owner reads, keyed to the `name` field.
 */

import type { Fetcher } from './fetcher.js';

import { refusalError } from './formErrors.js';

export interface PracticeName {
	name: string;
}

/** States the Practice's name. Only an Owner's session reaches this
 * without a 403 -- practicename.PutHandler is the authority, and this
 * module makes no role check of its own. */
export async function savePracticeName(
	fetcher: Fetcher,
	practiceId: string,
	name: string
): Promise<PracticeName> {
	const response = await fetcher(`/api/practices/${practiceId}/name`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ name })
	});
	if (!response.ok) {
		// A RefusalError, not a plain Error: the BFF keys its refusal to
		// the `name` field, and a plain Error would drop the sentence
		// written for the person in favor of the one written for the caller.
		throw await refusalError(response);
	}
	return response.json();
}
