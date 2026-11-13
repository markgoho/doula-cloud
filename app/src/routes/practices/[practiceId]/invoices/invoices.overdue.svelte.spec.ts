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
import { SvelteURL } from 'svelte/reactivity';
import { cleanup, render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import type { PracticeInvoiceListData } from '#lib/invoice.js';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
// See invoices.svelte.spec.ts: ListPage's frame (#1576) needs both the
// primitives' CSS and their registration for the table to get its room.
import '#lib/styles/app.css';
import Page from './InvoicesScreen.svelte';
import { toPageState } from '../../../routeFixture.js';
import { data, fixture, narrowedToOverdue } from './page.fixture.js';

const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>,
}));
vi.mock('$app/state', () => ({ page: pageState }));
if (!customElements.get('center-l')) registerLayoutPrimitives();
Object.assign(pageState, toPageState(fixture));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

// The route's own `afterNavigate` callback, kept so a spec can play a move to
// this list (SvelteKit calls it after every navigation, the first load too).
const navigation = vi.hoisted(() => ({ goto: vi.fn(), onMove: undefined as ((move: { from: undefined }) => void) | undefined }));
vi.mock('$app/navigation', () => ({
	goto: navigation.goto,
	invalidate: vi.fn(),
	beforeNavigate: vi.fn(),
	replaceState: vi.fn(),
	afterNavigate: (callback: (move: { from: undefined }) => void) => {
		navigation.onMove = callback;
	}
}));

const { practiceId } = fixture.params;

beforeEach(() => {
	apiFetchWithSession.mockReset();
	navigation.goto.mockReset();
	Object.assign(pageState, toPageState(fixture));
});

async function setup(page: PracticeInvoiceListData = data) {
	// Wide enough for DataTable's <table> rather than the record view its
	// content floor stacks into below 46rem -- the same call the sibling
	// spec makes, for the same reason.
	await testPage.viewport(1440, 900);
	return render(Page, { data: page });
}

