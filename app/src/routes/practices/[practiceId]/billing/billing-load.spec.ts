import { describe, expect, it, vi, afterEach } from 'vitest';
import { jsonResponse } from '#lib/testResponse.js';

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

function setup(status: number, body: unknown) {
	const fetchMock = vi.fn(async () => jsonResponse(body, status));
	vi.stubGlobal('fetch', fetchMock);
	return { fetchMock };
}

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('billing/+page.ts load', () => {
	it('fetches the practice billing path and returns the balance', async () => {
		const { load } = await import('./+page.js');
		const balance = { balance: 8, ledger: { items: [], hasMore: false } };
		const { fetchMock } = setup(200, balance);

		const result = await load({ params: { practiceId: 'practice-1' } } as Parameters<typeof load>[0]);

		expect(fetchMock).toHaveBeenCalledWith('/api/practices/practice-1/billing', expect.objectContaining({ credentials: 'include' }));
		expect(result).toEqual(balance);
	});

	it('redirects to login on a 401, rather than reaching for goto mid-load', async () => {
		const { load } = await import('./+page.js');
		setup(401, 'no session');

		await expect(load({ params: { practiceId: 'practice-1' } } as Parameters<typeof load>[0])).rejects.toMatchObject({
			status: 303,
			location: '/login?sessionEnded=true'
		});
	});

	it('throws a 403 SvelteKit error on a role refusal, for practices/+error.svelte to render', async () => {
		const { load } = await import('./+page.js');
		setup(403, 'not permitted to read this');

		await expect(load({ params: { practiceId: 'practice-1' } } as Parameters<typeof load>[0])).rejects.toMatchObject({
			status: 403
		});
	});

	// #918: the refusal reason survives the trip from the BFF to
	// practices/+error.svelte. Before it did, this load flattened every
	// 403 into one sentence and the error page had only the status left
	// to guess a cause from.
	it('carries the refusal reason through, rather than flattening every 403 into one sentence', async () => {
		const { load } = await import('./+page.js');
		setup(403, { code: 'MFA_REQUIRED', message: 'this Practice requires a second sign-in factor' });

		await expect(load({ params: { practiceId: 'practice-1' } } as Parameters<typeof load>[0])).rejects.toMatchObject({
			status: 403,
			body: { code: 'MFA_REQUIRED' }
		});
	});

	it('throws with the response status on any other failure', async () => {
		const { load } = await import('./+page.js');
		setup(500, 'boom');

		await expect(load({ params: { practiceId: 'practice-1' } } as Parameters<typeof load>[0])).rejects.toMatchObject({
			status: 500
		});
	});
});
