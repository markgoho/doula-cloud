import { postAct, refusal, type Outcome } from './act.ts';

/**
The pilot gate: when it was opened, or null while it is closed.
*/
export type PilotGate = { openedAt: string | null };

type Fetch = typeof fetch;

/**
Whether the gate is open, which the check page reads before it asks for a code.
*/
export async function readPilotGate(fetcher: Fetch = fetch): Promise<Outcome<PilotGate>> {
	const response = await fetcher('/api/pilot-gate', { credentials: 'include' });
	if (response.ok) return { kind: 'ok', value: await response.json() };
	return refusal(response);
}

/**
 * Opens the pilot gate under key, the name of this one request: a retry
 * with the same key is answered with the opening already made. The body
 * names no one; doula-ops takes the operator from the session.
 */
export async function sendPilotGateOpening(key: string, fetcher: Fetch = fetch): Promise<Outcome<PilotGate>> {
	const response = await postAct('/api/pilot-gate/openings', key, {}, fetcher);
	if (response.ok) return { kind: 'ok', value: await response.json() };
	return refusal(response);
}
