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
const navigation = vi.hoisted(() => ({ goto: vi.fn(), onMove: undefined as (() => void) | undefined }));
vi.mock('$app/navigation', () => ({
	goto: navigation.goto,
	invalidate: vi.fn(),
	afterNavigate: (callback: () => void) => {
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

		// $4,500.00 owed, all of it overdue -- the fixture's first
		// row is open and past its due date.
		await expect.element(testPage.getByText('Overdue invoices', { exact: true })).toBeVisible();
		await expect
			.element(testPage.getByRole('term').filter({ hasText: /^Overdue$/ }))
			.toBeVisible();
	});

	it('says how late an unpaid invoice is, in the row itself', async () => {
		await setup();

		// The cell, not the whole row: DataTable renders both a table cell
		// and a record-view line for every value, and only one of them is
		// on screen at this width.
		await expect
			.element(testPage.getByRole('cell', { name: /days overdue/ }))
			.toBeVisible();
	});

	it('offers the narrowing as the filter bar every Staff list uses, and says the whole book is shown', async () => {
		await setup();

		await expect.element(testPage.getByRole('checkbox', { name: 'Only overdue Invoices' })).not.toBeChecked();
		await expect.element(testPage.getByRole('button', { name: 'Apply' })).toBeVisible();
		await expect.element(testPage.getByRole('status')).toHaveTextContent('Showing every Invoice.');
		await expect.element(testPage.getByRole('link', { name: 'Clear filters' })).not.toBeInTheDocument();
	});

	it('names the field after the address parameter, so the form reaches it with no script', async () => {
		await setup();

		await expect.element(testPage.getByRole('checkbox', { name: 'Only overdue Invoices' })).toHaveAttribute('name', 'overdue');
		const form = testPage.getByRole('button', { name: 'Apply' }).element().closest('form')!;
		expect(form.method).toBe('get');
		expect(new URL(form.action).pathname).toBe(`/practices/${practiceId}/invoices`);
	});

	it('carries the overdue count in the result line once narrowed', async () => {
		pageState.url = new URL(narrowedToOverdue.url!);
		await setup({ ...data, isNarrowedToOverdue: true, overdueCount: 1 });

		await expect.element(testPage.getByRole('status')).toHaveTextContent('Showing 1 overdue Invoice.');
		await expect.element(testPage.getByRole('checkbox', { name: 'Only overdue Invoices' })).toBeChecked();
		await expect
			.element(testPage.getByRole('link', { name: 'Clear filters' }))
			.toHaveAttribute('href', `/practices/${practiceId}/invoices`);
	});

	// The same arrangement as the Clients list's bar: switches above, Apply with
	// the held "Clear filters" box on one row, the count under them on the left edge, a quiet legend.
	it('draws the filter bar the way the Clients list does, and does not move Apply once narrowed', async () => {
		await setup();
		const apply = () => testPage.getByRole('button', { name: 'Apply' }).element().getBoundingClientRect();
		const before = apply();
		const status = testPage.getByRole('status').element().getBoundingClientRect();
		const switchBox = testPage.getByRole('checkbox', { name: 'Only overdue Invoices' }).element().getBoundingClientRect();
		expect(switchBox.bottom).toBeLessThanOrEqual(before.top);
		expect(status.top).toBeGreaterThanOrEqual(before.bottom);
		const legend = testPage.getByRole('group', { name: 'Choose which Invoices to show' }).element().querySelector('legend')!;
		expect(legend.getBoundingClientRect().height).toBeLessThanOrEqual(1);
		await expect.element(testPage.getByRole('link', { name: 'Clear filters' })).not.toBeInTheDocument();

		cleanup();
		pageState.url = new URL(narrowedToOverdue.url!);
		await setup({ ...data, isNarrowedToOverdue: true, overdueCount: 1 });

		await expect.element(testPage.getByRole('link', { name: 'Clear filters' })).toBeVisible();
		expect(apply().left).toBe(before.left);
		expect(apply().top).toBe(before.top);
		expect(testPage.getByRole('status').element().getBoundingClientRect().left).toBe(status.left);
	});

	it('goes to the overdue address on Apply with the box ticked, and to the bare one with it clear', async () => {
		await setup();

		await testPage.getByRole('checkbox', { name: 'Only overdue Invoices' }).click();
		await testPage.getByRole('button', { name: 'Apply' }).click();
		expect(navigation.goto).toHaveBeenLastCalledWith(`/practices/${practiceId}/invoices?overdue=true`);

		await testPage.getByRole('checkbox', { name: 'Only overdue Invoices' }).click();
		await testPage.getByRole('button', { name: 'Apply' }).click();
		expect(navigation.goto).toHaveBeenLastCalledWith(`/practices/${practiceId}/invoices`);
	});

	it('moves focus to the result line once a changed narrowing has loaded, never on first arrival', async () => {
		// Reactive, because the route reads the address through a derived.
		pageState.url = new SvelteURL(fixture.url);
		const { rerender } = await setup();
		navigation.onMove!();
		await expect.element(testPage.getByRole('status')).not.toHaveFocus();

		pageState.url.searchParams.set('overdue', 'true');
		navigation.onMove!();
		await rerender({ data: { ...data, isNarrowedToOverdue: true, overdueCount: 1 } });

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
			isNarrowedToOverdue: true,
			hasMore: true,
			nextCursor: 'cursor-1',
		});
		await testPage.getByRole('button', { name: 'Load more' }).click();

		expect(apiFetchWithSession).toHaveBeenCalledWith(
			`/api/practices/${practiceId}/invoices?overdue=true&cursor=cursor-1`
		);
	});

	it('says nothing is late rather than "no invoices yet" when the narrowed list is empty', async () => {
		await setup({
			...data,
			items: [],
			isNarrowedToOverdue: true,
			overdueCents: 0,
			overdueCount: 0,
		});

		await expect
			.element(testPage.getByText(/Nothing is overdue/))
			.toBeVisible();
	});

	it('shows only the narrowed rows when the narrowing changes, and the whole book when it is lifted (#2254)', async () => {
		const [overdueInvoice, paidInvoice] = data.items;
		const { rerender } = await setup();

		await rerender({
			data: { ...data, items: [overdueInvoice], isNarrowedToOverdue: true },
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

	it('marks the narrowing the URL names before the read lands (#2255)', async () => {
		pageState.url = new URL(narrowedToOverdue.url!);
		await testPage.viewport(1440, 900);
		await render(Page, {});

		await expect.element(testPage.getByRole('checkbox', { name: 'Only overdue Invoices' })).toBeChecked();
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
