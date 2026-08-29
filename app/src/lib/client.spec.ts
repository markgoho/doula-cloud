import { describe, expect, it, vi } from 'vitest';
import { createClient, loadClients } from './client.js';

// eslint-disable-next-line unicorn/consistent-boolean-name -- mirrors the native Response.ok property this mock stands in for
function response(body: unknown, ok = true): Response {
	return {
		ok,
		text: () => Promise.resolve(typeof body === 'string' ? body : JSON.stringify(body)),
		json: () => Promise.resolve(body)
	} as Response;
}

describe('loadClients', () => {
	it('fetches the practice clients path and returns the decoded list', async () => {
		const clients = [
			{ clientId: 'client-1', name: 'Ada Lovelace', email: 'ada@example.com', hasWork: true, portalInviteStatus: 'sent' },
			{ clientId: 'client-2', name: 'Grace Hopper', email: 'grace@example.com', hasWork: false }
		];
		const fetcher = vi.fn().mockResolvedValue(response(clients));

		const result = await loadClients(fetcher, 'practice-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/clients');
		expect(result).toEqual(clients);
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(response('forbidden', false));

		await expect(loadClients(fetcher, 'practice-1')).rejects.toThrow('forbidden');
	});
});

describe('createClient', () => {
	it('posts the given name and email to the practice clients path', async () => {
		const fetcher = vi.fn().mockResolvedValue(response({ id: 'client-1' }));

		await createClient(fetcher, 'practice-1', { givenName: 'Ada', email: 'ada@example.com' });

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/clients', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ givenName: 'Ada', email: 'ada@example.com' })
		});
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(response('givenName is required', false));

		await expect(createClient(fetcher, 'practice-1', { givenName: '', email: '' })).rejects.toThrow(
			'givenName is required'
		);
	});
});
