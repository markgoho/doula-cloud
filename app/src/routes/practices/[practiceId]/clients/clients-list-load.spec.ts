import { beforeEach, describe, expect, it, vi } from 'vitest';
import { jsonResponse } from '#lib/testResponse.js';
import { loadEvent } from '#testing/loadEvent.js';
import { load, type ClientsListGate } from './+page.js';

// Only the row count matters to this load, so a row is its id and its work.
const withWork = { clientId: 'client-1', hasWork: true };
const withoutWork = { clientId: 'client-2', hasWork: false };

const apiFetch = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetch }));

const practiceId = 'practice-1';

function membership(roles: string[], isContractor = false) {
	return {
		session: { practiceId, practiceName: 'Test Practice', roles, isContractor },
	};
}

const loadFetch = vi.fn();

async function runLoad(roles: string[], isContractor = false) {
	const parent = vi.fn().mockResolvedValue(membership(roles, isContractor));
	const result = (await load(
		loadEvent(load, { params: { practiceId }, parent, fetch: loadFetch })
	)) as ClientsListGate;
	return { result, parent };
}

beforeEach(() => {
	apiFetch.mockReset();
});

// #539: every branch of the predicates themselves -- session-read shape,
// owner/admin carve-out -- is covered once in #lib/roles.spec.ts. This
// route's own `load` only has to prove the wiring: it reads the ancestor
// layout's already-resolved Membership through `parent()`, rather than a
// second `/api/practices/${practiceId}/session` fetch of its own.
describe('clients/+page.ts load (#539)', () => {
	it("derives the contractor gate from the ancestor layout's Membership", async () => {
		const { result, parent } = await runLoad(['doula'], true);

		expect(parent).toHaveBeenCalled();
		expect(result).toMatchObject({ isContractor: true, isOwner: false });
	});

	// She has no header link, so the count would decide nothing.
	it('registers the read "Try again" invalidates', async () => {
		const depends = vi.fn();
		const parent = vi.fn().mockResolvedValue(membership(['doula'], true));
		await load(loadEvent(load, { params: { practiceId }, parent, fetch: loadFetch, depends }));

		expect(depends).toHaveBeenCalledWith('app:clients');
	});

	it('reads no count for a contractor Doula', async () => {
		await runLoad(['doula'], true);

		expect(apiFetch).not.toHaveBeenCalled();
	});

	it('clears the gate for an owner', async () => {
		apiFetch.mockResolvedValue(
			jsonResponse({ items: [withWork, withoutWork], hasMore: false })
		);

		const { result } = await runLoad(['owner']);

		expect(result).toMatchObject({ isContractor: false, isOwner: true, isDoula: false });
	});

	// #2624: the role that decides whether "Mine" is drawn.
	it('reads whether she holds the Doula role', async () => {
		apiFetch.mockResolvedValue(jsonResponse({ items: [withWork], hasMore: false }));

		const { result } = await runLoad(['doula']);

		expect(result).toMatchObject({ isContractor: false, isDoula: true });
	});

	// #2675: only an explicit `false` from the session withholds the switch.
	it('reads whether another Doula holds the role', async () => {
		apiFetch.mockResolvedValue(jsonResponse({ items: [withWork], hasMore: false }));

		const alone = { ...membership(['doula']).session, hasOtherDoulas: false };
		const parent = vi.fn().mockResolvedValue({ session: alone });
		const { hasOtherDoulas } = (await load(
			loadEvent(load, { params: { practiceId }, parent, fetch: loadFetch })
		)) as ClientsListGate;
		const { result } = await runLoad(['doula']);

		expect(hasOtherDoulas).toBe(false);
		expect(result.hasOtherDoulas).toBe(true);
	});
});

const countsPath = `/api/practices/${practiceId}/clients/doula-counts`;

// staff-1 holds two Clients and staff-3 none; the reader holds none.
const counts = { mine: 0, doulas: { 'staff-1': 2 } };

function answer(rosterResponse: Response, countsResponse: Response = jsonResponse(counts)) {
	apiFetch.mockImplementation((path: string) =>
		Promise.resolve(
			path.endsWith('/staff') ? rosterResponse : (path.endsWith('/doula-counts') ? countsResponse : jsonResponse({ items: [withWork], hasMore: false }))
		)
	);
}

