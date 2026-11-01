import { postAct, refusal, type Outcome } from './act.ts';

/**
The Staff member whose two-factor authentication is reset, and each Practice whose Activity records it.
*/
export type SecondFactorStaff = { staffName: string; practiceNames: string[] };

type Fetch = typeof fetch;

/**
What the check page states: the Staff member and each Practice.
*/
export async function readSecondFactorCheck(staffId: string, fetcher: Fetch = fetch): Promise<Outcome<SecondFactorStaff>> {
	const response = await fetcher(`/api/staff/${staffId}/second-factor`, { credentials: 'include' });
	if (response.ok) return { kind: 'ok', value: await response.json() };
	return refusal(response);
}

/**
 * Resets the Staff member's two-factor authentication under key, the
 * name of this one request: a retry with the same key is answered with
 * the reset already made. The body names nothing; doula-ops takes the
 * operator from the session.
 */
export async function sendSecondFactorClear(staffId: string, key: string, fetcher: Fetch = fetch): Promise<Outcome<SecondFactorStaff>> {
	const response = await postAct(`/api/staff/${staffId}/second-factor-clears`, key, {}, fetcher);
	if (response.ok) return { kind: 'ok', value: await response.json() };
	return refusal(response);
}

/**
The Practices as a person reads them, or a plain none.
*/
export function practiceList(names: string[]): string {
	if (names.length === 0) return 'None';
	return names.join(', ');
}
