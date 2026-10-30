import { describe, expect, it, vi } from 'vitest';

vi.mock('$app/paths', () => ({
	resolve: (route: string, parameters: Record<string, string>) =>
		route.replaceAll(/\[(\w+)\]/g, (_match, key: string) => parameters[key]!)
}));

/*
 * #2244: a question's own page and "What kind of answer" both end in a
 * save that plans/mount.go refuses to anyone but an Owner (#1407). The
 * list draws no way in for anyone else, and this load sends someone who
 * arrives by address anyway to the list, which she may read.
 */
async function run(roles: string[], planType = 'birth_plan') {
	const { load } = await import('./+layout.js');
	const parent = vi.fn().mockResolvedValue({ session: { practiceId: 'practice-1', roles } });
	return load({ parent, params: { practiceId: 'practice-1', planType } } as unknown as Parameters<typeof load>[0]);
}

describe('plan-templates/[planType]/+layout.ts load (#2244)', () => {
	it('lets an Owner through', async () => {
		await expect(run(['owner'])).resolves.toEqual({});
	});

	it.each([[['admin']], [['doula']]])('sends %j to the list, on the plan type she came for', async (roles) => {
		await expect(run(roles)).rejects.toMatchObject({
			status: 307,
			location: '/practices/practice-1/settings/plan-templates?plan=birth_plan'
		});
	});

	it('sends her to the Care Plan list when the address names no plan type', async () => {
		await expect(run(['doula'], 'nonsense')).rejects.toMatchObject({
			status: 307,
			location: '/practices/practice-1/settings/plan-templates'
		});
	});
});
