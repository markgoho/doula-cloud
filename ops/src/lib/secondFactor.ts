import { postAct, refusal, stepUpRefusal, type Outcome, type StepUpOutcome } from './act.ts';

export type SecondFactorStaff = { staffName: string; practiceNames: string[] };

type Fetch = typeof fetch;

export async function readSecondFactorCheck(staffId: string, fetcher: Fetch = fetch): Promise<Outcome<SecondFactorStaff>> {
	const response = await fetcher(`/api/staff/${staffId}/second-factor`, { credentials: 'include' });
	if (response.ok) return { kind: 'ok', value: await response.json() };
	return refusal(response);
}

export async function sendSecondFactorClear(staffId: string, key: string, fetcher: Fetch = fetch): Promise<StepUpOutcome<SecondFactorStaff>> {
	const response = await postAct(`/api/staff/${staffId}/second-factor-clears`, key, {}, fetcher);
	if (response.ok) return { kind: 'ok', value: await response.json() };
	return stepUpRefusal(response);
}

export function practiceList(names: string[]): string {
	if (names.length === 0) return 'None';
	return names.join(', ');
}
