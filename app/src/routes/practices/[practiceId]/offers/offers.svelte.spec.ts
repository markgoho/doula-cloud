/*
 * A Staff member's own Offers inbox, by the state of its one read: a
 * failed first read is said on its own, not over an empty inbox, and a
 * failed re-read after an answer leaves the inbox drawn.
 */
import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';
import { toApiResponder, toPageState } from '../../../routeFixture.js';
import { fixture } from './page.fixture.js';
import { pageState } from '#testing/routePage.js';

vi.mock('$app/state', () => import('#testing/routePage.js'));
Object.assign(pageState, toPageState(fixture));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const accept = () => testPage.getByRole('button', { name: 'Accept' }).first();

beforeEach(() => {
	apiFetchWithSession.mockReset();
});

describe("a Staff member's own Offers inbox", () => {
	it('draws the inbox once its read lands', async () => {
		apiFetchWithSession.mockImplementation(toApiResponder(fixture));
		await render(Page, {});

		await expect.element(accept()).toBeVisible();
	});

	it('says a failed first read on its own, with no inbox under it', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ message: 'The inbox is down' }, 503));
		await render(Page, {});

		await expect.element(testPage.getByRole('alert')).toHaveTextContent('The inbox is down');
		expect(accept().query()).toBeNull();
	});

	it('keeps the inbox drawn when the re-read after an answer fails', async () => {
		let reads = 0;
		apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
			if (init?.method === 'POST') {
				return Promise.resolve(
					jsonResponse({ offerId: 'offer-1', state: 'accepted', engagementId: 'engagement-1', engagementKind: 'birth', clientName: 'P' })
				);
			}
			reads += 1;
			return reads === 1 ? toApiResponder(fixture)(path) : Promise.resolve(jsonResponse({ message: 'Down' }, 503));
		});
		await render(Page, {});

		await accept().click();

		await expect.element(testPage.getByText('Down')).toBeVisible();
		await expect.element(testPage.getByRole('heading', { level: 1, name: 'Your offers' })).toBeVisible();
	});
});
