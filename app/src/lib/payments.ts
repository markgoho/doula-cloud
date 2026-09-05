/**
 * A Practice's Stripe Connect linkage (#79): an Owner starts hosted
 * onboarding via `connect`, and any Staff member reads the current status
 * via `loadConnectStatus`. Both are read live from Stripe -- see
 * api/internal/payments/connect.go's doc comments.
 */

import { apiErrorMessage } from './api.js';

export type ConnectStatus =
	| 'not_connected'
	| 'onboarding_incomplete'
	| 'pending'
	| 'payouts_restricted'
	| 'active';

/** The status Stripe reports for one capability on a v2 Account's merchant
 * configuration. Accounts v1 reported booleans; v2 reports four values, and
 * `pending` is the one a boolean could not express -- Stripe is reviewing,
 * and there is nothing left for the Owner to do (#247). */
export type CapabilityStatus = 'active' | 'pending' | 'restricted' | 'unsupported';

export interface ConnectStatusResult {
	status: ConnectStatus;
	/**
	Whether the Practice can be paid by card at all.
	*/
	cardPaymentsStatus: CapabilityStatus;
	/** Whether that money can reach the Practice's bank. Moves
	 * independently of cardPaymentsStatus. */
	payoutsStatus: CapabilityStatus;
	/** Stripe field paths still awaiting the Owner. Always present -- the
	 * backend sends an empty list rather than omitting it. */
	requirementsDue: string[];
}

/** A minimal fetch-shaped function, injected rather than imported, so load
 * can be unit-tested without mocking the global fetch or SvelteKit's `$app`
 * modules -- mirrors billing.ts's Fetcher. */
export type Fetcher = (path: string, init?: RequestInit) => Promise<Response>;

function connectPath(practiceId: string): string {
	return `/api/practices/${practiceId}/payments/connect`;
}

/** Loads a Practice's current Stripe Connect status. Throws with the
 * response body's message on a non-2xx response, mirroring loadBalance's
 * error-surfacing convention. */
export async function loadConnectStatus(fetcher: Fetcher, practiceId: string): Promise<ConnectStatusResult> {
	const response = await fetcher(connectPath(practiceId));
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

/** Starts (or resumes) Stripe Connect onboarding and returns the
 * Stripe-hosted Account Link URL the caller's browser must navigate to.
 *
 * Throws with whatever sentence the server sent on a non-2xx response --
 * a non-Owner attempting the request, which `RequireOwner` rejects, or a
 * Practice who has not declared a website, which #442 refuses with
 * docs/api-design.md section 7's structured body. Read through
 * `apiErrorMessage` rather than as raw text, so the second of those
 * reaches the screen as a sentence and not as JSON. */
export async function connect(fetcher: Fetcher, practiceId: string): Promise<string> {
	const response = await fetcher(connectPath(practiceId), { method: 'POST' });
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	const body: { onboardingUrl: string } = await response.json();
	return body.onboardingUrl;
}
