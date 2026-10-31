import { afterEach, describe, expect, it, vi } from 'vitest';
import { jsonResponse } from '#lib/testResponse.js';
import type { ComingVersion } from '#lib/legal.js';
import { load as pageLoad } from './+page.js';

const coming: ComingVersion[] = [{ document: 'terms', name: 'Terms of Service', version: '2026-11-30' }];

function setup(answer: (input: RequestInfo | URL) => Promise<Response>) {
	const fetchMock = vi.fn(answer);
	vi.stubGlobal('fetch', fetchMock);
	return { fetchMock };
}

async function load() {
	return pageLoad({} as Parameters<typeof pageLoad>[0]);
}

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('/signup +page.ts load (#1824)', () => {
	it('hands the form the coming versions the BFF names', async () => {
		const { fetchMock } = setup(async () => jsonResponse({ coming }));

		await expect(load()).resolves.toEqual({ coming });
		expect(String(fetchMock.mock.calls[0]?.[0])).toMatch(/\/api\/legal\/coming$/);
	});

	/*
	 * The form must name what the press records, so it is not drawn
	 * without the read: a BFF that cannot answer it cannot take the
	 * signup either, and the page says the service is unavailable.
	 */
	it('refuses the page as unavailable when the BFF does not answer', async () => {
		setup(async () => jsonResponse('down', 502));

		await expect(load()).rejects.toMatchObject({ status: 503 });
	});

	it('refuses the page as unavailable when the request does not leave the browser', async () => {
		setup(async () => {
			throw new TypeError('Failed to fetch');
		});

		await expect(load()).rejects.toMatchObject({ status: 503 });
	});
});
