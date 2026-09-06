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

const loadArguments = {
	params: { practiceId: 'practice-1' },
	url: new URL('https://example.test/practices/practice-1/billing')
} as unknown as Parameters<typeof import('./+layout.js').load>[0];

afterEach(() => {
	vi.unstubAllGlobals();
});

// #835: this `load` runs one practice_memberships-backed query per
// navigation and hands the result down as `page.data.session` -- every
// descendant route reads that instead of fetching this endpoint itself.
describe('practices/[practiceId]/+layout.ts load', () => {
	it("fetches the Practice session and returns it as this navigation's Membership", async () => {
		const { load } = await import('./+layout.js');
		setup({
			'/api/practices/practice-1/session': {
				status: 200,
				body: { practiceName: 'Riverside Doula Collective', roles: ['owner'], isContractor: false }
			}
		});

		const result = await load(loadArguments);

		expect(result).toEqual({
			session: {
				practiceId: 'practice-1',
				practiceName: 'Riverside Doula Collective',
				roles: ['owner'],
				isContractor: false
			}
		});
	});

	it('redirects to login on a 401, rather than reaching for goto mid-load', async () => {
		const { load } = await import('./+layout.js');
		setup({ '/api/practices/practice-1/session': { status: 401, body: 'no session' } });

		await expect(load(loadArguments)).rejects.toMatchObject({ status: 303, location: '/login?sessionEnded=true' });
	});

	// #606: a live session barred from *this* Practice only -- not a
	// stale Membership, so it must not fall into #748's decideLanding
	// branch below. A real `Response.json`, not the `jsonResponse` fake:
	// `isMFARequired` reads a `.clone()` of the body, which the fake
	// (api.spec.ts's own `isMFARequired` tests use the same real
	// `Response`, for the same reason) does not implement.
	it('redirects to MFA enrolment on a 403 carrying MFA_REQUIRED, carrying returnTo', async () => {
		const { load } = await import('./+layout.js');
		vi.stubGlobal(
			'fetch',
			vi.fn(async () =>
				Response.json({ code: 'MFA_REQUIRED', message: 'this Practice requires a second sign-in factor' }, { status: 403 })
			)
		);

		await expect(load(loadArguments)).rejects.toMatchObject({
			status: 303,
			location: '/mfa/enroll?returnTo=%2Fpractices%2Fpractice-1%2Fbilling'
		});
	});

	// #748 AC1: removed from every Practice mid-session.
	it('sends her to /no-practice when an ordinary 403 leaves no other Membership standing', async () => {
		const { load } = await import('./+layout.js');
		setup({
			'/api/practices/practice-1/session': { status: 403, body: 'no membership at this practice' },
			'/api/staff/session': { status: 200, body: { memberships: [], lastPracticeId: undefined } }
		});

		await expect(load(loadArguments)).rejects.toMatchObject({ status: 303, location: '/no-practice' });
	});

	// #748 AC2: removed from this Practice, but another Membership still
	// stands -- she lands somewhere she can still work, not /no-practice.
	it('redirects to her one remaining Practice when a 404 leaves exactly one Membership standing', async () => {
		const { load } = await import('./+layout.js');
		setup({
			'/api/practices/practice-1/session': { status: 404, body: 'no matching staff account' },
			'/api/staff/session': {
				status: 200,
				body: { memberships: [{ practiceId: 'practice-2', practiceName: 'Hilltop Doulas', roles: ['doula'] }] }
			}
		});

		await expect(load(loadArguments)).rejects.toMatchObject({ status: 303, location: '/practices/practice-2' });
	});

	it('sends her to the picker at / when several other Memberships remain and none is last-used', async () => {
		const { load } = await import('./+layout.js');
		setup({
			'/api/practices/practice-1/session': { status: 403, body: 'no membership at this practice' },
			'/api/staff/session': {
				status: 200,
				body: {
					memberships: [
						{ practiceId: 'practice-2', practiceName: 'Hilltop Doulas', roles: ['doula'] },
						{ practiceId: 'practice-3', practiceName: 'Finger Lakes Birth Support', roles: ['owner'] }
					],
					lastPracticeId: undefined
				}
			}
		});

		await expect(load(loadArguments)).rejects.toMatchObject({ status: 303, location: '/' });
	});

	it("sends her to /no-practice when her Staff row itself is gone -- there is no session left to decide from", async () => {
		const { load } = await import('./+layout.js');
		setup({
			'/api/practices/practice-1/session': { status: 404, body: 'no matching staff account' },
			'/api/staff/session': { status: 404, body: 'no matching staff account' }
		});

		await expect(load(loadArguments)).rejects.toMatchObject({ status: 303, location: '/no-practice' });
	});

	it('redirects to login if the fallback staff-session read itself finds the session ended', async () => {
		const { load } = await import('./+layout.js');
		setup({
			'/api/practices/practice-1/session': { status: 403, body: 'no membership at this practice' },
			'/api/staff/session': { status: 401, body: 'no session' }
		});

		await expect(load(loadArguments)).rejects.toMatchObject({ status: 303, location: '/login?sessionEnded=true' });
	});

	it('throws with the response status on any other failure', async () => {
		const { load } = await import('./+layout.js');
		setup({ '/api/practices/practice-1/session': { status: 500, body: 'boom' } });

		await expect(load(loadArguments)).rejects.toMatchObject({ status: 500 });
	});
});