// The "Doula" filter's options come from the Staff roster, which only an
// Owner or an Admin may read (ADR-0008), and only the people who hold the
// Doula role are offered.
describe('the Doulas the list can be narrowed to', () => {
	const roster = {
		members: [
			{ staffId: 'staff-1', name: 'Kimiko Nakashima', roles: ['doula'], employmentType: 'employee' },
			{ staffId: 'staff-2', name: 'Dee Whitlock', roles: ['admin'], employmentType: 'employee' },
			{ staffId: 'staff-3', name: 'Lena Vasquez', roles: ['doula'], employmentType: 'contractor' }
		]
	};

	it('are the roster\'s Doulas, contractors included, for an Owner', async () => {
		answer(jsonResponse(roster), jsonResponse({ mine: 0, doulas: { 'staff-1': 2, 'staff-3': 1 } }));

		const { result } = await runLoad(['owner']);

		await expect(result.doulas).resolves.toEqual([
			{ value: 'staff-1', label: 'Kimiko Nakashima' },
			{ value: 'staff-3', label: 'Lena Vasquez' }
		]);
	});

	it('leaves out a Doula who holds no Client, rather than listing her as empty', async () => {
		answer(jsonResponse(roster));

		const { result } = await runLoad(['owner']);

		await expect(result.doulas).resolves.toEqual([{ value: 'staff-1', label: 'Kimiko Nakashima' }]);
	});

	it('keeps every Doula on the roster apart from the ones who hold a Client, to name a shared link', async () => {
		answer(jsonResponse(roster));

		const { result } = await runLoad(['owner']);

		await expect(result.everyDoula).resolves.toEqual([
			{ value: 'staff-1', label: 'Kimiko Nakashima' },
			{ value: 'staff-3', label: 'Lena Vasquez' }
		]);
		expect(result.canChooseDoula).toBe(true);
	});

	it('are not chosen by a Doula, and her list opens on Mine', async () => {
		const { result } = await runLoad(['doula']);

		expect(result).toMatchObject({ canChooseDoula: false, opensOnMine: true });
		await expect(result.everyDoula).resolves.toEqual([]);
	});

	it('reads the counts once, however many Doulas there are', async () => {
		answer(jsonResponse(roster));

		const { result } = await runLoad(['owner']);
		await result.doulas;

		expect(apiFetch.mock.calls.filter(([path]) => path === countsPath)).toHaveLength(1);
		expect(apiFetch).not.toHaveBeenCalledWith(expect.stringContaining('doula='), undefined, loadFetch);
	});

	it('keeps a Doula unmarked when the counts cannot be read', async () => {
		apiFetch.mockImplementation((path: string) =>
			path.endsWith('/staff') ? Promise.resolve(jsonResponse(roster)) : Promise.reject(new Error('offline'))
		);

		const { result } = await runLoad(['owner']);

		await expect(result.doulas).resolves.toEqual([
			{ value: 'staff-1', label: 'Kimiko Nakashima' },
			{ value: 'staff-3', label: 'Lena Vasquez' }
		]);
	});

	it('keeps a Doula unmarked when the counts are refused', async () => {
		answer(jsonResponse(roster), jsonResponse({ message: 'nope' }, 500));

		const { result } = await runLoad(['owner']);

		await expect(result.doulas).resolves.toEqual([
			{ value: 'staff-1', label: 'Kimiko Nakashima' },
			{ value: 'staff-3', label: 'Lena Vasquez' }
		]);
	});

	it('leaves out a Doula the counts hold at zero, as well as one they omit', async () => {
		answer(jsonResponse(roster), jsonResponse({ mine: 0, doulas: { 'staff-1': 0, 'staff-3': 1 } }));

		const { result } = await runLoad(['owner']);

		await expect(result.doulas).resolves.toEqual([{ value: 'staff-3', label: 'Lena Vasquez' }]);
	});

	it('are read for an Admin too', async () => {
		answer(jsonResponse(roster), jsonResponse({ mine: 0, doulas: { 'staff-1': 2, 'staff-3': 1 } }));

		const { result } = await runLoad(['admin']);

		await expect(result.doulas).resolves.toHaveLength(2);
	});

	it('are not read for a Doula, who may not read the roster', async () => {
		const { result } = await runLoad(['doula']);

		await expect(result.doulas).resolves.toEqual([]);
		expect(apiFetch).not.toHaveBeenCalledWith(`/api/practices/${practiceId}/staff`, undefined, loadFetch);
	});

	it('are none when the roster is refused', async () => {
		answer(jsonResponse({ error: 'nope' }, 403));

		const { result } = await runLoad(['owner']);

		await expect(result.doulas).resolves.toEqual([]);
	});

	it('are none when the roster cannot be reached', async () => {
		apiFetch.mockRejectedValue(new Error('offline'));

		const { result } = await runLoad(['owner']);

		await expect(result.doulas).resolves.toEqual([]);
	});
});

