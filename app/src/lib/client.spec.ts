import { describe, expect, it, vi } from 'vitest';
import {
	clientsResultSummary,
	createClient,
	editClient,
	loadClientCount,
	loadDoulaCounts,
	loadClients,
	mergeClient,
	searchClients,
	clientOrderFrom,
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

describe('loadClientCount', () => {
	it('reads the count path and returns the two numbers', async () => {
		const fetcher = vi.fn().mockResolvedValue(response({ records: 22 }));

		await expect(loadClientCount(fetcher, 'practice-1')).resolves.toEqual({ records: 22 });
		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/clients/count');
	});

	it('throws on a refusal', async () => {
		const fetcher = vi.fn().mockResolvedValue(response({ message: 'You cannot see this Practice.' }, 403));

		await expect(loadClientCount(fetcher, 'practice-1')).rejects.toThrow('You cannot see this Practice.');
	});
});

describe('loadDoulaCounts', () => {
	it('reads the doula-counts path and returns the counts', async () => {
		const counts = { mine: 1, doulas: { 'staff-1': 1, 'staff-2': 3 } };
		const fetcher = vi.fn().mockResolvedValue(response(counts));

		await expect(loadDoulaCounts(fetcher, 'practice-1')).resolves.toEqual(counts);
		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/clients/doula-counts');
	});

	it('throws on a refusal', async () => {
		const fetcher = vi.fn().mockResolvedValue(response({ message: 'You cannot see this Practice.' }, 403));

		await expect(loadDoulaCounts(fetcher, 'practice-1')).rejects.toThrow('You cannot see this Practice.');
	});
});

describe('loadClients', () => {
	it('fetches the practice clients path and returns the decoded page', async () => {
		const page = {
			items: [
				{ clientId: 'client-1', name: 'Ada Lovelace', email: 'ada@example.com', hasWork: true, portalInviteStatus: 'invited', portalInviteDelivery: 'sent' },
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

	it('appends mine=true when showMine is set (#2624)', async () => {
		const fetcher = vi.fn().mockResolvedValue(response({ items: [], hasMore: false }));

		await loadClients(fetcher, 'practice-1', { showMine: true, cursor: 'next-page' });

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/clients?mine=true&cursor=next-page');
	});

	it('appends owes=true and noContract=true when showOwing and showNoContract are set (#2724)', async () => {
		const fetcher = vi.fn().mockResolvedValue(response({ items: [], hasMore: false }));

		await loadClients(fetcher, 'practice-1', { showMine: true, showOwing: true, showNoContract: true, cursor: 'next-page' });

		expect(fetcher).toHaveBeenCalledWith(
			'/api/practices/practice-1/clients?mine=true&owes=true&noContract=true&cursor=next-page'
		);
	});

	it('appends the Doula beside every other param', async () => {
		const fetcher = vi.fn().mockResolvedValue(response({ items: [], hasMore: false }));

		await loadClients(fetcher, 'practice-1', { showOwing: true, doula: 'staff-1', cursor: 'next-page' });

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/clients?owes=true&doula=staff-1&cursor=next-page');
	});

	it('sends a name in a POST body, with every other filter, and never in the address (#1829)', async () => {
		const fetcher = vi.fn().mockResolvedValue(response({ items: [], hasMore: false }));

		await loadClients(fetcher, 'practice-1', {
			showAll: true,
			showMine: true,
			showOwing: true,
			showNoContract: true,
			doula: 'staff-1',
			dueFrom: '2027-03-01',
			dueTo: '2027-03-15',
			name: 'Anne & Marie 100%',
			order: 'name',
			cursor: 'next-page'
		});

		const [path, init] = fetcher.mock.calls[0];
		expect(path).toBe('/api/practices/practice-1/clients/list');
		expect(path).not.toContain('Anne');
		expect(init.method).toBe('POST');
		expect(JSON.parse(init.body)).toEqual({
			name: 'Anne & Marie 100%',
			all: true,
			mine: true,
			owes: true,
			noContract: true,
			doula: 'staff-1',
			due: '',
			dueFrom: '2027-03-01',
			dueTo: '2027-03-15',
			order: 'name',
			cursor: 'next-page'
		});
	});

	it('asks for a due month, or for chosen days, after the Doula and before the order (#2871)', async () => {
		const fetcher = vi.fn().mockResolvedValue(response({ items: [], hasMore: false }));

		await loadClients(fetcher, 'practice-1', { doula: 'staff-1', due: '2027-03', order: 'name', cursor: 'next-page' });
		await loadClients(fetcher, 'practice-1', { dueFrom: '2027-03-01', dueTo: '2027-03-15' });
		await loadClients(fetcher, 'practice-1', { dueFrom: '2027-03-01' });

		expect(fetcher).toHaveBeenNthCalledWith(1, '/api/practices/practice-1/clients?doula=staff-1&due=2027-03&order=name&cursor=next-page');
		expect(fetcher).toHaveBeenNthCalledWith(2, '/api/practices/practice-1/clients?dueFrom=2027-03-01&dueTo=2027-03-15');
		expect(fetcher).toHaveBeenNthCalledWith(3, '/api/practices/practice-1/clients');
	});

	it('sends the unset filters of a named list as off', async () => {
		const fetcher = vi.fn().mockResolvedValue(response({ items: [], hasMore: false }));

		await loadClients(fetcher, 'practice-1', { name: 'Camille' });

		expect(JSON.parse(fetcher.mock.calls[0][1].body)).toEqual({
			name: 'Camille',
			all: false,
			mine: false,
			owes: false,
			noContract: false,
			doula: '',
			due: '',
			dueFrom: '',
			dueTo: '',
			order: '',
			cursor: ''
		});
	});

	it('throws with the response body text when a named list is refused', async () => {
		const fetcher = vi.fn().mockResolvedValue(response('forbidden', 403));

		await expect(loadClients(fetcher, 'practice-1', { name: 'Camille' })).rejects.toThrow('forbidden');
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
	// #1829: what was typed goes in the body of a POST, never in the URL,
	// so no request log records a Client's details.
	it('posts every field in the body, and puts none in the URL', async () => {
		const fetcher = vi.fn().mockResolvedValue(response({ matches: [] }));
		const fields = {
			name: 'Ada',
			dateOfBirth: '1815-12-10',
			email: 'ada@example.com',
			phone: '555-0100'
		};

		await searchClients(fetcher, 'practice-1', fields);

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/clients/search', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(fields)
		});
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

	it('decodes a 409 into a named conflict, carrying substitution, rather than throwing', async () => {
		const matches = [{ id: 'client-2', ...editFields, givenName: 'Ada', engagements: [], wouldSurvive: true }];
		const fetcher = vi
			.fn()
			.mockResolvedValue(response({ matches, substitution: false }, 409));

		const result = await editClient(fetcher, 'practice-1', 'client-1', editFields, false);

		expect(result).toEqual({ conflict: true, matches, substitution: false });
	});

	// #1610: edit.go answers 409 for an erased or a merged record too, with
	// a message and no matches. That is a refusal, not a gate.
	it('throws a 409 that names no match, with its own message', async () => {
		const fetcher = vi.fn().mockResolvedValue(
			response(
				{ code: 'ALREADY_EXISTS', message: "this client's data has been erased and cannot be edited" },
				409
			)
		);

		await expect(editClient(fetcher, 'practice-1', 'client-1', editFields, false)).rejects.toThrow(
			"this client's data has been erased and cannot be edited"
		);
	});

	it('says the record could not be saved where a 409 names nothing at all', async () => {
		const fetcher = vi.fn().mockResolvedValue(response({}, 409));

		await expect(editClient(fetcher, 'practice-1', 'client-1', editFields, false)).rejects.toThrow(
			'The Client record could not be saved.'
		);
	});

	it('decodes gate one (substitution: true, which offers no merge)', async () => {
		const matches = [{ id: 'client-2', ...editFields, givenName: 'Ada', engagements: [], wouldSurvive: false }];
		const fetcher = vi
			.fn()
			.mockResolvedValue(response({ matches, substitution: true }, 409));

		const result = await editClient(fetcher, 'practice-1', 'client-1', editFields, false);

		expect(result).toEqual({ conflict: true, matches, substitution: true });
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

describe('the Clients list order (#2869)', () => {
	it('puts the order in the address of a GET, before the cursor', async () => {
		const fetcher = vi.fn().mockResolvedValue(response({ items: [], hasMore: false }));

		await loadClients(fetcher, 'practice-1', { order: 'owed', cursor: 'next-page' });

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/clients?order=owed&cursor=next-page');
	});

	it('reads an order the list offers, and anything else as its own', () => {
		expect(clientOrderFrom('name')).toBe('name');
		expect(clientOrderFrom('name-desc')).toBe('name-desc');
		expect(clientOrderFrom('owed')).toBe('owed');
		expect(clientOrderFrom('overdue')).toBe('overdue');
		expect(clientOrderFrom('newest')).toBe('');
		expect(clientOrderFrom('')).toBe('');
	});

	it('ends the result line on the order\'s own words', () => {
		expect(clientsResultSummary(30, true, false, false, false, false, { order: 'name' })).toBe('Showing the first 30 Clients, by name, A to Z.');
		expect(clientsResultSummary(2, false, false, false, false, false, { order: 'name-desc' })).toBe('Showing 2 Clients, by name, Z to A.');
		expect(clientsResultSummary(2, false, false, false, true, false, { order: 'owed' })).toBe('Showing 2 Clients who owe money, most owed first.');
		expect(clientsResultSummary(2, false, false, false, false, false, { order: 'overdue' })).toBe('Showing 2 Clients, most overdue first.');
	});
});

describe('clientsResultSummary (#2068)', () => {
	it('says the list is ordered soonest due first, and the first 30 when more wait', () => {
		expect(clientsResultSummary(30, true, false)).toBe('Showing the first 30 Clients, soonest due first, then most recently ended.');
	});

	it('says how many there are in all while more exist, for every narrowing (#2867)', () => {
		const tail = ', soonest due first, then most recently ended.';
		const doula = { doulaName: 'Kimiko Nakashima' };
		expect(clientsResultSummary(30, true, false, false, false, false, {}, 58)).toBe(`Showing the first 30 of 58 Clients${tail}`);
		expect(clientsResultSummary(30, true, true, false, false, false, {}, 58)).toBe(
			`Showing the first 30 of 58 Clients, including anyone not yet in care${tail}`
		);
		expect(clientsResultSummary(30, true, false, true, false, false, {}, 58)).toBe(`Showing the first 30 of your 58 Clients${tail}`);
		expect(clientsResultSummary(30, true, false, false, false, false, doula, 58)).toBe(
			`Showing the first 30 of Kimiko Nakashima's 58 Clients${tail}`
		);
		expect(clientsResultSummary(30, true, false, false, true, true, { name: 'Cam' }, 58)).toBe(
			`Showing the first 30 of 58 active Clients named "Cam" who owe money and have no Contract${tail}`
		);
	});

	it('says the plain line once every Client is shown, and never a total smaller than the rows', () => {
		const tail = ', soonest due first, then most recently ended.';
		expect(clientsResultSummary(58, false, false, false, false, false, {}, 58)).toBe(`Showing 58 Clients${tail}`);
		expect(clientsResultSummary(31, true, false, false, false, false, {}, 30)).toBe(`Showing the first 31 of 31 Clients${tail}`);
	});

	it('says Clients in the plural when the total is, however few are shown', () => {
		expect(clientsResultSummary(1, true, false, false, true, false, {}, 2)).toBe(
			'Showing the first 1 of 2 Clients who owe money, soonest due first, then most recently ended.'
		);
	});

	it('says no total when the page carried none', () => {
		expect(clientsResultSummary(30, true, false, false, false, false, {}, undefined)).toBe(
			'Showing the first 30 Clients, soonest due first, then most recently ended.'
		);
	});

	// A loss is not a birth: the order names what it sorts by, neutrally.
	it('never calls an ended pregnancy a birth in how it says the list is ordered', () => {
		const lines = [
			clientsResultSummary(3, false, false),
			clientsResultSummary(3, true, true),
			clientsResultSummary(3, false, false, true, true, true, { doulaName: 'Lena Vasquez', name: 'Cam' })
		];
		for (const line of lines) {
			expect(line).toMatch(/, soonest due first, then most recently ended\.$/);
			expect(line).not.toMatch(/birth/i);
		}
	});

	it('counts the Clients the filter shows', () => {
		expect(clientsResultSummary(0, false, false)).toBe('No Clients in care yet.');
		expect(clientsResultSummary(0, false, true)).toBe('No Clients.');
		expect(clientsResultSummary(1, false, false)).toBe('Showing 1 Client, soonest due first, then most recently ended.');
		expect(clientsResultSummary(3, false, true)).toBe('Showing 3 Clients, including anyone not yet in care, soonest due first, then most recently ended.');
		expect(clientsResultSummary(25, true, true)).toBe('Showing the first 25 Clients, including anyone not yet in care, soonest due first, then most recently ended.');
	});

	// #2624
	it('words the list as hers while "Only my Clients" is on', () => {
		expect(clientsResultSummary(0, false, false, true)).toBe('You are not the Doula for any Client yet. Clear the filters to widen the list.');
		expect(clientsResultSummary(5, false, false, true)).toBe('Showing 5 of your Clients, soonest due first, then most recently ended.');
		expect(clientsResultSummary(30, true, false, true)).toBe('Showing the first 30 of your Clients, soonest due first, then most recently ended.');
	});

	// #2724
	it('words the list for the Clients who owe money', () => {
		expect(clientsResultSummary(3, false, false, false, true)).toBe('Showing 3 Clients who owe money, soonest due first, then most recently ended.');
		expect(clientsResultSummary(1, false, false, false, true)).toBe('Showing 1 Client who owes money, soonest due first, then most recently ended.');
		expect(clientsResultSummary(30, true, false, false, true)).toBe('Showing the first 30 Clients who owe money, soonest due first, then most recently ended.');
		expect(clientsResultSummary(0, false, false, false, true)).toBe('No Client owes money. Clear the filters to widen the list.');
	});

	it('words the list for the Clients with no Contract', () => {
		expect(clientsResultSummary(4, false, false, false, false, true)).toBe('Showing 4 active Clients with no Contract, soonest due first, then most recently ended.');
		expect(clientsResultSummary(0, false, false, false, false, true)).toBe('No active Client is missing a Contract. Clear the filters to widen the list.');
	});

	it('words the list when both apply, and when it is hers', () => {
		expect(clientsResultSummary(2, false, false, false, true, true)).toBe('Showing 2 active Clients who owe money and have no Contract, soonest due first, then most recently ended.');
		expect(clientsResultSummary(1, false, false, false, true, true)).toBe('Showing 1 active Client who owes money and has no Contract, soonest due first, then most recently ended.');
		expect(clientsResultSummary(0, false, false, false, true, true)).toBe('No active Client owes money and is missing a Contract. Clear the filters to widen the list.');
		expect(clientsResultSummary(2, false, false, true, true)).toBe('Showing 2 of your Clients who owe money, soonest due first, then most recently ended.');
		expect(clientsResultSummary(2, false, false, true, false, true)).toBe('Showing 2 of your active Clients with no Contract, soonest due first, then most recently ended.');
		expect(clientsResultSummary(0, false, false, true, true, true)).toBe('None of your active Clients owe money and are missing a Contract. Clear the filters to widen the list.');
	});

	it('words the list narrowed to a name', () => {
		const named = { name: 'Camille' };
		expect(clientsResultSummary(3, false, false, false, false, false, named)).toBe('Showing 3 Clients named "Camille", soonest due first, then most recently ended.');
		expect(clientsResultSummary(1, false, false, false, false, false, named)).toBe('Showing 1 Client named "Camille", soonest due first, then most recently ended.');
		expect(clientsResultSummary(30, true, true, false, false, false, named)).toBe('Showing the first 30 Clients named "Camille", including anyone not yet in care, soonest due first, then most recently ended.');
		expect(clientsResultSummary(0, false, false, false, false, false, named)).toBe('No Client is named "Camille". Turn on "Also show anyone not yet in care" to look among them.');
		expect(clientsResultSummary(0, false, true, false, false, false, named)).toBe('No Client is named "Camille". Clear the filters to widen the list.');
	});

	it('words the list narrowed to a Doula, and to a Doula and a name', () => {
		const doula = { doulaName: 'Kimiko Nakashima' };
		expect(clientsResultSummary(5, false, false, false, false, false, doula)).toBe("Showing Kimiko Nakashima's 5 Clients, soonest due first, then most recently ended.");
		expect(clientsResultSummary(5, false, false, false, false, false, { ...doula, name: 'Cam' })).toBe(
			'Showing Kimiko Nakashima\'s 5 Clients named "Cam", soonest due first, then most recently ended.'
		);
		expect(clientsResultSummary(1, false, false, false, false, false, doula)).toBe("Showing Kimiko Nakashima's 1 Client, soonest due first, then most recently ended.");
		expect(clientsResultSummary(30, true, false, false, false, false, doula)).toBe("Showing the first 30 of Kimiko Nakashima's Clients, soonest due first, then most recently ended.");
		expect(clientsResultSummary(2, true, false, false, false, true, doula)).toBe("Showing the first 2 of Kimiko Nakashima's active Clients with no Contract, soonest due first, then most recently ended.");
		expect(clientsResultSummary(2, false, false, false, false, true, doula)).toBe("Showing Kimiko Nakashima's 2 active Clients with no Contract, soonest due first, then most recently ended.");
		expect(clientsResultSummary(0, false, false, false, false, false, doula)).toBe('Kimiko Nakashima has no Clients. Clear the filters to widen the list.');
	});

	it('words a name beside the other narrowings, and says nobody matched them', () => {
		const named = { name: 'Cam' };
		expect(clientsResultSummary(2, false, false, false, true, true, named)).toBe(
			'Showing 2 active Clients named "Cam" who owe money and have no Contract, soonest due first, then most recently ended.'
		);
		expect(clientsResultSummary(1, false, false, false, true, true, named)).toBe(
			'Showing 1 active Client named "Cam" who owes money and has no Contract, soonest due first, then most recently ended.'
		);
		expect(clientsResultSummary(1, false, false, false, true, false, named)).toBe('Showing 1 Client named "Cam" who owes money, soonest due first, then most recently ended.');
		expect(clientsResultSummary(2, false, false, false, false, true, named)).toBe('Showing 2 active Clients named "Cam" with no Contract, soonest due first, then most recently ended.');
		expect(clientsResultSummary(2, false, false, true, false, false, named)).toBe('Showing 2 of your Clients named "Cam", soonest due first, then most recently ended.');
		expect(clientsResultSummary(0, false, false, true, false, false, named)).toBe('None of your Clients is named "Cam". Clear the filters to widen the list.');
		expect(clientsResultSummary(0, false, false, false, true, false, named)).toBe('No Client named "Cam" owes money. Clear the filters to widen the list.');
		expect(clientsResultSummary(0, false, false, false, false, true, named)).toBe('No active Client named "Cam" is missing a Contract. Clear the filters to widen the list.');
	});

	// An empty narrowed result says what was looked for, in the count line's words.
	it('names what an empty narrowed list was looking for', () => {
		const doula = { doulaName: 'Priya Raman' };
		const clear = ' Clear the filters to widen the list.';
		expect(clientsResultSummary(0, false, false, false, true, false, doula)).toBe(`None of Priya Raman's Clients owe money.${clear}`);
		expect(clientsResultSummary(0, false, false, false, false, false, { doulaName: 'Renata Alvarez' })).toBe(`Renata Alvarez has no Clients.${clear}`);
		expect(clientsResultSummary(0, false, true, false, false, false, { doulaName: 'Renata Alvarez' })).toBe(`Renata Alvarez has no Clients.${clear}`);
		expect(clientsResultSummary(0, false, false, false, false, false, { ...doula, name: 'Cam' })).toBe(`None of Priya Raman's Clients is named "Cam".${clear}`);
		expect(clientsResultSummary(0, false, false, true, false, false, doula)).toBe(`You have no Clients with Priya Raman.${clear}`);
		expect(clientsResultSummary(0, false, false, true, false, false)).toBe(`You are not the Doula for any Client yet.${clear}`);
	});

	// ADR-0017: the default list is any Engagement, in any status, or a pending Request.
	it('does not name the default list’s group, and says what the switch adds', () => {
		expect(clientsResultSummary(2, false, false)).toBe(
			'Showing 2 Clients, soonest due first, then most recently ended.'
		);
		expect(clientsResultSummary(2, false, true)).toBe(
			'Showing 2 Clients, including anyone not yet in care, soonest due first, then most recently ended.'
		);
	});

	it('says nothing of the switch beside a filter that already implies care, since it changes nothing there', () => {
		const expected = 'Showing 2 of your Clients, soonest due first, then most recently ended.';
		expect(clientsResultSummary(2, false, true, true)).toBe(expected);
		expect(clientsResultSummary(2, false, true, false, true)).toBe(
			'Showing 2 Clients who owe money, soonest due first, then most recently ended.'
		);
		expect(clientsResultSummary(2, false, true, false, false, false, { doulaName: 'Priya Raman' })).toBe(
			"Showing Priya Raman's 2 Clients, soonest due first, then most recently ended."
		);
	});

	it('keeps the switch in the line beside a name, which implies no care', () => {
		expect(clientsResultSummary(2, false, true, false, false, false, { name: 'Cam' })).toBe(
			'Showing 2 Clients named "Cam", including anyone not yet in care, soonest due first, then most recently ended.'
		);
	});

	describe('beside a due month or days (#2871)', () => {
		const march = { month: '2027-03', from: '', to: '' };
		const days = { month: '', from: '2027-03-01', to: '2027-03-15' };
		const tail = ', soonest due first, then most recently ended.';

		it('says when after the Clients, her Doula and a name, and before what they owe', () => {
			expect(clientsResultSummary(4, false, false, false, false, false, { due: march })).toBe(`Showing 4 Clients due in March 2027${tail}`);
			expect(clientsResultSummary(2, false, false, false, false, false, { doulaName: 'Priya Raman', due: march })).toBe(
				`Showing Priya Raman's 2 Clients due in March 2027${tail}`
			);
			expect(clientsResultSummary(30, true, false, false, true, false, { name: 'Cam', due: days }, 41)).toBe(
				`Showing the first 30 of 41 Clients named "Cam" due Mar 1 to Mar 15, 2027 who owe money${tail}`
			);
		});

		it('says nothing of the switch for the rest, which a due date makes no difference to', () => {
			expect(clientsResultSummary(1, false, true, false, false, false, { due: march })).toBe(`Showing 1 Client due in March 2027${tail}`);
		});

		it('says nobody is due when nobody is, in the words of the narrowing', () => {
			expect(clientsResultSummary(0, false, false, false, false, false, { due: march })).toBe(
				'No Client is due in March 2027. Clear the filters to widen the list.'
			);
			expect(clientsResultSummary(0, false, false, false, false, false, { due: days, name: 'Cam' })).toBe(
				'No Client named "Cam" is due Mar 1 to Mar 15, 2027. Clear the filters to widen the list.'
			);
			expect(clientsResultSummary(0, false, false, false, true, false, { doulaName: 'Priya Raman', due: march })).toBe(
				"None of Priya Raman's Clients owe money and are due in March 2027. Clear the filters to widen the list."
			);
			expect(clientsResultSummary(0, false, false, false, false, false, { doulaName: 'Priya Raman', due: march })).toBe(
				"None of Priya Raman's Clients are due in March 2027. Clear the filters to widen the list."
			);
		});
	});
});
