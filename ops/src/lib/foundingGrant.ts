import { postAct, refusal, type Outcome } from './act.ts';

export type FoundingGrantCheck = {
	practiceName: string;
	staffCount: number;
	credits: number;
	grantedCredits: number | null;
};
export type FoundingGrantResult = { practiceName: string; staffCount: number; credits: number };

type Fetch = typeof fetch;

export function credits(count: number): string {
	return count === 1 ? '1 Credit' : `${count} Credits`;
}

export async function readFoundingGrantCheck(practiceId: string, fetcher: Fetch = fetch): Promise<Outcome<FoundingGrantCheck>> {
	const response = await fetcher(`/api/practices/${practiceId}/founding-grant`, { credentials: 'include' });
	if (response.ok) return { kind: 'ok', value: await response.json() };
	return refusal(response);
}

export async function sendFoundingGrant(practiceId: string, key: string, fetcher: Fetch = fetch): Promise<Outcome<FoundingGrantResult>> {
	const response = await postAct(`/api/practices/${practiceId}/founding-grants`, key, {}, fetcher);
	if (response.ok) return { kind: 'ok', value: await response.json() };
	return refusal(response);
}