// "Mine" is drawn only for a Doula who holds a live Client.
describe('whether she holds a live Client', () => {
	it('holds one when her own count is above zero', async () => {
		answer(jsonResponse({}), jsonResponse({ mine: 3, doulas: {} }));

		const { result } = await runLoad(['doula']);

		await expect(result.hasMine).resolves.toBe(true);
		expect(apiFetch).toHaveBeenCalledWith(countsPath, undefined, loadFetch);
		expect(apiFetch).not.toHaveBeenCalledWith(expect.stringContaining('mine=true'), undefined, loadFetch);
	});

	it('holds none when her own count is zero', async () => {
		answer(jsonResponse({}), jsonResponse({ mine: 0, doulas: {} }));

		const { result } = await runLoad(['doula']);

		await expect(result.hasMine).resolves.toBe(false);
	});

	it('is drawn when the read fails, as it was before the read', async () => {
		answer(jsonResponse({}), jsonResponse({ error: 'nope' }, 500));

		const { result } = await runLoad(['doula']);

		await expect(result.hasMine).resolves.toBe(true);
	});

	it('is not read for anyone who is not a Doula', async () => {
		apiFetch.mockResolvedValue(jsonResponse({ items: [withWork], hasMore: false }));

		const { result } = await runLoad(['owner']);

		await expect(result.hasMine).resolves.toBe(false);
		expect(apiFetch).not.toHaveBeenCalledWith(expect.stringContaining('mine=true'), undefined, loadFetch);
	});

	it('shares the one counts read with the Doula filter for an owner who is also a Doula', async () => {
		answer(jsonResponse({ members: [] }), jsonResponse({ mine: 1, doulas: {} }));

		const { result } = await runLoad(['owner', 'doula']);
		await Promise.all([result.hasMine, result.doulas]);

		expect(apiFetch.mock.calls.filter(([path]) => path === countsPath)).toHaveLength(1);
	});
});

// #1609 (ADR-0017's amendment of 2026-10-02): the count of Client
// records, read with `all=true`, decides where the header link goes --
// never the "Clients with work" rows the screen shows.
describe('whether the Practice holds a Client record (#1609)', () => {
	it('reads the count with all=true, not the filtered list', async () => {
		apiFetch.mockResolvedValue(jsonResponse({ items: [], hasMore: false }));

		await runLoad(['owner']);

		expect(apiFetch).toHaveBeenCalledWith(
			`/api/practices/${practiceId}/clients?all=true`,
			undefined,
			loadFetch
		);
	});

	it('holds none at a Practice with no Client', async () => {
		apiFetch.mockResolvedValue(jsonResponse({ items: [], hasMore: false }));

		const { result } = await runLoad(['owner']);

		await expect(result.hasAnyClient).resolves.toBe(false);
	});

	// A Client with no work yet is hidden by the default filter, and the
	// count still finds her. An erased Client is a record the same way,
	// and `all=true` returns her too.
	it('holds one at a Practice whose only Client has no work', async () => {
		apiFetch.mockResolvedValue(
			jsonResponse({ items: [withoutWork], hasMore: false })
		);

		const { result } = await runLoad(['owner']);

		await expect(result.hasAnyClient).resolves.toBe(true);
	});

	it('holds one at a Practice whose only Client has work', async () => {
		apiFetch.mockResolvedValue(
			jsonResponse({ items: [withWork], hasMore: false })
		);

		const { result } = await runLoad(['owner']);

		await expect(result.hasAnyClient).resolves.toBe(true);
	});

	// The search works at a Practice of any size, so a failed count falls
	// back to it rather than to a link that might skip a returning Client.
	it('falls back to the search when the count cannot be read', async () => {
		apiFetch.mockResolvedValue(jsonResponse({ error: 'nope' }, 500));

		const { result } = await runLoad(['owner']);

		await expect(result.hasAnyClient).resolves.toBe(true);
	});
});
