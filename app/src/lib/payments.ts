/**
 * A Practice's Stripe Connect (Standard tier) linkage (#79): an Owner
 * starts hosted onboarding via `connect`, and any Staff member reads the
 * current not_connected / onboarding_incomplete / active status via
 * `loadConnectStatus`. Both are read live from Stripe -- see
 * api/internal/payments/connect.go's doc comments.
 */

export type ConnectStatus = 'not_connected' | 'onboarding_incomplete' | 'active';

export interface ConnectStatusResult {
	status: ConnectStatus;
	chargesEnabled: boolean;
	payoutsEnabled: boolean;
	detailsSubmitted: boolean;
}

/** A minimal fetch-shaped function, injected rather than imported, so load
 * can be unit-tested without mocking the global fetch or SvelteKit's `$app`
 * modules -- mirrors billing.ts's Fetcher. */
export type Fetcher = (path: string, init?: RequestInit) => Promise<Response>;

function connectPath(practiceId: string): string {
	return `/api/practices/${practiceId}/payments/connect`;
}

/** Loads a Practice's current Stripe Connect status. Throws with the
 * response body text on a non-2xx response, mirroring loadBalance's
 * error-surfacing convention. */
export async function loadConnectStatus(fetcher: Fetcher, practiceId: string): Promise<ConnectStatusResult> {
	const response = await fetcher(connectPath(practiceId));
	if (!response.ok) {
		throw new Error(await response.text());
	}
	return response.json();
}

/** Starts (or resumes) Stripe Connect onboarding and returns the
 * Stripe-hosted Account Link URL the caller's browser must navigate to.
 * Throws with the response body text on a non-2xx response -- e.g. a
 * non-Owner attempting the request, which the backend's `RequireOwner`
 * rejects. */
export async function connect(fetcher: Fetcher, practiceId: string): Promise<string> {
	const response = await fetcher(connectPath(practiceId), { method: 'POST' });
	if (!response.ok) {
		throw new Error(await response.text());
	}
	const body: { onboardingUrl: string } = await response.json();
	return body.onboardingUrl;
}
