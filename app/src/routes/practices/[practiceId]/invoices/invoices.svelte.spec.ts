import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import type { PracticeInvoicePage } from '#lib/invoice.js';
// DataTable's frame needs stack-l's display:block default (primitives.css)
// to work as a container-query context -- see DataTable.svelte.spec.ts.
import '#lib/styles/app.css';
import Page from './+page.svelte';
import { toPageState } from '../../../routeFixture.js';
import { data, fixture } from './page.fixture.js';

/*
 * The screen's content and the `page` it reads both come from the route's
 * own fixture (#596), so what this spec asserts on and what the continuum
 * sweep measures are one description. `vi.mock` is hoisted above every
 * import, so the object is declared empty here and filled from the
 * fixture once the imports have run -- the route reads `page` inside its
 * own functions rather than destructuring it at module scope, so the
 * later write is seen. This is the same installation
 * `route-continuum.svelte.spec.ts` performs, through the same
 * `toPageState`.
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

const [openInvoice, paidInvoice] = data.items;
const { practiceId } = fixture.params;

beforeEach(() => {
	apiFetchWithSession.mockReset();
});

// `session` merges in from practices/[practiceId]/+layout.ts (#835) --
// this route never reads it, but the generated `data` prop type requires
// it, since SvelteKit really does merge ancestor layout data into it at
// runtime.
const sessionStub = {
	practiceId,
	practiceName: 'Riverside Doula Collective',
	roles: ['owner'],
	isContractor: false
};

async function setup(page: PracticeInvoicePage = data) {
	// Wide enough for DataTable's <table> rather than the <dl> record view
	// its content floor stacks into below 46rem (#508) -- the same call the
	// Billing ledger's spec makes for the same reason. What the list says
	// is asserted here; that it says the same thing in a narrow container
	// is DataTable's own spec's job.
	await testPage.viewport(1440, 900);
	await render(Page, { params: fixture.params, data: { ...page, session: sessionStub } });
}

describe('the Practice-wide invoice list (#265)', () => {
	it('answers "who owes us money" with the whole book, not one Engagement', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('heading', { name: fixture.readyText }))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: openInvoice.clientName }))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: paidInvoice.clientName }))
			.toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: '$4,500.00' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Open' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Paid', exact: true })).toBeVisible();
	});

	it('names each Client as the way in to her Engagement', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('link', { name: openInvoice.clientName }))
			.toHaveAttribute('href', `/practices/${practiceId}/engagements/${openInvoice.engagementId}`);
	});

	it('shows what is outstanding across the Practice', async () => {
		await setup();

		// The summary is a description list, so its three values are read
		// positionally -- both figures also appear as a row's own amount,
		// which is exactly why an unscoped getByText would not say which is
		// the whole book's.
		const totals = testPage.getByRole('definition');
		await expect.element(totals.nth(0)).toHaveTextContent('$4,500.00');
		await expect.element(totals.nth(1)).toHaveTextContent('1');
		await expect.element(totals.nth(2)).toHaveTextContent('$2,500.00');
	});

	it('says so plainly when nothing has been billed yet', async () => {
		// Not the happy path, so it is this spec's own to declare -- but it
		// is declared as a departure from the fixture rather than as a
		// second description of the same screen.
		await setup({ ...data, items: [], outstandingCents: 0, outstandingCount: 0, paidCents: 0 });

		await expect
			.element(
				testPage.getByRole('cell', {
					name: 'No invoices yet. One appears here as soon as a contract is billed.'
				})
			)
			.toBeVisible();
	});

	it('appends the next page rather than replacing the one already read', async () => {
		// A second page is content the fixture does not hold: the screen the
		// sweep measures is the first page, and this is what arrives after an
		// interaction.
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
				outstandingCents: 450_100,
				outstandingCount: 2,
				paidCents: 250_000
			})
		);

		await setup({ ...data, hasMore: true, nextCursor: 'cursor-1' });
		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect.element(testPage.getByRole('link', { name: 'Cleo' })).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: openInvoice.clientName }))
			.toBeVisible();
		expect(apiFetchWithSession).toHaveBeenCalledWith(
			`/api/practices/${practiceId}/invoices?cursor=cursor-1`
		);
	});

	it('reports a failed next page in place rather than losing the list', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse('invalid cursor', 400));

		await setup({ ...data, hasMore: true, nextCursor: 'cursor-1' });
		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect.element(testPage.getByRole('alert')).toHaveTextContent('invalid cursor');
		await expect
			.element(testPage.getByRole('link', { name: openInvoice.clientName }))
			.toBeVisible();
	});
});
