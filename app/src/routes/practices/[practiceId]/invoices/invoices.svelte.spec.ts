import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import type { PracticeInvoicePage } from '#lib/invoice.js';
// DataTable's frame needs stack-l's display:block default (primitives.css)
// to work as a container-query context -- see DataTable.svelte.spec.ts.
import '#lib/styles/app.css';
import Page from './+page.svelte';

vi.mock('$app/state', () => ({
	page: { params: { practiceId: 'practice-1' }, url: new URL('https://example.test/invoices') }
}));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const data: PracticeInvoicePage = {
	items: [
		{
			id: 'inv-1',
			engagementId: 'eng-1',
			contractId: 'contract-1',
			clientName: 'Ada',
			status: 'open',
			amountCents: 150_000,
			currency: 'usd',
			createdAt: '2026-08-01T00:00:00Z'
		},
		{
			id: 'inv-2',
			engagementId: 'eng-2',
			contractId: 'contract-2',
			clientName: 'Bea',
			status: 'paid',
			amountCents: 250_000,
			currency: 'usd',
			createdAt: '2026-07-01T00:00:00Z',
			paidAt: '2026-07-04T00:00:00Z'
		}
	],
	hasMore: false,
	outstandingCents: 150_000,
	outstandingCount: 1,
	paidCents: 250_000
};

beforeEach(() => {
	apiFetchWithSession.mockReset();
});

async function setup(page: PracticeInvoicePage = data) {
	// Wide enough for DataTable's <table> rather than the <dl> record view
	// its content floor stacks into below 46rem (#508) -- the same call the
	// Billing ledger's spec makes for the same reason. What the list says
	// is asserted here; that it says the same thing in a narrow container
	// is DataTable's own spec's job.
	await testPage.viewport(1440, 900);
	await render(Page, { params: { practiceId: 'practice-1' }, data: page });
}

describe('the Practice-wide invoice list (#265)', () => {
	it('answers "who owes us money" with the whole book, not one Engagement', async () => {
		await setup();

		await expect.element(testPage.getByRole('heading', { name: 'Invoices' })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Ada' })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Bea' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: '$1,500.00' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Open' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Paid', exact: true })).toBeVisible();
	});

	it('names each Client as the way in to her Engagement', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('link', { name: 'Ada' }))
			.toHaveAttribute('href', '/practices/practice-1/engagements/eng-1');
	});

	it('shows what is outstanding across the Practice', async () => {
		await setup();

		// The summary is a description list, so its three values are read
		// positionally -- both figures also appear as a row's own amount,
		// which is exactly why an unscoped getByText would not say which is
		// the whole book's.
		const totals = testPage.getByRole('definition');
		await expect.element(totals.nth(0)).toHaveTextContent('$1,500.00');
		await expect.element(totals.nth(1)).toHaveTextContent('1');
		await expect.element(totals.nth(2)).toHaveTextContent('$2,500.00');
	});

	it('says so plainly when nothing has been billed yet', async () => {
		await setup({ items: [], hasMore: false, outstandingCents: 0, outstandingCount: 0, paidCents: 0 });

		await expect
			.element(
				testPage.getByRole('cell', {
					name: 'No invoices yet. One appears here as soon as a contract is billed.'
				})
			)
			.toBeVisible();
	});

	it('appends the next page rather than replacing the one already read', async () => {
		apiFetchWithSession.mockResolvedValue(
			jsonResponse({
				items: [
					{
						id: 'inv-3',
						engagementId: 'eng-3',
						contractId: 'contract-3',
						clientName: 'Cleo',
						status: 'open',
						amountCents: 100,
						currency: 'usd',
						createdAt: '2026-06-01T00:00:00Z'
					}
				],
				hasMore: false,
				outstandingCents: 150_100,
				outstandingCount: 2,
				paidCents: 250_000
			})
		);

		await setup({ ...data, hasMore: true, nextCursor: 'cursor-1' });
		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect.element(testPage.getByRole('link', { name: 'Cleo' })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Ada' })).toBeVisible();
		expect(apiFetchWithSession).toHaveBeenCalledWith('/api/practices/practice-1/invoices?cursor=cursor-1');
	});

	it('reports a failed next page in place rather than losing the list', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse('invalid cursor', 400));

		await setup({ ...data, hasMore: true, nextCursor: 'cursor-1' });
		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect.element(testPage.getByRole('alert')).toHaveTextContent('invalid cursor');
		await expect.element(testPage.getByRole('link', { name: 'Ada' })).toBeVisible();
	});
});
