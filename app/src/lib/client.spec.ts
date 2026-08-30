import { describe, expect, it, vi } from 'vitest';
import { createClient, loadClients } from './client.js';
import { jsonResponse as response } from './testResponse.js';

describe('loadClients', () => {
	it('fetches the practice clients path and returns the decoded page', async () => {
		const page = {
			items: [
				{ clientId: 'client-1', name: 'Ada Lovelace', email: 'ada@example.com', hasWork: true, portalInviteStatus: 'sent' },
				{ clientId: 'client-2', name: 'Grace Hopper', email: 'grace@example.com', hasWork: false }
			],
			hasMore: false
		};
		const fetcher = vi.fn().mockResolvedValue(response(page));

		const result = await loadClients(fetcher, 'practice-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/clients');
		expect(result).toEqual(page);
	});

	it('appends the cursor when given one', async () => {
		const fetcher = vi.fn().mockResolvedValue(response({ items: [], hasMore: false }));

		await loadClients(fetcher, 'practice-1', 'next-page');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/clients?cursor=next-page');
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(response('forbidden', 403));

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
		const fetcher = vi.fn().mockResolvedValue(response('givenName is required', 400));

		await expect(createClient(fetcher, 'practice-1', { givenName: '', email: '' })).rejects.toThrow(
			'givenName is required'
		);
	});
});
