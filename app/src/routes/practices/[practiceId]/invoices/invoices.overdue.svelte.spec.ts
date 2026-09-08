/*
 * #768's screen: the Practice-wide list reports which Invoices are past
 * their due date and by how long, and can be narrowed to them.
 *
 * Its own file rather than more of `invoices.svelte.spec.ts`, which
 * mocks `$app/state` at module scope for the unnarrowed URL; the
 * narrowed state is a different page state, and one mock cannot be two.
 */
import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import type { PracticeInvoiceListData } from '#lib/invoice.js';
import '#lib/styles/app.css';
import Page from './+page.svelte';
import { toPageState } from '../../../routeFixture.js';
import { data, fixture } from './page.fixture.js';

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

const sessionStub = {
	practiceId,
	staffId: 'staff-1',
	practiceName: 'Riverside Doula Collective',
	roles: ['owner'],
	isContractor: false
};

beforeEach(() => {
	apiFetchWithSession.mockReset();
});

async function setup(page: PracticeInvoiceListData = data) {
	// Wide enough for DataTable's <table> rather than the record view its
	// content floor stacks into below 46rem -- the same call the sibling
	// spec makes, for the same reason.
	await testPage.viewport(1440, 900);
	await render(Page, { params: fixture.params, data: { ...page, session: sessionStub } });
}

describe('the overdue book (#768)', () => {
	it('says how much of the outstanding money is late, beside the outstanding figure itself', async () => {
		await setup();

		// $4,500.00 outstanding, all of it overdue -- the fixture's first
		// row is open and past its due date.
		await expect.element(testPage.getByText('Overdue invoices')).toBeVisible();
		await expect.element(testPage.getByText('Overdue', { exact: true })).toBeVisible();
	});

	it('says how late an unpaid invoice is, in the row itself', async () => {
		await setup();

		// The cell, not the whole row: DataTable renders both a table cell
		// and a record-view line for every value, and only one of them is
		// on screen at this width.
		await expect.element(testPage.getByRole('cell', { name: /days overdue/ })).toBeVisible();
	});

	it('offers the narrowing as a link, carrying the practice overdue count', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('link', { name: `Overdue (${data.overdueCount})` }))
			.toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'All invoices' })).toBeVisible();
	});

	it('asks every later page for the same narrowing it was read under', async () => {
		apiFetchWithSession.mockResolvedValue(
			jsonResponse({ ...data, items: [], hasMore: false, nextCursor: undefined })
		);

		await setup({ ...data, isNarrowedToOverdue: true, hasMore: true, nextCursor: 'cursor-1' });
		await testPage.getByRole('button', { name: 'Load more' }).click();

		expect(apiFetchWithSession).toHaveBeenCalledWith(
			`/api/practices/${practiceId}/invoices?overdue=true&cursor=cursor-1`
		);
	});

	it('says nothing is late rather than "no invoices yet" when the narrowed list is empty', async () => {
		await setup({ ...data, items: [], isNarrowedToOverdue: true, overdueCents: 0, overdueCount: 0 });

		await expect.element(testPage.getByRole('cell', { name: /Nothing is overdue/ })).toBeVisible();
	});
});
