import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { formatSentAt, type FeedbackSummary } from '#lib/founderFeedback.js';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import { jsonResponse } from '#lib/testResponse.js';
// DataTable's frame needs the layout primitives' CSS to be a
// container-query context, and ListPage needs them registered -- see
// invoices.svelte.spec.ts.
import '#lib/styles/app.css';
import Page from './+page.svelte';
import type { FeedbackListData } from './+page.js';
import { toPageState } from '../../routeFixture.js';
import { data, deadLettered, empty, fixture, opened, retrying } from './page.fixture.js';

const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
if (!customElements.get('center-l')) registerLayoutPrimitives();
Object.assign(pageState, toPageState(fixture));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

beforeEach(() => {
	apiFetchWithSession.mockReset();
});

async function setup(list: FeedbackListData = data) {
	// Wide enough for DataTable's <table> rather than the record view it
	// stacks into in a narrow container (#508). What the lists say is
	// asserted here; that they say it at 320px is the continuum sweep's job.
	await testPage.viewport(1440, 900);
	await render(Page, { params: {}, data: list });
}

const older: FeedbackSummary = {
	id: 'feedback-9',
	kind: 'something_else',
	routeId: '/(app)/practices/[practiceId]/schedule',
	sentAt: '2026-09-01T09:00:00Z',
	issue: { state: 'pending', attempts: 0 }
};

describe('the founder feedback list (#1526)', () => {
	it('lists the pieces whose issue did not open first, under their own heading', async () => {
		await setup();

		const headings = testPage.getByRole('heading', { level: 2 });
		await expect.element(headings.nth(0)).toHaveTextContent('Issue not opened');
		await expect.element(headings.nth(1)).toHaveTextContent('All feedback');

		// The first table is the only one with the job's own error on it.
		const unopened = testPage.getByRole('table').nth(0);
		await expect.element(unopened.getByRole('columnheader', { name: 'Last error' })).toBeVisible();
		await expect.element(unopened.getByRole('cell', { name: 'Not opened: all 5 attempts failed' })).toBeVisible();
		await expect.element(unopened.getByRole('cell', { name: deadLettered.issue.lastError })).toBeVisible();
		await expect.element(unopened.getByRole('cell', { name: 'Not opened yet: 2 attempts failed' })).toBeVisible();
		await expect.element(unopened.getByRole('link', { name: formatSentAt(opened.sentAt) })).not.toBeInTheDocument();
	});

	it('lists every piece with its kind, route pattern and issue, and no free text', async () => {
		await setup();

		const all = testPage.getByRole('table').nth(1);
		await expect.element(all.getByRole('cell', { name: 'An idea or a request' })).toBeVisible();
		await expect.element(all.getByRole('cell', { name: opened.routeId })).toBeVisible();
		await expect.element(all.getByRole('cell', { name: 'Issue #12' })).toBeVisible();
		await expect.element(all.getByRole('cell', { name: 'Waiting to open' })).toBeVisible();
	});

	it('names each piece by when it was sent, as the way in to it', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('link', { name: formatSentAt(opened.sentAt) }))
			.toHaveAttribute('href', '/feedback/feedback-1');
	});

	/*
	 * The app preloads a link's data on hover (app.html), and loading a
	 * piece is the request the BFF records as a read. A hover is not a
	 * read, so every row link in both tables has to sit under an opt-out.
	 *
	 * `closest` on a data attribute: the querySelector exception for a
	 * fact with no accessible signal. Preloading is plumbing SvelteKit
	 * reads off the DOM; no role or name carries it.
	 */
	it('never preloads a piece on hover, because loading one is what records the read', async () => {
		await setup();

		const links = testPage.getByRole('link').elements();
		expect(links).toHaveLength(6);
		for (const link of links) {
			expect(link.closest<HTMLElement>('[data-sveltekit-preload-data]')?.dataset.sveltekitPreloadData).toBe('off');
		}
	});

	it('says so in both tables when nothing has been sent', async () => {
		await setup(empty.props!.data as FeedbackListData);

		await expect
			.element(testPage.getByRole('cell', { name: 'No attempt to open an issue has failed.' }))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('cell', { name: 'No feedback yet. A piece appears here as soon as somebody sends one.' }))
			.toBeVisible();
	});

	it('appends the next page of the whole list rather than replacing the one already read', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ items: [older], hasMore: false }));

		await setup();
		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect.element(testPage.getByRole('link', { name: formatSentAt(older.sentAt) })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: formatSentAt(opened.sentAt) })).toBeVisible();
		expect(apiFetchWithSession).toHaveBeenCalledWith('/api/staff/feedback?cursor=cursor-1');
	});

	it('asks for the next page of the unopened list with the same narrowing', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ items: [], hasMore: false }));

		await setup({
			unopened: { items: [deadLettered, retrying], hasMore: true, nextCursor: 'cursor-2' },
			all: { items: [deadLettered, retrying], hasMore: false }
		});
		await testPage.getByRole('button', { name: 'Load more' }).click();

		expect(apiFetchWithSession).toHaveBeenCalledWith('/api/staff/feedback?issue=unopened&cursor=cursor-2');
	});

	it('reports a failed next page in place, for whichever list it was', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse('invalid cursor', 400));

		await setup({
			unopened: { items: [deadLettered], hasMore: true, nextCursor: 'cursor-2' },
			all: { items: [deadLettered, opened], hasMore: true, nextCursor: 'cursor-1' }
		});
		await testPage.getByRole('button', { name: 'Load more' }).nth(0).click();
		await expect.element(testPage.getByRole('alert')).toHaveTextContent('invalid cursor');

		await testPage.getByRole('button', { name: 'Load more' }).nth(1).click();
		await expect.element(testPage.getByRole('alert').nth(1)).toHaveTextContent('invalid cursor');
		await expect.element(testPage.getByRole('link', { name: formatSentAt(opened.sentAt) })).toBeVisible();
	});
});
