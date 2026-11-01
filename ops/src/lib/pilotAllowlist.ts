import type { FormError } from './authRefusal.ts';
import { postAct, refusal, type Outcome } from './act.ts';

export const ADDRESS_FIELD = 'pilot-address';

export type PilotAddressResult = { address: string; addedAt: string };

/**
GOV.UK's email-address rule, the one doula-ops applies: an @ with something either side.
*/
export function addressErrors(address: string): FormError[] {
	const trimmed = address.trim();
	if (trimmed === '') return [{ message: 'Enter an email address', targetId: ADDRESS_FIELD }];
	const at = trimmed.indexOf('@');
	if (at <= 0 || at === trimmed.length - 1) {
		return [{ message: 'Enter an email address in the correct format, like name@example.com', targetId: ADDRESS_FIELD }];
	}
	return [];
}

/**
 * Adds address to the pilot allowlist under key, the name of this one
 * request: a retry with the same key is answered with the add already made.
 * The body names no one; doula-ops takes the operator from the session.
 */
export async function sendPilotAddress(
	address: string,
	key: string,
	fetcher: typeof fetch = fetch
): Promise<Outcome<PilotAddressResult>> {
	const response = await postAct('/api/pilot-allowlist', key, { address }, fetcher);
	if (response.ok) return { kind: 'ok', value: await response.json() };
	return refusal(response, { address: ADDRESS_FIELD });
}
