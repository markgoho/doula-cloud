/**
 * A Staff person deleting her own login (#892, ADR-0033): one act, no
 * precheck to read first. Decoupled from SvelteKit and the DOM, the same
 * seam practiceDeletion.ts and clientErasure.ts use.
 *
 * There is deliberately no eligibility read to pair with this the way
 * `loadDeletionStatus` pairs with `initiateDeletion`. The only refusal is
 * the last-Owner one, and it names the Practices in the way in its own
 * message -- a precheck would be a second place for the same list to be
 * computed, and would go stale between the read and the act. She reads
 * the refusal when it happens, with the Practices named.
 */

import type { Fetcher } from './fetcher.js';

import { apiErrorMessage } from './apiErrorMessage.js';

/**
 * Deletes the caller's own login (staffauth.DeleteLoginHandler). Sends
 * `X-Confirmed`, the same backstop staff.ts's removeMember and endSessions
 * and practiceDeletion.ts's initiateDeletion send: this is a
 * confirmed-destructive action gated behind a ConfirmDialog.
 *
 * No id anywhere in the path or the body. The endpoint only ever acts on
 * the row the session cookie's identity resolves to, which is how
 * self-only is enforced where it can actually be enforced -- there is no
 * parameter here for a caller to redirect at anybody else.
 *
 * Throws with the response body text on the last-Owner refusal, so the
 * caller renders the Practices in the way verbatim rather than
 * reconstructing them.
 */
export async function deleteOwnLogin(fetcher: Fetcher): Promise<void> {
	const response = await fetcher('/api/staff/account', {
		method: 'DELETE',
		headers: { 'X-Confirmed': 'true' }
	});
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
}
