import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import { jsonResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';
import '#lib/styles/app.css';
import { toApiResponder, toPageState } from '../../../routeFixture.js';
import { fixture, items } from './page.fixture.js';

if (!customElements.get('center-l')) registerLayoutPrimitives();

const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

async function setup(respond: (path: string) => Response | Promise<Response> = toApiResponder(fixture)) {
	apiFetchWithSession.mockImplementation(respond);
	await render(Page, {});
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
});

describe("The Practice's whole activity ledger (#2006)", () => {
	it('shows every row of the first page, in the words of each row', async () => {
		// DataTable draws its table, with `cell` roles, only past its own
		// 55rem floor; narrower it draws records.
		await testPage.viewport(1440, 900);
		await setup();

		await expect.element(testPage.getByRole('heading', { level: 1, name: 'Activity' })).toBeVisible();
		await expect
			.element(testPage.getByRole('cell', { name: 'Contract price for Persephone Adeyemi-Wollstonecraft changed to the new rate' }))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('cell', { name: 'Employment type changed for Anne-Marie Ochieng-Whitfield' }))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('cell', { name: 'Two-factor authentication no longer a must for all Staff' }))
			.toBeVisible();
	});

	it('loads the next page when asked', async () => {
		await testPage.viewport(1440, 900);
		const next = { ...items[0]!, subjectId: 'engagement-2', subjectName: 'Camille', action: 'visit_logged' };
		const nextPage = { items: [next], hasMore: false };
		const firstPage = { items, hasMore: true, nextCursor: 'cursor-1' };
		await setup((path) => jsonResponse(path.includes('cursor=') ? nextPage : firstPage));

		await testPage.getByRole('button', { name: /more/i }).click();

		await expect.element(testPage.getByRole('cell', { name: 'Visit added for Camille' })).toBeVisible();
		expect(apiFetchWithSession).toHaveBeenCalledWith('/api/practices/practice-1/activity?cursor=cursor-1');
	});

	it('says so when the ledger cannot be read', async () => {
		await setup(() => jsonResponse('not permitted to read this', 403));

		await expect.element(testPage.getByText('not permitted to read this')).toBeVisible();
	});
});
