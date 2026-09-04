import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import type { PendingRequestItem } from '#lib/engagementRequest.js';
// DataTable's frame needs stack-l's display:block default (primitives.css)
// to work as a container-query context -- see DataTable.svelte.spec.ts.
import '#lib/styles/app.css';
import Page from './+page.svelte';
import { toPageState } from '../../../routeFixture.js';
import { fixture, requests } from './page.fixture.js';

/*
 * The `page` this route reads comes from its own fixture (#596), so the
 * params this spec installs and the params the continuum sweep installs
 * are one description. `vi.mock` is hoisted above every import, so the
 * object is declared empty here and filled from the fixture once the
 * imports have run.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const { practiceId } = fixture.params;

// Both Requests come from the fixture (#596): the birth ask carries a
// due date, the postpartum one does not, which is the difference the
// "Not given" test below is about.
const [birthRequest, postpartumRequest] = requests;

beforeEach(async () => {
	apiFetchWithSession.mockReset();
	// DataTable's own content floor (#508) stacks it into a <dl> below
	// 46rem, and this file's assertions are about the <table> specifically.
	await testPage.viewport(1440, 900);
});

function mockPages(...pages: { items: PendingRequestItem[]; hasMore: boolean; nextCursor?: string }[]) {
	for (const body of pages) {
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse(body));
	}
}

describe('the pending-Request inbox', () => {
	it('names every Request waiting, and links each one to its decision', async () => {
		mockPages({ items: requests, hasMore: false });

		render(Page);

		const row = testPage.getByRole('link', { name: birthRequest.clientName });
		await expect.element(row).toBeVisible();
		await expect
			.element(row)
			.toHaveAttribute('href', `/practices/${practiceId}/engagement-requests/${birthRequest.requestId}`);
		await expect.element(testPage.getByRole('cell', { name: 'Birth' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: birthRequest.requestedByName })).toBeVisible();
		expect(apiFetchWithSession).toHaveBeenCalledWith(`/api/practices/${practiceId}/engagement-requests`);
	});

	// A postpartum ask carries no due date, and the cell says so rather
	// than going blank -- a blank cell reads as a fact that failed to load.
	it('says a Request named no due date rather than leaving the cell empty', async () => {
		mockPages({ items: [postpartumRequest], hasMore: false });

		render(Page);

		await expect.element(testPage.getByRole('cell', { name: 'Not given' })).toBeVisible();
	});

	it('says so when nothing is waiting', async () => {
		mockPages({ items: [], hasMore: false });

		render(Page);

		// getByRole, not getByText: the record view carries the same
		// message in a hidden <p>, and only a role query excludes it.
		await expect
			.element(testPage.getByRole('cell', { name: 'No requests are waiting for a decision.' }))
			.toBeVisible();
	});

	it('appends the next page rather than replacing what is on screen', async () => {
		mockPages(
			{ items: [birthRequest], hasMore: true, nextCursor: 'cursor-1' },
			{ items: [postpartumRequest], hasMore: false }
		);

		render(Page);

		await testPage.getByRole('button', { name: 'Load more' }).click();

		// getByRole, not getByText: the record view links the same rows in
		// a hidden tree, and only a role query excludes it.
		await expect
			.element(testPage.getByRole('link', { name: postpartumRequest.clientName }))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: birthRequest.clientName }))
			.toBeVisible();
		expect(apiFetchWithSession).toHaveBeenLastCalledWith(
			`/api/practices/${practiceId}/engagement-requests?cursor=cursor-1`
		);
	});

	it('reports a refusal instead of an empty table', async () => {
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse('forbidden', 403));

		render(Page);

		await expect.element(testPage.getByRole('alert')).toHaveTextContent('forbidden');
	});

	it('reports a failure to load the next page without losing the first', async () => {
		mockPages({ items: [birthRequest], hasMore: true, nextCursor: 'cursor-1' });
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse('service problem', 500));

		render(Page);

		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect.element(testPage.getByRole('alert')).toHaveTextContent('service problem');
	});
});
