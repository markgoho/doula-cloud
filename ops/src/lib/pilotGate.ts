import { postAct, refusal, type Outcome } from './act.ts';

export type PilotGate = { openedAt: string | null };

type Fetch = typeof fetch;

export async function readPilotGate(fetcher: Fetch = fetch): Promise<Outcome<PilotGate>> {
	const response = await fetcher('/api/pilot-gate', { credentials: 'include' });
	if (response.ok) return { kind: 'ok', value: await response.json() };
	return refusal(response);
}

export async function sendPilotGateOpening(key: string, fetcher: Fetch = fetch): Promise<Outcome<PilotGate>> {
	const response = await postAct('/api/pilot-gate/openings', key, {}, fetcher);
	if (response.ok) return { kind: 'ok', value: await response.json() };
	return refusal(response);
}
