import { describe, expect, it, vi } from 'vitest';

// #465, mirroring clients-list-load.spec.ts (#539): every branch of the
// predicates themselves lives in #lib/roles.spec.ts, so this route's own
// `load` only has to prove the wiring: it reads the ancestor layout's
// already-resolved Membership through `parent()`.
describe('clients/[clientId]/+page.ts load (#465, #691)', () => {
	it("derives the contractor gate from the ancestor layout's Membership", async () => {
		const { load } = await import('./+page.js');
		const parent = vi.fn().mockResolvedValue({
			session: { practiceId: 'practice-1', practiceName: 'Test Practice', roles: ['doula'], isContractor: true }
		});

		const result = await load({ parent } as unknown as Parameters<typeof load>[0]);

		expect(parent).toHaveBeenCalled();
		expect(result).toEqual({ isContractor: true, isOwner: false });
	});

	it('flags isOwner for an Owner (#691)', async () => {
		const { load } = await import('./+page.js');
		const parent = vi.fn().mockResolvedValue({
			session: { practiceId: 'practice-1', practiceName: 'Test Practice', roles: ['owner'], isContractor: false }
		});

		const result = await load({ parent } as unknown as Parameters<typeof load>[0]);

		expect(result).toEqual({ isContractor: false, isOwner: true });
	});
});
