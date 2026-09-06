import { describe, expect, it, vi } from 'vitest';

// #539: every branch of the predicates themselves -- session-read shape,
// owner/admin carve-out -- is covered once in #lib/roles.spec.ts. This
// route's own `load` only has to prove the wiring: it reads the ancestor
// layout's already-resolved Membership through `parent()`, rather than a
// second `/api/practices/${practiceId}/session` fetch of its own.
describe('clients/+page.ts load (#539)', () => {
	it("derives the contractor gate from the ancestor layout's Membership", async () => {
		const { load } = await import('./+page.js');
		const parent = vi.fn().mockResolvedValue({
			session: { practiceId: 'practice-1', practiceName: 'Test Practice', roles: ['doula'], isContractor: true }
		});

		const result = await load({ parent } as unknown as Parameters<typeof load>[0]);

		expect(parent).toHaveBeenCalled();
		expect(result).toEqual({ isContractor: true, isOwner: false });
	});

	it('clears the gate for an owner', async () => {
		const { load } = await import('./+page.js');
		const parent = vi.fn().mockResolvedValue({
			session: { practiceId: 'practice-1', practiceName: 'Test Practice', roles: ['owner'], isContractor: false }
		});

		const result = await load({ parent } as unknown as Parameters<typeof load>[0]);

		expect(result).toEqual({ isContractor: false, isOwner: true });
	});
});
