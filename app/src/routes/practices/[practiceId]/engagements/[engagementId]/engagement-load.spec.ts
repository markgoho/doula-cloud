import { afterEach, describe, expect, it, vi } from 'vitest';
import { jsonResponse } from '#lib/testResponse.js';

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

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
	it('fetches the Engagement and returns it', async () => {
		const detail = {
			engagementId: 'engagement-1',
			clientId: 'client-1',
			clientName: 'Tasha Bell',
			status: 'active',
			createdAt: '2027-05-01T00:00:00Z'
		};
		const { fetchMock } = setup(200, detail);

		expect(await callLoad()).toEqual(detail);
		expect(fetchMock).toHaveBeenCalledWith(
			'/api/practices/practice-1/engagements/engagement-1',
			expect.objectContaining({ credentials: 'include' })
		);
	});

	it('redirects to login on a 401, rather than reaching for goto mid-load', async () => {
		setup(401, 'no session');

		await expect(callLoad()).rejects.toMatchObject({
			status: 303,
			location: '/login?sessionEnded=true'
		});
	});

	// The reason this read moved into load at all: a Doula who is not
	// attached to this Engagement gets a 403 (ADR-0008, #350), and that
	// belongs in practices/+error.svelte rather than in a local error
	// string the page owns.
	it('throws a 403 SvelteKit error on a refusal, for practices/+error.svelte to render', async () => {
		setup(403, 'not permitted to read this');

		await expect(callLoad()).rejects.toMatchObject({
			status: 403,
			body: { message: 'not permitted to read this' }
		});
	});

	it('throws the endpoint status and message on any other failure', async () => {
		setup(500, 'internal error');

		await expect(callLoad()).rejects.toMatchObject({
			status: 500,
			body: { message: 'internal error' }
		});
	});
});
