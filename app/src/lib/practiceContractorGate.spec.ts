import { describe, expect, it, afterEach, vi } from 'vitest';
import { jsonResponse } from './testResponse.js';
import { loadContractorGate } from './practiceContractorGate.js';

function setup(status: number, body: unknown) {
	const fetchMock = vi.fn(async () => jsonResponse(body, status));
	vi.stubGlobal('fetch', fetchMock);
	return { fetchMock };
}

afterEach(() => {
	vi.unstubAllGlobals();
});

// #539: the single source of truth for every branch of this gate. Both
// `clients/+page.ts` and `clients/search/+page.ts` call this function
// directly and are covered by a thin wiring test each -- re-walking every
// branch there too would be the same duplicated-session-read problem this
// module was extracted to kill, one level up.
describe('loadContractorGate (#501, #539)', () => {
	it('reads the practice session and clears an employee Doula', async () => {
		const { fetchMock } = setup(200, { roles: ['doula'], isContractor: false });

		const result = await loadContractorGate('practice-1');

		expect(fetchMock).toHaveBeenCalledWith(
			'/api/practices/practice-1/session',
			expect.objectContaining({ credentials: 'include' })
		);
		expect(result).toEqual({ isContractor: false, isOwner: false });
	});

	it('flags an employee Owner (#691)', async () => {
		setup(200, { roles: ['owner'], isContractor: false });

		const result = await loadContractorGate('practice-1');

		expect(result).toEqual({ isContractor: false, isOwner: true });
	});

	it('flags a contractor Doula holding no owner or admin role', async () => {
		setup(200, { roles: ['doula'], isContractor: true });

		const result = await loadContractorGate('practice-1');

		expect(result).toEqual({ isContractor: true, isOwner: false });
	});

	it("clears a solo Practice's owner-contractor (ADR-0017) and flags her isOwner", async () => {
		setup(200, { roles: ['owner', 'doula'], isContractor: true });

		const result = await loadContractorGate('practice-1');

		expect(result).toEqual({ isContractor: false, isOwner: true });
	});

	it('clears an admin who also carries a contractor membership', async () => {
		setup(200, { roles: ['admin', 'doula'], isContractor: true });

		const result = await loadContractorGate('practice-1');

		expect(result).toEqual({ isContractor: false, isOwner: false });
	});

	it('falls back to isContractor: false, isOwner: false when the session read fails', async () => {
		setup(500, 'boom');

		const result = await loadContractorGate('practice-1');

		expect(result).toEqual({ isContractor: false, isOwner: false });
	});

	it('falls back to isContractor: false, isOwner: false on a network failure', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => {
				throw new Error('network down');
			})
		);

		const result = await loadContractorGate('practice-1');

		expect(result).toEqual({ isContractor: false, isOwner: false });
	});
});
