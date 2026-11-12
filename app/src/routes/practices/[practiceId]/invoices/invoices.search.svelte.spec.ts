/*
 * #2746: finding one Invoice -- a check "for invoice INV-0042", or a bank
 * transfer that names a Client -- by its reference or its Client, and the
 * bar and the list agreeing while a newer filter loads.
 *
 * Its own file because the search lives in the history entry's state
 * (`page.state`), a page state the sibling specs never set.
 */
import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { SvelteMap, SvelteURL } from 'svelte/reactivity';
import { cleanup, render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import type { PracticeInvoiceListData } from '#lib/invoice.js';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import '#lib/styles/app.css';
import Page from './InvoicesScreen.svelte';
import { toPageState } from '../../../routeFixture.js';
import { data, fixture, narrowedToOverdue } from './page.fixture.js';

const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>,
	state: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
if (!customElements.get('center-l')) registerLayoutPrimitives();

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto, invalidate: vi.fn(), afterNavigate: vi.fn(), beforeNavigate: vi.fn(), replaceState: vi.fn() }));

const { practiceId } = fixture.params;
const listPath = `/api/practices/${practiceId}/invoices/list`;
const [overdueInvoice, paidInvoice] = data.items;

beforeEach(() => {
	apiFetchWithSession.mockReset();
	goto.mockReset();
	goto.mockResolvedValue(undefined);
	Object.assign(pageState, toPageState(fixture));
});

interface SetupOptions {
	search?: string;
	url?: string;
	read?: PracticeInvoiceListData | undefined;
}

async function setupSearch({ search = '', url = fixture.url, read = data }: SetupOptions = {}) {
	pageState.url = new URL(url);
	pageState.state = search ? { invoicesSearch: search } : {};
	await testPage.viewport(1440, 900);
	return render(Page, { data: read });
}

function lastBody(): Record<string, unknown> {
	const call = apiFetchWithSession.mock.calls.at(-1)!;
	return JSON.parse((call[1] as RequestInit).body as string) as Record<string, unknown>;
}

describe('finding an Invoice by its reference or its Client (#2746)', () => {
	it('offers one field for a reference or a Client, which never reaches the address', async () => {
		await setupSearch();

		const field = testPage.getByRole('searchbox', { name: 'Reference or Client' });
		await expect.element(field).toBeVisible();
		await expect.element(field).not.toHaveAttribute('name');
	});

	it('applies the typed search with the view, in the history entry and not the address', async () => {
		await setupSearch();

		await testPage.getByRole('radio', { name: 'Overdue' }).click();
		await testPage.getByRole('searchbox', { name: 'Reference or Client' }).fill('  INV-0042 ');
		await testPage.getByRole('button', { name: 'Apply' }).click();

		expect(goto).toHaveBeenLastCalledWith(`/practices/${practiceId}/invoices?show=overdue`, {
			state: { invoicesSearch: 'INV-0042' }
		});
	});

	it('reads a search in a POST body under the view, and says what was looked for', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ ...data, items: [overdueInvoice] }));
		await setupSearch({ search: '0042', url: narrowedToOverdue.url!, read: { ...data, view: 'overdue' } });

		await expect
			.element(testPage.getByRole('status'))
			.toHaveTextContent('Showing 1 overdue Invoice matching "0042".');
		expect(apiFetchWithSession).toHaveBeenLastCalledWith(listPath, expect.objectContaining({ method: 'POST' }));
		expect(lastBody()).toEqual({ view: 'overdue', search: '0042', cursor: '' });
		await expect.element(testPage.getByRole('searchbox', { name: 'Reference or Client' })).toHaveValue('0042');
		expect(testPage.getByRole('link', { name: paidInvoice.clientName }).elements()).toHaveLength(0);
	});

	it('shows a removable tag for the search and one for the view, each keeping the other', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ ...data, items: [overdueInvoice] }));
		await setupSearch({ search: 'Anne', url: narrowedToOverdue.url!, read: { ...data, view: 'overdue' } });

		await testPage.getByRole('link', { name: 'Remove filter: Matching Anne' }).click();
		expect(goto).toHaveBeenLastCalledWith(`/practices/${practiceId}/invoices?show=overdue`, { state: { invoicesSearch: '' } });

		await testPage.getByRole('link', { name: 'Remove filter: Overdue Invoices' }).click();
		expect(goto).toHaveBeenLastCalledWith(`/practices/${practiceId}/invoices`, { state: { invoicesSearch: 'Anne' } });

		await expect
			.element(testPage.getByRole('link', { name: 'Clear filters' }))
			.toHaveAttribute('href', `/practices/${practiceId}/invoices`);
	});

	it('says when nothing matched, and offers the way back to every Invoice', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ ...data, items: [] }));
		await setupSearch({ search: 'Zelda' });

		await expect.element(testPage.getByText('No Invoice matches "Zelda".', { exact: true }).last()).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Show every Invoice' }))
			.toHaveAttribute('href', `/practices/${practiceId}/invoices`);
	});

	it('keeps the search on every later page', async () => {
		apiFetchWithSession
			.mockResolvedValueOnce(jsonResponse({ ...data, items: [overdueInvoice], hasMore: true, nextCursor: 'cursor-1' }))
			.mockResolvedValueOnce(jsonResponse({ ...data, items: [{ ...overdueInvoice, id: 'inv-later', clientName: 'Cleo' }] }));
		await setupSearch({ search: 'Anne' });

		await expect
			.element(testPage.getByRole('status'))
			.toHaveTextContent('Showing the first 1 Invoices matching "Anne", newest first.');
		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect.element(testPage.getByRole('link', { name: 'Cleo' })).toBeVisible();
		expect(lastBody()).toEqual({ view: 'all', search: 'Anne', cursor: 'cursor-1' });
		await expect.element(testPage.getByRole('status')).toHaveTextContent('Showing 2 Invoices matching "Anne", newest first.');
	});

	it('says a search that did not run, leaves the list unmarked, and Try again runs it again', async () => {
		apiFetchWithSession.mockResolvedValueOnce(new Response('no', { status: 500 }));
		await setupSearch({ search: 'Anne' });

		await expect.element(testPage.getByText('Your search did not run. Nothing you saved is lost.')).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Apply' })).toBeEnabled();
		await expect.element(testPage.getByRole('searchbox', { name: 'Reference or Client' })).toHaveValue('Anne');

		apiFetchWithSession.mockResolvedValueOnce(jsonResponse({ ...data, items: [overdueInvoice] }));
		await testPage.getByRole('button', { name: 'Try again' }).click();

		await expect.element(testPage.getByRole('link', { name: overdueInvoice.clientName })).toBeVisible();
		await expect.element(testPage.getByText('Your search did not run. Nothing you saved is lost.')).not.toBeInTheDocument();
		expect(lastBody()).toEqual({ view: 'all', search: 'Anne', cursor: '' });
	});

	it('leaves a failed page read to the search while one is applied', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ ...data, items: [overdueInvoice] }));
		pageState.state = { invoicesSearch: 'Anne' };
		await testPage.viewport(1440, 900);
		await render(Page, { hasReadFailed: true });

		await expect.element(testPage.getByRole('link', { name: overdueInvoice.clientName })).toBeVisible();
		await expect.element(testPage.getByRole('alert')).not.toBeInTheDocument();
	});

	it('lists the whole read again when the search is removed and the address is unchanged', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ ...data, items: [overdueInvoice] }));
		// The history entry's state as SvelteKit's is: reactive, so a move that
		// changes only it is seen.
		const entry = new SvelteMap([['invoicesSearch', 'Anne']]);
		await setupSearch({ read: data });
		cleanup();
		pageState.state = {
			get invoicesSearch() {
				return entry.get('invoicesSearch');
			}
		};
		await render(Page, { data });
		await expect.element(testPage.getByRole('status')).toHaveTextContent('matching "Anne"');

		entry.set('invoicesSearch', '');

		await expect.element(testPage.getByRole('status')).toHaveTextContent('Showing 3 Invoices, newest first.');
	});
});

