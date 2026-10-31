import { describe, expect, it, vi, afterEach } from 'vitest';
import { jsonResponse } from '#lib/testResponse.js';
import { loadedValue, settledRead } from '#lib/testStreamedRead.js';
import { load } from './+page.js';

function setup(status: number, body: unknown) {
	const fetchMock = vi.fn(async () => jsonResponse(body, status));
	vi.stubGlobal('fetch', fetchMock);
	return { fetchMock };
}

afterEach(() => {
	vi.unstubAllGlobals();
});

const emptyPage = { items: [], hasMore: false };

describe('contracts/+page.ts load', () => {
	it('fetches both Practice-wide roll-up paths and hands over both pages', async () => {
		const { fetchMock } = setup(200, emptyPage);

		const data = await load({ params: { practiceId: 'practice-1' } } as Parameters<typeof load>[0]);

		expect(fetchMock).toHaveBeenCalledWith(
			'/api/practices/practice-1/contracts/awaiting-signature',
			expect.objectContaining({ credentials: 'include' })
		);
		expect(fetchMock).toHaveBeenCalledWith(
			'/api/practices/practice-1/contracts/void-requests',
			expect.objectContaining({ credentials: 'include' })
		);
		await expect(loadedValue(data)).resolves.toEqual({ contracts: emptyPage, voidRequests: emptyPage });
	});

	// #2249: the screen has changed by the time a refusal lands, so the
	// screen follows it; the read carries the response through unread.
	it('hands a refusal to the screen rather than throwing mid-load', async () => {
		setup(403, { code: 'FORBIDDEN', message: 'not permitted to read this' });

		const data = await load({ params: { practiceId: 'practice-1' } } as Parameters<typeof load>[0]);

		await expect(settledRead(data)).resolves.toMatchObject({ isLoaded: false, response: { status: 403 } });
	});
});
