import { describe, expect, it, vi, afterEach } from 'vitest';
import { jsonResponse } from '#lib/testResponse.js';

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

function setup(byPath: Record<string, { status: number; body: unknown } | undefined>) {
	const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
		const path = String(input);
		const entry = byPath[path];
		if (!entry) throw new Error(`unexpected fetch: ${path}`);
		return jsonResponse(entry.body, entry.status);
	});
	vi.stubGlobal('fetch', fetchMock);
	return { fetchMock };
}

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('/+page.ts load', () => {
	it('redirects a signed-in Staff visitor to her only Practice', async () => {
		const { load } = await import('./+page.js');
		setup({
			'/api/staff/session': {
				status: 200,
				body: { memberships: [{ practiceId: 'practice-1', practiceName: 'Riverside Doulas', roles: ['owner'] }] }
			}
		});

		await expect(load({} as Parameters<typeof load>[0])).rejects.toMatchObject({
			status: 303,
			location: '/practices/practice-1'
		});
	});

	it('hands back a Practice picker for a Staff visitor with several memberships and no last-used one', async () => {
		const { load } = await import('./+page.js');
		const memberships = [
			{ practiceId: 'practice-1', practiceName: 'Riverside Doulas', roles: ['owner'] },
			{ practiceId: 'practice-2', practiceName: 'Hilltop Doulas', roles: ['doula'] }
		];
		setup({
			'/api/staff/session': { status: 200, body: { memberships, lastPracticeId: undefined } }
		});

		const result = await load({} as Parameters<typeof load>[0]);

		expect(result).toEqual({ type: 'staff-picker', memberships });
	});

	// #745: a Staff member whose last Membership was removed holds a
	// session that resolves to a person and to no Practice. That is a
	// state with two ways out of it, not a picker with nothing in it.
	it('sends a Staff visitor with no Membership to the no-Practice screen', async () => {
		const { load } = await import('./+page.js');
		setup({
			'/api/staff/session': { status: 200, body: { memberships: [], lastPracticeId: undefined } }
		});

		await expect(load({} as Parameters<typeof load>[0])).rejects.toMatchObject({
			status: 303,
			location: '/no-practice'
		});
	});

	// The other half of #745: a live credential that resolves to no staff
	// row at all -- a signup whose second half failed. It is not "signed
	// out", and offering her the sign-up and log-in links is offering the
	// two things that have already stopped working for her.
	it('sends a signed-in identity that belongs to neither population to the no-Practice screen', async () => {
		const { load } = await import('./+page.js');
		setup({
			'/api/staff/session': { status: 404, body: 'no matching staff account' },
			'/api/portal/session': { status: 404, body: 'no matching client account' }
		});

		await expect(load({} as Parameters<typeof load>[0])).rejects.toMatchObject({
			status: 303,
			location: '/no-practice'
		});
	});

	it('redirects a signed-in Client-portal visitor to her only Engagement, once no Staff session answers', async () => {
		const { load } = await import('./+page.js');
		setup({
			'/api/staff/session': { status: 401, body: 'no session' },
			'/api/portal/session': {
				status: 200,
				body: { engagements: [{ engagementId: 'engagement-1', practiceName: 'Riverside Doulas', status: 'active' }] }
			}
		});

		await expect(load({} as Parameters<typeof load>[0])).rejects.toMatchObject({
			status: 303,
			location: '/portal/engagements/engagement-1'
		});
	});

	it('hands back an Engagement picker for a Client-portal visitor with several Engagements', async () => {
		const { load } = await import('./+page.js');
		const engagements = [
			{ engagementId: 'engagement-1', practiceName: 'Riverside Doulas', status: 'active' },
			{ engagementId: 'engagement-2', practiceName: 'Hilltop Doulas', status: 'active' }
		];
		setup({
			'/api/staff/session': { status: 401, body: 'no session' },
			'/api/portal/session': { status: 200, body: { engagements } }
		});

		const result = await load({} as Parameters<typeof load>[0]);

		expect(result).toEqual({ type: 'portal-picker', engagements });
	});

	it('lands a visitor with neither session on the signed-out page, without treating either 401 as an expired session', async () => {
		const { load } = await import('./+page.js');
		setup({
			'/api/staff/session': { status: 401, body: 'no session' },
			'/api/portal/session': { status: 401, body: 'no session' }
		});

		const result = await load({} as Parameters<typeof load>[0]);

		expect(result).toEqual({ type: 'signed-out' });
		expect(goto).not.toHaveBeenCalled();
	});

	it('fails toward the signed-out landing when a probe throws, rather than a broken page', async () => {
		const { load } = await import('./+page.js');
		const fetchMock = vi.fn(async () => {
			throw new TypeError('Failed to fetch');
		});
		vi.stubGlobal('fetch', fetchMock);

		const result = await load({} as Parameters<typeof load>[0]);

		expect(result).toEqual({ type: 'signed-out' });
	});
});
