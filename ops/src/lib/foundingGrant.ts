import { refusal, type Outcome } from './act.ts';

export type FoundingGrantCheck = {
	practiceName: string;
	staffCount: number;
	credits: number;
	grantedCredits: number | null;
};
export type FoundingGrantResult = { practiceName: string; staffCount: number; credits: number };

type Fetch = typeof fetch;

/**
A count of Credits as a person reads it: 1 Credit, 12 Credits.
*/
export function credits(count: number): string {
	return count === 1 ? '1 Credit' : `${count} Credits`;
}

/**
What the check page states: the Practice, its Staff, and the grant they size.
*/
export async function readFoundingGrantCheck(practiceId: string, fetcher: Fetch = fetch): Promise<Outcome<FoundingGrantCheck>> {
	const response = await fetcher(`/api/practices/${practiceId}/founding-grant`, { credentials: 'include' });
	if (response.ok) return { kind: 'ok', value: await response.json() };
	return refusal(response);
}

/**
 * Gives the grant under key, the name of this one request: a retry with
 * the same key is answered with the grant already given. The body names
 * nothing; doula-ops sizes the grant and takes the operator from the session.
 */
export async function sendFoundingGrant(practiceId: string, key: string, fetcher: Fetch = fetch): Promise<Outcome<FoundingGrantResult>> {
	const response = await fetcher(`/api/practices/${practiceId}/founding-grants`, {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json', 'Idempotency-Key': key },
		body: '{}'
	});
	if (response.ok) return { kind: 'ok', value: await response.json() };
	return refusal(response);
}