describe('the overdue book (#768)', () => {
	it('says how much of what Clients owe is late, beside that figure itself', async () => {
		await setup();

		// $4,500.00 owed on one Invoice is overdue -- the fixture's first
		// row is open and past its due date. Its label says it is a part of
		// what Clients owe, the line above it.
		const terms = testPage.getByRole('term');
		await expect.element(terms.nth(0)).toHaveTextContent('Clients owe');
		await expect.element(terms.nth(1)).toHaveTextContent('Of that, overdue');
		await expect.element(testPage.getByRole('definition').nth(1)).toHaveTextContent('$4,500.00 on 1 Invoice');
	});

	it('says how late an unpaid invoice is with its amount, and the day payment was due beside it', async () => {
		await setup();

		// The cell, not the whole row: DataTable renders both a table cell
		// and a record-view line for every value, and only one of them is
		// on screen at this width.
		await expect.element(testPage.getByRole('cell', { name: /^\$4,500\.00\s*,\s*56 days overdue$/ })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Payment due Aug 31, 2026', exact: true })).toBeVisible();
	});

	it('offers every Invoice, unpaid and overdue as one choice, and says what is shown and in what order', async () => {
		await setup();

		await expect.element(testPage.getByRole('radio', { name: 'Every Invoice' })).toBeChecked();
		await expect.element(testPage.getByRole('radio', { name: 'Unpaid' })).not.toBeChecked();
		await expect.element(testPage.getByRole('radio', { name: 'Overdue' })).not.toBeChecked();
		await expect.element(testPage.getByRole('button', { name: 'Apply' })).toBeVisible();
		await expect.element(testPage.getByRole('status')).toHaveTextContent('Showing 3 Invoices, newest first.');
		await expect.element(testPage.getByRole('link', { name: 'Clear filters' })).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('list', { name: 'Applied filters' })).not.toBeInTheDocument();
	});

	it('names the radios so the form reaches an address the load reads with no script', async () => {
		await setup();

		await expect.element(testPage.getByRole('radio', { name: 'Overdue' })).toHaveAttribute('name', 'show');
		await expect.element(testPage.getByRole('radio', { name: 'Overdue' })).toHaveAttribute('value', 'overdue');
		const form = testPage.getByRole('button', { name: 'Apply' }).element().closest('form')!;
		expect(form.method).toBe('get');
		expect(new URL(form.action).pathname).toBe(`/practices/${practiceId}/invoices`);
	});

	it('says how many are overdue and in what order once narrowed, with a tag that removes it', async () => {
		pageState.url = new URL(narrowedToOverdue.url!);
		await setup({ ...data, view: 'overdue', items: [data.items[0], data.items[0]].map((row, index) => ({ ...row, id: `late-${index}` })), overdueCount: 2 });

		await expect.element(testPage.getByRole('status')).toHaveTextContent('Showing 2 overdue Invoices, longest overdue first.');
		await expect.element(testPage.getByRole('radio', { name: 'Overdue' })).toBeChecked();
		await expect
			.element(testPage.getByRole('link', { name: 'Clear filters' }))
			.toHaveAttribute('href', `/practices/${practiceId}/invoices`);
		await expect
			.element(testPage.getByRole('link', { name: 'Remove filter: Overdue Invoices' }))
			.toHaveAttribute('href', `/practices/${practiceId}/invoices`);
	});

	it('says how far through a longer list the reader is, and counts the rows Load more adds', async () => {
		apiFetchWithSession.mockResolvedValue(
			jsonResponse({ ...data, items: [{ ...data.items[0], id: 'inv-later', clientName: 'Cleo' }], hasMore: false })
		);
		await setup({ ...data, invoiceCount: 4, hasMore: true, nextCursor: 'cursor-1' });

		await expect.element(testPage.getByRole('status')).toHaveTextContent('Showing the first 3 of 4 Invoices, newest first.');
		await testPage.getByRole('button', { name: 'Load more' }).click();
		await expect.element(testPage.getByRole('status')).toHaveTextContent('Showing 4 Invoices, newest first.');
	});

	// The radios and the search on one line with Apply after them and the held
	// "Clear filters" box, the count under them on the left edge, a quiet
	// legend: the Clients list's bar, with its fields' row holding the radios.
	it('draws the radios beside Apply, and does not move Apply once narrowed', async () => {
		await setup();
		const apply = () => testPage.getByRole('button', { name: 'Apply' }).element().getBoundingClientRect();
		const before = apply();
		const status = testPage.getByRole('status').element().getBoundingClientRect();
		const radio = testPage.getByRole('radio', { name: 'Overdue' }).element().getBoundingClientRect();
		expect(radio.right).toBeLessThanOrEqual(before.left);
		expect(radio.bottom).toBeGreaterThan(before.top);
		expect(status.top).toBeGreaterThanOrEqual(before.bottom);
		const legend = testPage.getByRole('group', { name: 'Choose which Invoices to show' }).element().querySelector('legend')!;
		expect(legend.getBoundingClientRect().height).toBeLessThanOrEqual(1);
		await expect.element(testPage.getByRole('link', { name: 'Clear filters' })).not.toBeInTheDocument();

		cleanup();
		pageState.url = new URL(narrowedToOverdue.url!);
		await setup({ ...data, view: 'overdue', overdueCount: 1 });

		await expect.element(testPage.getByRole('link', { name: 'Clear filters' })).toBeVisible();
		expect(apply().left).toBe(before.left);
		expect(apply().top).toBe(before.top);
		// The tag stands before the line on its row, so the line starts after it.
		expect(testPage.getByRole('status').element().getBoundingClientRect().left).toBeGreaterThan(status.left);
	});

	it('goes to the address for the chosen view on Apply', async () => {
		await setup();

		await testPage.getByRole('radio', { name: 'Overdue' }).click();
		await testPage.getByRole('button', { name: 'Apply' }).click();
		const noSearch = { state: { invoicesSearch: '' } };
		expect(navigation.goto).toHaveBeenLastCalledWith(`/practices/${practiceId}/invoices?show=overdue`, noSearch);

		await testPage.getByRole('radio', { name: 'Unpaid' }).click();
		await testPage.getByRole('button', { name: 'Apply' }).click();
		expect(navigation.goto).toHaveBeenLastCalledWith(`/practices/${practiceId}/invoices?show=unpaid`, noSearch);

		await testPage.getByRole('radio', { name: 'Every Invoice' }).click();
		await testPage.getByRole('button', { name: 'Apply' }).click();
		expect(navigation.goto).toHaveBeenLastCalledWith(`/practices/${practiceId}/invoices`, noSearch);
	});

	it('moves focus to the result line once a changed narrowing has loaded, never on first arrival', async () => {
		// Reactive, because the route reads the address through a derived.
		pageState.url = new SvelteURL(fixture.url);
		const { rerender } = await setup();
		navigation.onMove!({ from: undefined });
		await expect.element(testPage.getByRole('status')).not.toHaveFocus();

		pageState.url.searchParams.set('show', 'overdue');
		navigation.onMove!({ from: undefined });
		await rerender({ data: { ...data, view: 'overdue', overdueCount: 1 } });

		await expect.element(testPage.getByRole('status')).toHaveFocus();
	});

	it('asks every later page for the same narrowing it was read under', async () => {
		apiFetchWithSession.mockResolvedValue(
			jsonResponse({
				...data,
				items: [],
				hasMore: false,
				nextCursor: undefined,
			})
		);

		await setup({
			...data,
			view: 'overdue',
			hasMore: true,
			nextCursor: 'cursor-1',
		});
		await testPage.getByRole('button', { name: 'Load more' }).click();

		expect(apiFetchWithSession).toHaveBeenCalledWith(
			`/api/practices/${practiceId}/invoices?overdue=true&cursor=cursor-1`
		);
	});

	it.each([
		['overdue', 'No Invoice is overdue.'],
		['unpaid', 'No Invoice is unpaid.']
	] as const)('says what an empty %s view looked for, and the way back to every Invoice', async (view, text) => {
		await setup({ ...data, items: [], view, overdueCents: 0, overdueCount: 0, outstandingCount: 0 });

		// The empty state, under the bar; the bar's own status line says the same words for a screen reader only.
		await expect.element(testPage.getByText(text, { exact: true }).last()).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Show every Invoice' }))
			.toHaveAttribute('href', `/practices/${practiceId}/invoices`);
	});

	it('shows only the narrowed rows when the narrowing changes, and the whole book when it is lifted (#2254)', async () => {
		const [overdueInvoice, paidInvoice] = data.items;
		const { rerender } = await setup();

		await rerender({
			data: { ...data, items: [overdueInvoice], view: 'overdue' },
		});

		await expect
			.element(testPage.getByRole('link', { name: overdueInvoice.clientName }))
			.toBeVisible();
		expect(
			testPage.getByRole('link', { name: paidInvoice.clientName }).elements()
		).toHaveLength(0);

		await rerender({ data: { ...data } });

		await expect
			.element(testPage.getByRole('link', { name: paidInvoice.clientName }))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: overdueInvoice.clientName }))
			.toBeVisible();
	});

	it('marks the view the address names', async () => {
		pageState.url = new URL(`https://example.test/practices/${practiceId}/invoices?show=unpaid`);
		await setup({ ...data, view: 'unpaid' });

		await expect.element(testPage.getByRole('radio', { name: 'Unpaid' })).toBeChecked();
		await expect.element(testPage.getByRole('link', { name: 'Remove filter: Unpaid Invoices' })).toBeVisible();
	});

	it('marks the narrowing the URL names before the read lands (#2255)', async () => {
		pageState.url = new URL(narrowedToOverdue.url!);
		await testPage.viewport(1440, 900);
		await render(Page, {});

		await expect.element(testPage.getByRole('radio', { name: 'Overdue' })).toBeChecked();
	});

	it('keeps the pages already loaded when the same read is handed back', async () => {
		apiFetchWithSession.mockResolvedValue(
			jsonResponse({
				...data,
				items: [{ ...data.items[0], id: 'inv-3', clientName: 'Cleo' }],
				hasMore: false,
			})
		);
		const first = { ...data, hasMore: true, nextCursor: 'cursor-1' };
		const { rerender } = await setup(first);
		await testPage.getByRole('button', { name: 'Load more' }).click();
		await expect
			.element(testPage.getByRole('link', { name: 'Cleo' }))
			.toBeVisible();

		await rerender({ data: first });

		await expect
			.element(testPage.getByRole('link', { name: 'Cleo' }))
			.toBeVisible();
	});
});
