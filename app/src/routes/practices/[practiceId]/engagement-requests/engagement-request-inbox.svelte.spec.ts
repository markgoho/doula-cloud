import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import type { PendingRequestItem } from '#lib/engagementRequest.js';
// DataTable's frame needs stack-l's display:block default (primitives.css)
// to work as a container-query context -- see DataTable.svelte.spec.ts.
import '#lib/styles/app.css';
import Page from './+page.svelte';

vi.mock('$app/state', () => ({ page: { params: { practiceId: 'practice-1' } } }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

// The longest realistic values, per ADR-0025: a double-barrelled surname
// and a full staff name, not a polite fixture.
const birthRequest: PendingRequestItem = {
	requestId: 'request-1',
	clientId: 'client-1',
	clientName: 'Marguerite Ashworth-Delacroix',
	kind: 'birth',
	dueDate: '2027-03-01',
	requestedByName: 'Tasha Bell-Okonkwo',
	requestedAt: '2026-08-01T10:00:00Z'
};

const postpartumRequest: PendingRequestItem = {
	requestId: 'request-2',
	clientId: 'client-2',
	clientName: 'Rosalind Fairweather',
	kind: 'postpartum',
	requestedByName: 'Ada Doula',
	requestedAt: '2026-08-02T10:00:00Z'
};

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
		mockPages({ items: [birthRequest, postpartumRequest], hasMore: false });

		render(Page);

		const row = testPage.getByRole('link', { name: 'Marguerite Ashworth-Delacroix' });
		await expect.element(row).toBeVisible();
		await expect
			.element(row)
			.toHaveAttribute('href', '/practices/practice-1/engagement-requests/request-1');
		await expect.element(testPage.getByRole('cell', { name: 'Birth' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Tasha Bell-Okonkwo' })).toBeVisible();
		expect(apiFetchWithSession).toHaveBeenCalledWith('/api/practices/practice-1/engagement-requests');
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
		await expect.element(testPage.getByRole('link', { name: 'Rosalind Fairweather' })).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Marguerite Ashworth-Delacroix' }))
			.toBeVisible();
		expect(apiFetchWithSession).toHaveBeenLastCalledWith(
			'/api/practices/practice-1/engagement-requests?cursor=cursor-1'
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