describe('the bar and the list agree while a newer filter loads (#2746)', () => {
	it('marks the rows and Apply busy until the read for the address lands', async () => {
		await setupSearch();
		cleanup();
		// Reactive, because the route reads the address through a derived.
		const url = new SvelteURL(fixture.url);
		pageState.url = url;
		const { rerender } = await render(Page, { data });
		await expect.element(testPage.getByRole('table')).toHaveAttribute('aria-busy', 'false');

		url.searchParams.set('show', 'overdue');

		await expect.element(testPage.getByRole('table')).toHaveAttribute('aria-busy', 'true');
		await expect.element(testPage.getByRole('status')).toHaveTextContent('Showing 3 Invoices, newest first.');

		await rerender({ data: { ...data, items: [overdueInvoice], view: 'overdue' } });

		await expect.element(testPage.getByRole('table')).toHaveAttribute('aria-busy', 'false');
		await expect.element(testPage.getByRole('status')).toHaveTextContent('Showing 1 overdue Invoice.');
	});

	it('holds the applied tags row while no filter is on, so applying one moves nothing under it', async () => {
		await setupSearch();
		const before = testPage.getByRole('status').element().getBoundingClientRect().top;

		cleanup();
		await setupSearch({ url: narrowedToOverdue.url!, read: { ...data, view: 'overdue' } });

		await expect.element(testPage.getByRole('link', { name: 'Remove filter: Overdue Invoices' })).toBeVisible();
		expect(testPage.getByRole('status').element().getBoundingClientRect().top).toBe(before);
	});

	it('brings the totals up to date with each later page', async () => {
		apiFetchWithSession.mockResolvedValue(
			jsonResponse({ ...data, items: [{ ...overdueInvoice, id: 'inv-later' }], outstandingCents: 180_000, outstandingCount: 1 })
		);
		await setupSearch({ read: { ...data, hasMore: true, nextCursor: 'cursor-1' } });

		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect.element(testPage.getByRole('definition').nth(0)).toHaveTextContent('$1,800.00 on 1 Invoice');
	});
});

describe('the state said once (DESIGN.md, "Money is said once")', () => {
	it('says a paid Invoice in a plain word and an overdue one in its badge', async () => {
		await setupSearch();

		const paid = testPage.getByRole('cell', { name: /^\$2,500\.00\s*,\s*paid$/ });
		const overdue = testPage.getByRole('cell', { name: /^\$4,500\.00\s*,\s*56 days overdue$/ });
		await expect.element(paid).toBeVisible();
		await expect.element(overdue).toBeVisible();
		// querySelector, case 2: a badge's icon is drawn and not announced, so
		// only the drawn tree tells a badge from a plain word.
		expect(paid.element().querySelector('svg')).toBeNull();
		expect(overdue.element().querySelector('svg')).not.toBeNull();
	});
});
