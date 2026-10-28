import { describe, expect, it, vi, afterEach } from 'vitest';
import { jsonResponse } from '#lib/testResponse.js';
import { loadedValue, settledRead } from '#lib/testStreamedRead.js';

function setup(status: number, body: unknown) {
	const fetchMock = vi.fn(async () => jsonResponse(body, status));
	vi.stubGlobal('fetch', fetchMock);
	return { fetchMock };
}

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('billing/+page.ts load', () => {
	it('fetches the practice billing path and hands over the balance', async () => {
		const { load } = await import('./+page.js');
		const balance = { balance: 8, ledger: { items: [], hasMore: false } };
		const { fetchMock } = setup(200, balance);

		const data = await load({ params: { practiceId: 'practice-1' } } as Parameters<typeof load>[0]);

		expect(fetchMock).toHaveBeenCalledWith('/api/practices/practice-1/billing', expect.objectContaining({ credentials: 'include' }));
		await expect(loadedValue(data)).resolves.toEqual(balance);
	});

	// #2249: the screen has changed by the time a refusal lands, so the
	// screen follows it; the read carries the response through unread.
	it('hands a refusal to the screen rather than throwing mid-load', async () => {
		const { load } = await import('./+page.js');
		setup(403, { code: 'FORBIDDEN', message: 'not permitted to read this' });

		const data = await load({ params: { practiceId: 'practice-1' } } as Parameters<typeof load>[0]);

		await expect(settledRead(data)).resolves.toMatchObject({ isLoaded: false, response: { status: 403 } });
	});
});
