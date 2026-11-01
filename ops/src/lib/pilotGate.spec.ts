/* eslint-disable unicorn/no-null -- doula-ops answers null for a gate still closed */
import { describe, expect, it, vi } from 'vitest';
import { readPilotGate, sendPilotGateOpening } from './pilotGate.ts';

function answering(status: number, body?: unknown) {
	return vi.fn<typeof fetch>(
		async () => new Response(body === undefined ? undefined : JSON.stringify(body), { status })
	);
}

describe('readPilotGate', () => {
	it('reads whether the gate is open', async () => {
		const fetcher = answering(200, { openedAt: null });

		expect(await readPilotGate(fetcher)).toEqual({ kind: 'ok', value: { openedAt: null } });
		expect(fetcher).toHaveBeenCalledWith('/api/pilot-gate', { credentials: 'include' });
	});

	it('sends a signed-out read to sign in', async () => {
		expect(await readPilotGate(answering(401, { code: 'UNAUTHORIZED', message: 'Sign in' }))).toEqual({
			kind: 'signed-out'
		});
	});
});

describe('sendPilotGateOpening', () => {
	it('sends no fields under its key and hands back what changed', async () => {
		const gate = { openedAt: '2027-01-04T14:00:00Z' };
		const fetcher = answering(200, gate);

		expect(await sendPilotGateOpening('key-1', fetcher)).toEqual({ kind: 'ok', value: gate });
		expect(fetcher).toHaveBeenCalledWith('/api/pilot-gate/openings', {
			method: 'POST',
			credentials: 'include',
			headers: { 'Content-Type': 'application/json', 'Idempotency-Key': 'key-1' },
			body: '{}'
		});
	});

	it('asks for a fresh code again', async () => {
		expect(
			await sendPilotGateOpening('key-1', answering(403, { code: 'SECOND_FACTOR_REQUIRED', message: 'code' }))
		).toEqual({ kind: 'step-up' });
	});

	it('says the gate is already open', async () => {
		const message = 'The pilot gate is already open. It was opened on January 4, 2027, and nothing has changed.';

		expect(await sendPilotGateOpening('key-1', answering(409, { code: 'CONFLICT', message }))).toEqual({
			kind: 'refused',
			errors: [{ message }]
		});
	});
});
