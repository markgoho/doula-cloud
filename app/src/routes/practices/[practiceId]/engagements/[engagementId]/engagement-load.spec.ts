import { afterEach, describe, expect, it, vi } from 'vitest';
import { jsonResponse } from '#lib/testResponse.js';
import { loadedValue, settledRead } from '#lib/testStreamedRead.js';

function setup(status: number, body: unknown) {
	const fetchMock = vi.fn(async () => jsonResponse(body, status));
	vi.stubGlobal('fetch', fetchMock);
	return { fetchMock };
}

const parameters = { practiceId: 'practice-1', engagementId: 'engagement-1' };

async function callLoad() {
	const { load } = await import('./+page.js');
	return load({ params: parameters } as Parameters<typeof load>[0]);
}

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('engagements/[engagementId]/+page.ts load', () => {
	it('fetches the Engagement and hands it over', async () => {
		const detail = {
			engagementId: 'engagement-1',
			clientId: 'client-1',
			clientName: 'Tasha Bell',
			status: 'active',
			kind: 'birth',
			createdAt: '2027-05-01T00:00:00Z'
		};
		const { fetchMock } = setup(200, detail);

		await expect(loadedValue(await callLoad())).resolves.toEqual(detail);
		expect(fetchMock).toHaveBeenCalledWith(
			'/api/practices/practice-1/engagements/engagement-1',
			expect.objectContaining({ credentials: 'include' })
		);
	});

	// A Doula who is not attached to this Engagement gets a 403 (ADR-0008,
	// #350). The screen has changed by the time it lands (#2249), so the
	// screen draws it; the read carries the response through unread.
	it('hands a refusal to the screen rather than throwing mid-load', async () => {
		setup(403, { code: 'FORBIDDEN', message: 'not permitted to read this' });

		await expect(settledRead(await callLoad())).resolves.toMatchObject({
			isLoaded: false,
			response: { status: 403 }
		});
	});
});
