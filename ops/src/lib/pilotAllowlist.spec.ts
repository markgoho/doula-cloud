import { describe, expect, it, vi } from 'vitest';
import { ADDRESS_FIELD, addressErrors, sendPilotAddress } from './pilotAllowlist.ts';

function answering(status: number, body?: unknown) {
	return vi.fn<typeof fetch>(
		async () => new Response(body === undefined ? undefined : JSON.stringify(body), { status })
	);
}

describe('addressErrors', () => {
	it('asks for an address when none is entered', () => {
		expect(addressErrors('  ')).toEqual([{ message: 'Enter an email address', field: ADDRESS_FIELD }]);
	});

	it('asks for an address in the right format', () => {
		for (const address of ['ana.example.com', 'ana@', '@example.com']) {
			expect(addressErrors(address)).toEqual([
				{ message: 'Enter an email address in the correct format, like name@example.com', field: ADDRESS_FIELD }
			]);
		}
	});

	it('takes an address with an @ and something either side', () => {
		expect(addressErrors(' Ana@Example.com ')).toEqual([]);
	});
});

describe('sendPilotAddress', () => {
	it('sends only the address under its key and hands back what changed', async () => {
		const result = { address: 'ana@example.com', addedAt: '2026-11-02T14:00:00Z' };
		const fetcher = answering(200, result);

		expect(await sendPilotAddress('Ana@Example.com', 'key-1', fetcher)).toEqual({ kind: 'ok', value: result });
		expect(fetcher).toHaveBeenCalledWith('/api/pilot-allowlist', {
			method: 'POST',
			credentials: 'include',
			headers: { 'Content-Type': 'application/json', 'Idempotency-Key': 'key-1' },
			body: '{"address":"Ana@Example.com"}'
		});
	});

	it('says why an address already on the list is refused', async () => {
		const message = 'ana@example.com is already on the pilot allowlist. It was added on 2 November 2026, and nothing has changed.';

		expect(await sendPilotAddress('ana@example.com', 'key-1', answering(409, { code: 'CONFLICT', message }))).toEqual({
			kind: 'refused',
			errors: [{ message }]
		});
	});

	it('puts a refused address beside its field', async () => {
		const refusal = {
			code: 'INVALID_ARGUMENT',
			message: 'Correct the answers marked below.',
			details: { address: 'Enter an email address in the correct format, like name@example.com' }
		};

		expect(await sendPilotAddress('ana@', 'key-1', answering(422, refusal))).toEqual({
			kind: 'refused',
			errors: [{ message: refusal.details.address, field: ADDRESS_FIELD }]
		});
	});

	it('names an ended session', async () => {
		expect(await sendPilotAddress('ana@example.com', 'key-1', answering(401, { message: 'ended' }))).toEqual({
			kind: 'signed-out'
		});
	});
});
