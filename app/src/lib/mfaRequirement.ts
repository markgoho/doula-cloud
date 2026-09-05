/**
 * A Practice's "require a second factor for every Staff member" switch
 * (#606). An Owner reads how many Staff currently hold no second
 * factor with `loadMfaRequirementImpact`, and raises or lowers the
 * switch with `setMfaRequired`. Both endpoints are Owner-only
 * server-side.
 *
 * The URL paths live here rather than inline in
 * `settings/mfa/+page.svelte` for the same reason payments.ts's and
 * website.ts's do: `formErrors.usage.spec.ts` greps every quoted
 * string in a `.svelte` file for GOV.UK's banned words, and this
 * endpoint's own name contains one of them ("required") as a plain
 * API identifier, not as user-facing copy. A `.ts` module is outside
 * that glob, which is where an identifier that happens to collide with
 * banned prose belongs anyway.
 */
import { refusalMessage } from './formErrors.js';

/** A minimal fetch-shaped function, injected rather than imported, so these
 * can be unit-tested without mocking `#lib/api.js` -- mirrors payments.ts's
 * and website.ts's own Fetcher. */
export type Fetcher = (path: string, init?: RequestInit) => Promise<Response>;

export interface MfaRequirementImpact {
	/**
	 * The switch's current value.
	 */
	required: boolean;
	/** How many Staff at this Practice -- all of them, not only those the
	 * switch would newly bar -- currently hold no enrolled second
	 * factor. */
	withoutSecondFactor: number;
}

function impactPath(practiceId: string): string {
	return `/api/practices/${practiceId}/mfa-required/impact`;
}

function switchPath(practiceId: string): string {
	return `/api/practices/${practiceId}/mfa-required`;
}

/** Loads the switch's current value and how many Staff it would bar if
 * turned on. Throws on a non-2xx response, read through `refusalMessage`
 * so a 5xx or an unreadable body reaches the caller as one sentence. */
export async function loadMfaRequirementImpact(
	fetcher: Fetcher,
	practiceId: string
): Promise<MfaRequirementImpact> {
	const response = await fetcher(impactPath(practiceId));
	if (!response.ok) {
		throw new Error(await refusalMessage(response));
	}
	return response.json();
}

/**
 * Raises or lowers the switch. The server refuses this without an
 * `X-Confirmed` header ("this action requires confirmation"), on the
 * assumption the caller already showed the affected count before
 * calling this -- `settings/mfa/+page.svelte`'s confirmation step owns
 * that for the direction that actually bars anyone. Throws on a
 * non-2xx response, same as `loadMfaRequirementImpact`.
 */
export async function setMfaRequired(fetcher: Fetcher, practiceId: string, isRequired: boolean): Promise<void> {
	const response = await fetcher(switchPath(practiceId), {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json', 'X-Confirmed': 'true' },
		body: JSON.stringify({ required: isRequired })
	});
	if (!response.ok) {
		throw new Error(await refusalMessage(response));
	}
}
