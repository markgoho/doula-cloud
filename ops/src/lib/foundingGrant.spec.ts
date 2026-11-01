import { describe, expect, it, vi } from 'vitest';
import { credits, readFoundingGrantCheck, sendFoundingGrant } from './foundingGrant.ts';

function answering(status: number, body?: unknown) {
	return vi.fn<typeof fetch>(
		async () => new Response(body === undefined ? undefined : JSON.stringify(body), { status })
	);
}

describe('credits', () => {
	it('counts one Credit and many', () => {
		expect(credits(1)).toBe('1 Credit');
		expect(credits(12)).toBe('12 Credits');
	});
});

describe('readFoundingGrantCheck', () => {
	it('reads the Practice and the grant its Staff size', async () => {
		// eslint-disable-next-line unicorn/no-null -- doula-ops answers null for a Practice with no grant yet
		const check = { practiceName: 'Rooted', staffCount: 4, credits: 12, grantedCredits: null };
		const fetcher = answering(200, check);

		expect(await readFoundingGrantCheck('p-1', fetcher)).toEqual({ kind: 'ok', value: check });
		expect(fetcher).toHaveBeenCalledWith('/api/practices/p-1/founding-grant', { credentials: 'include' });
	});

	it('names an ended session', async () => {
		expect(await readFoundingGrantCheck('p-1', answering(401, { message: 'ended' }))).toEqual({ kind: 'signed-out' });
	});
});

describe('sendFoundingGrant', () => {
	it('sends no fields under its key and hands back what changed', async () => {
		const result = { practiceName: 'Rooted', staffCount: 4, credits: 12 };
		const fetcher = answering(200, result);

		expect(await sendFoundingGrant('p-1', 'key-1', fetcher)).toEqual({ kind: 'ok', value: result });
		expect(fetcher).toHaveBeenCalledWith('/api/practices/p-1/founding-grants', {
			method: 'POST',
			credentials: 'include',
			headers: { 'Content-Type': 'application/json', 'Idempotency-Key': 'key-1' },
			body: '{}'
		});
	});

	it('says why a grant is refused', async () => {
		const message = 'This Practice already holds its founding grant of 12 Credits. A founding grant is given once.';

		expect(await sendFoundingGrant('p-1', 'key-1', answering(409, { code: 'CONFLICT', message }))).toEqual({
			kind: 'refused',
			errors: [{ message }]
		});
	});
});
