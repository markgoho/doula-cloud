import { describe, expect, it, vi } from 'vitest';
import {
	createClient,
	editClient,
	loadClients,
	mergeClient,
	searchClients,
	type ClientEditFields
} from './client.js';
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

		await loadClients(fetcher, 'practice-1', { cursor: 'next-page' });

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/clients?cursor=next-page');
	});

	it('appends all=true when showAll is set', async () => {
		const fetcher = vi.fn().mockResolvedValue(response({ items: [], hasMore: false }));

		await loadClients(fetcher, 'practice-1', { showAll: true });

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/clients?all=true');
	});

	it('combines all=true and the cursor when both are given', async () => {
		const fetcher = vi.fn().mockResolvedValue(response({ items: [], hasMore: false }));

		await loadClients(fetcher, 'practice-1', { showAll: true, cursor: 'next-page' });

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/clients?all=true&cursor=next-page');
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(response('forbidden', 403));

		await expect(loadClients(fetcher, 'practice-1')).rejects.toThrow('forbidden');
	});
});

const createFields = {
	givenName: 'Ada',
	familyName: 'Lovelace',
	preferredName: '',
	email: 'ada@example.com',
	phone: '',
	dateOfBirth: ''
};

describe('createClient', () => {
	it('posts the typed fields and override flag to the practice clients path', async () => {
		const saved = { id: 'client-1', ...createFields, addressLine1: '', addressLine2: '', addressLocality: '', addressRegion: '', addressPostalCode: '' };
		const fetcher = vi.fn().mockResolvedValue(response(saved));

		const result = await createClient(fetcher, 'practice-1', createFields, false);

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/clients', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ ...createFields, override: false })
		});
		expect(result).toEqual({ conflict: false, record: saved });
	});

	it('sends override: true when told to', async () => {
		const fetcher = vi.fn().mockResolvedValue(response({ id: 'client-1', ...createFields }));

		await createClient(fetcher, 'practice-1', createFields, true);

		const body: { override: boolean } = JSON.parse(fetcher.mock.calls[0][1].body as string);
		expect(body.override).toBe(true);
	});

	it('decodes a 409 into a named conflict rather than throwing', async () => {
		const matches = [{ id: 'client-2', ...editFields, givenName: 'Ada', engagements: [] }];
		const fetcher = vi.fn().mockResolvedValue(response({ matches }, 409));

		const result = await createClient(fetcher, 'practice-1', createFields, false);

		expect(result).toEqual({ conflict: true, matches });
	});

	it('throws with the response body text on a non-conflict, non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(response('givenName is required', 400));

		await expect(
			createClient(fetcher, 'practice-1', { ...createFields, givenName: '' }, false)
		).rejects.toThrow('givenName is required');
	});
});

describe('searchClients', () => {
	it('queries only the fields given, in order', async () => {
		const fetcher = vi.fn().mockResolvedValue(response({ matches: [] }));

		await searchClients(fetcher, 'practice-1', { name: 'Ada', dateOfBirth: '', email: '', phone: '' });

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/clients/search?name=Ada');
	});

	it('queries every field when all four are given', async () => {
		const fetcher = vi.fn().mockResolvedValue(response({ matches: [] }));

		await searchClients(fetcher, 'practice-1', {
			name: 'Ada',
			dateOfBirth: '1815-12-10',
			email: 'ada@example.com',
			phone: '555-0100'
		});

		expect(fetcher).toHaveBeenCalledWith(
			'/api/practices/practice-1/clients/search?name=Ada&dateOfBirth=1815-12-10&email=ada%40example.com&phone=555-0100'
		);
	});

	it('queries with no query string when every field is blank', async () => {
		const fetcher = vi.fn().mockResolvedValue(response({ matches: [] }));

		await searchClients(fetcher, 'practice-1', { name: '', dateOfBirth: '', email: '', phone: '' });

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/clients/search');
	});

	it('returns the decoded matches', async () => {
		const matches = [{ id: 'client-2', ...editFields, givenName: 'Ada', engagements: [] }];
		const fetcher = vi.fn().mockResolvedValue(response({ matches }));

		const result = await searchClients(fetcher, 'practice-1', {
			name: 'Ada',
			dateOfBirth: '',
			email: '',
			phone: ''
		});

		expect(result).toEqual(matches);
	});

	it('throws with the response body text on a non-ok response, including a contractor 403', async () => {
		const fetcher = vi.fn().mockResolvedValue(
			response('a contractor doula does not search for clients at a practice she contracts for', 403)
		);

		await expect(
			searchClients(fetcher, 'practice-1', { name: 'Ada', dateOfBirth: '', email: '', phone: '' })
		).rejects.toThrow('a contractor doula does not search for clients');
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

	it('decodes a 409 into a named conflict, carrying substitution and mergeOffered, rather than throwing', async () => {
		const matches = [{ id: 'client-2', ...editFields, givenName: 'Ada', engagements: [], wouldSurvive: true }];
		const fetcher = vi
			.fn()
			.mockResolvedValue(response({ matches, substitution: false, mergeOffered: true }, 409));

		const result = await editClient(fetcher, 'practice-1', 'client-1', editFields, false);

		expect(result).toEqual({ conflict: true, matches, substitution: false, mergeOffered: true });
	});

	it('decodes gate one (substitution: true, mergeOffered always false)', async () => {
		const matches = [{ id: 'client-2', ...editFields, givenName: 'Ada', engagements: [], wouldSurvive: false }];
		const fetcher = vi
			.fn()
			.mockResolvedValue(response({ matches, substitution: true, mergeOffered: false }, 409));

		const result = await editClient(fetcher, 'practice-1', 'client-1', editFields, false);

		expect(result).toEqual({ conflict: true, matches, substitution: true, mergeOffered: false });
	});

	it('throws with the response body text on a non-conflict, non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(response('client not found', 404));

		await expect(editClient(fetcher, 'practice-1', 'client-1', editFields, false)).rejects.toThrow(
			'client not found'
		);
	});
});

describe('mergeClient', () => {
	it('posts the typed fields plus otherClientId to the merge path', async () => {
		const survivor = { id: 'client-2', ...editFields };
		const fetcher = vi.fn().mockResolvedValue(response(survivor));

		const result = await mergeClient(fetcher, 'practice-1', 'client-1', editFields, 'client-2');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/clients/client-1/merge', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ ...editFields, otherClientId: 'client-2' })
		});
		expect(result).toEqual(survivor);
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(response('this client record has already been merged into another', 409));

		await expect(
			mergeClient(fetcher, 'practice-1', 'client-1', editFields, 'client-2')
		).rejects.toThrow('this client record has already been merged into another');
	});
});
