import { describe, expect, it, vi } from 'vitest';
import { actKey, postAct } from './act.ts';

describe('actKey', () => {
	it('names each act anew', () => {
		expect(actKey()).not.toBe(actKey());
	});
});

describe('postAct', () => {
	it('sends the body under the Idempotency-Key, with the session cookie', async () => {
		const fetcher = vi.fn(async () => new Response('{}'));

		await postAct('/api/an-act', 'key-1', { credits: 2 }, fetcher);

		expect(fetcher).toHaveBeenCalledWith('/api/an-act', {
			method: 'POST',
			credentials: 'include',
			headers: { 'Content-Type': 'application/json', 'Idempotency-Key': 'key-1' },
			body: '{"credits":2}'
		});
	});

	it('sends an empty object when the act names nothing', async () => {
		const fetcher = vi.fn(async () => new Response('{}'));

		await postAct('/api/an-act', 'key-1', undefined, fetcher);

		expect(fetcher).toHaveBeenCalledWith('/api/an-act', expect.objectContaining({ body: '{}' }));
	});
});
