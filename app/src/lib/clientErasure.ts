/**
 * Erasing a Client on her own request (#691, ADR-0027, #394's endpoint):
 * the eligibility read the detail screen checks before ever offering the
 * confirmation, and the erasure act itself. Decoupled from SvelteKit and
 * the DOM, the same seam clientDetail.ts and engagementRequest.ts use.
 */

import { apiErrorMessage } from './apiErrorMessage.js';

/** One of a Client's still-draft-or-open invoices, blocking her erasure
 * -- mirrors client.UnsettledInvoiceSummary. Enough for the confirmation
 * screen to name what has to be settled or voided first, without
 * exposing the wider Invoice history ADR-0008 reserves for Owner and
 * Admin -- this precheck is Owner-only in its own right (mirroring
 * EraseHandler's own gate), not a general Invoice read. */
export interface UnsettledInvoice {
	invoiceId: string;
	status: string;
	amountCents: number;
	currency: string;
	createdAt: string;
}

/** What an Owner reads before she can reach #691's confirmation step --
 * mirrors client.EraseEligibility. `erasedAt` set means the act already
 * ran; a non-empty `unsettledInvoices` means it cannot run yet, and
 * names exactly what EraseHandler's own 409 would otherwise be the only
 * way to learn. */
export interface EraseEligibility {
	erasedAt?: string;
	unsettledInvoices: UnsettledInvoice[];
}

/** What a successful erasure returns -- mirrors client.ErasureResponse.
 * The confirmation screen merges this straight into the Client's own
 * detail state rather than re-fetching, the same "update from the
 * response" pattern the Client detail screen's handleWithdraw already
 * uses for a different mutation. */
export interface ErasureOutcome {
	erasedAt: string;
	stripeRedactionEligibleAt?: string;
	stripeCustomersQueued: number;
	portalAccountQueued: boolean;
}

/** A minimal fetch-shaped function, injected rather than imported --
 * mirrors client.ts's Fetcher. */
export type Fetcher = (path: string, init?: RequestInit) => Promise<Response>;

function erasurePath(practiceId: string, clientId: string): string {
	return `/api/practices/${practiceId}/clients/${clientId}/erasure`;
}

/** Reads whether this Client can be erased right now
 * (client.EraseEligibilityHandler). Owner-only, mirroring EraseHandler's
 * own gate -- a non-Owner's 403 throws with the response body text, the
 * same convention every other write/read in this module family follows,
 * so the caller renders it through Notice rather than showing the erase
 * control at all. */
export async function loadEraseEligibility(
	fetcher: Fetcher,
	practiceId: string,
	clientId: string
): Promise<EraseEligibility> {
	const response = await fetcher(erasurePath(practiceId, clientId));
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

/** Runs the erasure itself (client.EraseHandler). Throws with the
 * response body text on a refusal -- a race that erased her a moment
 * ago, or a race that opened a new invoice since the eligibility read --
 * mirroring loadClientDetail's error-surfacing convention, so the
 * caller's existing catch-and-render-a-Notice pattern already covers it. */
export async function eraseClient(
	fetcher: Fetcher,
	practiceId: string,
	clientId: string
): Promise<ErasureOutcome> {
	const response = await fetcher(erasurePath(practiceId, clientId), { method: 'POST' });
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}
