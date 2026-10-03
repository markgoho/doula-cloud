import { describe, expect, it, vi } from 'vitest';

// #1609: the predicate itself is covered once in #lib/roles.spec.ts. This
// load only has to prove the wiring: it reads the ancestor layout's
// already-resolved Membership through `parent()`, the same as the
// search's own load.
describe('clients/new/+layout.ts load (#1609)', () => {
	it.each([
		[['doula'], true, true],
		[['owner'], false, false]
	])('reads %j with isContractor %s as a contractor Doula: %s', async (roles, isContractor, expected) => {
		const { load } = await import('./+layout.js');
		const parent = vi.fn().mockResolvedValue({
			session: { practiceId: 'practice-1', practiceName: 'Test Practice', roles, isContractor }
		});

		const result = await load({ parent } as unknown as Parameters<typeof load>[0]);

		expect(parent).toHaveBeenCalled();
		expect(result).toEqual({ isContractor: expected });
	});
});
