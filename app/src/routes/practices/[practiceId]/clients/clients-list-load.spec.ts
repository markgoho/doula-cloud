import { beforeEach, describe, expect, it, vi } from 'vitest';
import { jsonResponse } from '#lib/testResponse.js';
import type { ClientsListGate } from './+page.js';

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

async function runLoad(roles: string[], isContractor = false) {
	const { load } = await import('./+page.js');
	const parent = vi.fn().mockResolvedValue(membership(roles, isContractor));
	const result = (await load({
		params: { practiceId },
		parent,
	} as unknown as Parameters<typeof load>[0])) as ClientsListGate;
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
	it('reads no count for a contractor Doula', async () => {
		await runLoad(['doula'], true);

		expect(apiFetch).not.toHaveBeenCalled();
	});

	it('clears the gate for an owner', async () => {
		apiFetch.mockResolvedValue(
			jsonResponse({ items: [withWork, withoutWork], hasMore: false })
		);

		const { result } = await runLoad(['owner']);

		expect(result).toMatchObject({ isContractor: false, isOwner: true });
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
			`/api/practices/${practiceId}/clients?all=true`
		);
	});

	it('holds none at a Practice with no Client', async () => {
		apiFetch.mockResolvedValue(jsonResponse({ items: [], hasMore: false }));

		const { result } = await runLoad(['owner']);

		expect(result.hasAnyClient).toBe(false);
	});

	// A Client with no work yet is hidden by the default filter, and the
	// count still finds her. An erased Client is a record the same way,
	// and `all=true` returns her too.
	it('holds one at a Practice whose only Client has no work', async () => {
		apiFetch.mockResolvedValue(
			jsonResponse({ items: [withoutWork], hasMore: false })
		);

		const { result } = await runLoad(['owner']);

		expect(result.hasAnyClient).toBe(true);
	});

	it('holds one at a Practice whose only Client has work', async () => {
		apiFetch.mockResolvedValue(
			jsonResponse({ items: [withWork], hasMore: false })
		);

		const { result } = await runLoad(['owner']);

		expect(result.hasAnyClient).toBe(true);
	});

	// The search works at a Practice of any size, so a failed count falls
	// back to it rather than to a link that might skip a returning Client.
	it('falls back to the search when the count cannot be read', async () => {
		apiFetch.mockResolvedValue(jsonResponse({ error: 'nope' }, 500));

		const { result } = await runLoad(['owner']);

		expect(result.hasAnyClient).toBe(true);
	});
});
