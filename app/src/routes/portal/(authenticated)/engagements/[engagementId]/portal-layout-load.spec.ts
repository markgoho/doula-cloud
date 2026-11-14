import { afterEach, describe, expect, it, vi } from 'vitest';
import { jsonResponse } from '#lib/testResponse.js';
import { isEngagementReadNew } from '#lib/portalEngagementRead.js';
import { loadEvent } from '#testing/loadEvent.js';
import { load } from './+layout.js';

const body = {
	engagementId: 'engagement-1',
	practiceName: 'Riverside Doulas',
	practiceTimezone: 'America/New_York',
	clientName: 'Tasha Bell',
	createdAt: '2026-03-12T20:00:00Z',
	offersBirthPlan: true
};

function callLoad(loadFetch: typeof fetch) {
	return load(loadEvent(load, {
		params: { engagementId: 'engagement-1' },
		url: new URL('https://example.test/portal/engagements/engagement-1'),
		depends: vi.fn(),
		fetch: loadFetch
	}));
}

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('portal engagements/[engagementId]/+layout.ts load (#2587)', () => {
	it("reads the Engagement once, through the fetch that load is passed, not the window's", async () => {
		const windowFetch = vi.fn();
		vi.stubGlobal('fetch', windowFetch);
		const loadFetch = vi.fn<typeof fetch>(async () => jsonResponse(body, 200));

		await expect(callLoad(loadFetch)).resolves.toEqual(body);

		expect(loadFetch).toHaveBeenCalledTimes(1);
		expect(loadFetch).toHaveBeenCalledWith('/api/portal/engagements/engagement-1', expect.anything());
		expect(windowFetch).not.toHaveBeenCalled();
	});

	it('counts as a read the hub need not repeat', async () => {
		isEngagementReadNew();
		await callLoad(vi.fn<typeof fetch>(async () => jsonResponse(body, 200)));

		expect(isEngagementReadNew()).toBe(true);
		expect(isEngagementReadNew()).toBe(false);
	});

	it('sends a signed-out Client to sign-in without navigating mid-load', async () => {
		await expect(
			callLoad(vi.fn<typeof fetch>(async () => jsonResponse({}, 401)))
		).rejects.toMatchObject({ status: 303 });
	});

	it('refuses the route with the read status for any other failure', async () => {
		await expect(
			callLoad(vi.fn<typeof fetch>(async () => jsonResponse({ message: 'gone' }, 404)))
		).rejects.toMatchObject({ status: 404 });
	});
});
