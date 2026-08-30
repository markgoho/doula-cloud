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
		const balance = { balance: 8, ledger: [] };
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

	it('throws with the response status on any other failure', async () => {
		const { load } = await import('./+page.js');
		setup(500, 'boom');

		await expect(load({ params: { practiceId: 'practice-1' } } as Parameters<typeof load>[0])).rejects.toMatchObject({
			status: 500
		});
	});
});
