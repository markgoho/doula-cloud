/**
 * A Practice deleting itself (#871, ADR-0031): the pending-deletion
 * status read every settings-hub precheck and the delete screen itself
 * both need, and the two acts -- starting the 30-day window and
 * restoring before it closes. Decoupled from SvelteKit and the DOM, the
 * same seam clientErasure.ts and mfaRequirement.ts use.
 */

import type { Fetcher } from './fetcher.js';

import { apiErrorMessage } from './apiErrorMessage.js';

/** What an Owner reads before deleting, and while the 30-day window is
 * open -- mirrors practicedeletion.StatusResponse. `pending` false and
 * `deletionRequestedAt` unset means nothing is running; `hasUnsettledInvoices`
 * names, ahead of time, exactly what InitiateHandler's own 409 would
 * otherwise be the only way to learn -- the same role
 * clientErasure.EraseEligibility plays ahead of #691's confirmation. */
export interface DeletionStatus {
	pending: boolean;
	deletionRequestedAt?: string;
	finalizeAt?: string;
	hasUnsettledInvoices: boolean;
}

/** What starting the window returns -- mirrors
 * practicedeletion.InitiateResponse. */
export interface DeletionOutcome {
	deletionRequestedAt: string;
	finalizeAt: string;
}

function deletionPath(practiceId: string): string {
	return `/api/practices/${practiceId}/deletion`;
}

/** Reads whether this Practice's deletion is pending, and whether an
 * unsettled Invoice would refuse starting one. Owner-only server-side,
 * the same gate every other read here mirrors -- a non-Owner throws with
 * the response body text, and the caller renders it through Notice
 * rather than showing the delete control at all. */
export async function loadDeletionStatus(fetcher: Fetcher, practiceId: string): Promise<DeletionStatus> {
	const response = await fetcher(deletionPath(practiceId));
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

/** Starts the 30-day window (practicedeletion.InitiateHandler). Sends
 * `X-Confirmed`, the same backstop staff.ts's removeMember and
 * endSessions send: this is one of the confirmed-destructive actions
 * this screen already gates behind a ConfirmDialog. Throws with the
 * response body text on a refusal -- already pending, already deleted,
 * or an unsettled Invoice. */
export async function initiateDeletion(fetcher: Fetcher, practiceId: string): Promise<DeletionOutcome> {
	const response = await fetcher(deletionPath(practiceId), {
		method: 'POST',
		headers: { 'X-Confirmed': 'true' }
	});
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

/** Restores a Practice pending deletion (practicedeletion.RestoreHandler).
 * No X-Confirmed: restoring undoes the destructive act rather than
 * committing to it, the same reasoning mfaRequirement's own
 * "stop requiring" direction sends nothing to confirm. Throws with the
 * response body text if nothing is pending to restore. */
export async function restorePractice(fetcher: Fetcher, practiceId: string): Promise<void> {
	const response = await fetcher(deletionPath(practiceId), { method: 'DELETE' });
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
}
