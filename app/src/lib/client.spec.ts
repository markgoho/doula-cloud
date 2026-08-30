import { describe, expect, it, vi } from 'vitest';
import { createClient, editClient, loadClients, type ClientEditFields } from './client.js';
import { jsonResponse as response } from './testResponse.js';

const editFields: ClientEditFields = {
	givenName: 'Ada',
	familyName: 'Lovelace',
	preferredName: '',
	email: 'ada@example.com',
	phone: '',
	addressLine1: '',
	addressLine2: '',
	addressLocality: '',
	addressRegion: '',
	addressPostalCode: '',
	dateOfBirth: '',
	fieldValues: { pronouns: 'she/her' }
};

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

describe('editClient', () => {
	it('puts the full record, override flag included, to the client path', async () => {
		const saved = { id: 'client-1', ...editFields };
		const fetcher = vi.fn().mockResolvedValue(response(saved));

		const result = await editClient(fetcher, 'practice-1', 'client-1', editFields, false);

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/clients/client-1', {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ ...editFields, override: false })
		});
		expect(result).toEqual({ conflict: false, record: saved });
	});

	it('sends override: true when told to', async () => {
		const fetcher = vi.fn().mockResolvedValue(response({ id: 'client-1', ...editFields }));

		await editClient(fetcher, 'practice-1', 'client-1', editFields, true);

		const body: { override: boolean } = JSON.parse(fetcher.mock.calls[0][1].body as string);
		expect(body.override).toBe(true);
	});

	it('decodes a 409 into a named conflict rather than throwing', async () => {
		const matches = [{ id: 'client-2', ...editFields, givenName: 'Ada', engagements: [] }];
		const fetcher = vi.fn().mockResolvedValue(response({ matches }, 409));

		const result = await editClient(fetcher, 'practice-1', 'client-1', editFields, false);

		expect(result).toEqual({ conflict: true, matches });
	});

	it('throws with the response body text on a non-conflict, non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(response('client not found', 404));

		await expect(editClient(fetcher, 'practice-1', 'client-1', editFields, false)).rejects.toThrow(
			'client not found'
		);
	});
});
