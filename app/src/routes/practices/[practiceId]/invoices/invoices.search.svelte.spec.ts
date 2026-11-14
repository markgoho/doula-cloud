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
import { data, fixture, narrowedToOverdue, typical } from './page.fixture.js';
import { apiFetchWithSession, goto, pageState } from '#testing/routePage.js';

vi.mock('$app/state', () => import('#testing/routePage.js'));
vi.mock('#lib/api.js', () => import('#testing/routePage.js'));
vi.mock('$app/navigation', async () => ({
	...(await import('#testing/routePage.js')),
	invalidate: vi.fn(),
	afterNavigate: vi.fn(),
	beforeNavigate: vi.fn(),
	replaceState: vi.fn(),
	snapshot: vi.fn()
}));
if (!customElements.get('center-l')) registerLayoutPrimitives();

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
	width?: number;
}

async function setupSearch({ search = '', url = fixture.url, read = data, width = 1440 }: SetupOptions = {}) {
	pageState.url = new URL(url);
	pageState.state = search ? { invoicesSearch: search } : {};
	await testPage.viewport(width, 900);
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
			.toHaveTextContent('Showing 1 overdue Invoice matching "0042", longest overdue first.');
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
			.element(testPage.getByRole('link', { name: 'Show all Invoices' }))
			.toHaveAttribute('href', `/practices/${practiceId}/invoices`);
		// H8: heard once, from the live result line; the empty state draws it for sight only.
		await expect.element(testPage.getByRole('status').filter({ hasText: 'No Invoice matches "Zelda".' })).toBeInTheDocument();
		expect(testPage.getByRole('paragraph').filter({ hasText: 'No Invoice matches' }).elements()).toHaveLength(0);
	});

	// H10: in a narrowed view the way out keeps what she typed.
	it('offers to search every Invoice for the same text when nothing matched in a narrowed view', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ ...data, items: [] }));
		await setupSearch({ search: 'INV-0035', url: narrowedToOverdue.url!, read: { ...data, view: 'overdue' } });

		await testPage.getByRole('button', { name: 'Search all Invoices for "INV-0035"' }).click();

		expect(goto).toHaveBeenLastCalledWith(`/practices/${practiceId}/invoices`, { state: { invoicesSearch: 'INV-0035' } });
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

		await expect.element(testPage.getByText('Your search did not run.')).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Apply' })).toBeEnabled();
		await expect.element(testPage.getByRole('searchbox', { name: 'Reference or Client' })).toHaveValue('Anne');

		apiFetchWithSession.mockResolvedValueOnce(jsonResponse({ ...data, items: [overdueInvoice] }));
		await testPage.getByRole('button', { name: 'Try again' }).click();

		await expect.element(testPage.getByRole('link', { name: overdueInvoice.clientName })).toBeVisible();
		await expect.element(testPage.getByText('Your search did not run.')).not.toBeInTheDocument();
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
		await expect.element(testPage.getByRole('status')).toHaveTextContent('Showing 1 overdue Invoice, longest overdue first.');
	});

	it('holds one tag\'s room beside the result line while no filter is on, so applying one moves nothing under it', async () => {
		const tableTop = () => testPage.getByRole('table').element().getBoundingClientRect().top;
		await setupSearch();
		const before = tableTop();

		cleanup();
		await setupSearch({ url: narrowedToOverdue.url!, read: { ...data, view: 'overdue' } });

		await expect.element(testPage.getByRole('link', { name: 'Remove filter: Overdue Invoices' })).toBeVisible();
		expect(Math.abs(tableTop() - before)).toBeLessThanOrEqual(0.5);
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
	it('says a refunded Invoice beside its amount and its paid day, and an overdue one in its badge', async () => {
		await setupSearch();

		const paid = testPage.getByRole('cell', { name: /^\$2,500\.00\s*,\s*paid, \$1,250\.50 refunded$/ });
		const overdue = testPage.getByRole('cell', { name: /^\$4,500\.00\s*,\s*56 days overdue$/ });
		await expect.element(paid).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: /^Paid Jul 4, 2026$/ }).last()).toBeVisible();
		await expect.element(overdue).toBeVisible();
		// querySelector, case 2: a badge's icon is drawn and not announced, so
		// only the drawn tree tells a badge from a plain word.
		expect(paid.element().querySelector('svg')).toBeNull();
		expect(overdue.element().querySelector('svg')).not.toBeNull();
	});

	// H4, C3 (#2746): written off is failed, so it is the red badge the
	// Engagement's Invoices section draws it in; void stays a plain word.
	it('draws a written-off Invoice in the red badge, as the Engagement does, and void as a plain word', async () => {
		await setupSearch({
			read: {
				...data,
				items: [
					{ ...data.items[0]!, status: 'uncollectible', paidOn: undefined, daysOverdue: 0 },
					{ ...data.items[0]!, id: 'inv-void', reference: 'INV-0050', status: 'void', paidOn: undefined, daysOverdue: 0 }
				]
			}
		});

		const writtenOff = testPage.getByRole('cell', { name: /,\s*written off$/ });
		await expect.element(writtenOff).toBeVisible();
		const badge = writtenOff.getByText('written off', { exact: true }).element();
		expect(getComputedStyle(badge).getPropertyValue('--badge-hue').trim()).toBe(
			getComputedStyle(document.documentElement).getPropertyValue('--color-error').trim()
		);
		const voided = testPage.getByRole('cell', { name: /,\s*void$/ });
		await expect.element(voided).toBeVisible();
		expect(voided.element().querySelector('svg')).toBeNull();
	});
});

// C4 (#2746): every column starts, and the amounts end, at one place whichever
// rows a view or a search shows -- a four-figure amount in one, none in another.
const typicalBook = typical.props!.data as PracticeInvoiceListData;
// The longest state a row can say, "paid, $1,500.00 refunded", on one paid row.
const book: PracticeInvoiceListData = {
	...typicalBook,
	items: typicalBook.items.map((invoice, index) => (index === 1 ? { ...invoice, refundedCents: invoice.amountCents } : invoice))
};
const views = [
	{ name: 'all', url: fixture.url, search: '', items: book.items, view: 'all' },
	{ name: 'unpaid', url: `${fixture.url}?show=unpaid`, search: '', items: book.items.filter((invoice) => invoice.status === 'open'), view: 'unpaid' },
	{ name: 'overdue', url: narrowedToOverdue.url!, search: '', items: book.items.filter((invoice) => invoice.daysOverdue > 0), view: 'overdue' },
	{ name: '"Falk"', url: fixture.url, search: 'Falk', items: book.items.filter((invoice) => invoice.clientName.includes('Falk')), view: 'all' },
	{ name: '"DC-0015"', url: fixture.url, search: 'DC-0015', items: book.items.filter((invoice) => invoice.reference === 'DC-0015'), view: 'all' }
] as const;

// Where each column starts and where the amounts end, once the table is drawn.
async function columnEdges(label: string): Promise<number[]> {
	await expect.element(testPage.getByRole('table')).toBeVisible();
	await document.fonts.ready;
	expect(testPage.getByRole('row').elements().length, `${label} shows rows`).toBeGreaterThan(1);
	const starts = testPage.getByRole('columnheader').elements().map((header) => Math.round(header.getBoundingClientRect().left));
	const table = testPage.getByRole('table').element();
	// querySelector, case 2: the amount's own box, inside a cell a screen reader hears whole.
	const ends = new Set([...table.querySelectorAll(':scope > tbody .amount')].map((amount) => Math.round(amount.getBoundingClientRect().right)));
	expect(ends.size, `amounts end on one edge in ${label}`).toBe(1);
	const bodyRows = testPage.getByRole('row').filter({ has: testPage.getByRole('rowheader') }).elements();
	for (const row of bodyRows) {
		expect(row.getBoundingClientRect().height, `a row's height in ${label}`).toBeCloseTo(40, 0);
	}
	// Round 6's finding (#2746): a two-word Client name stays on one line.
	for (const name of testPage.getByRole('rowheader').getByRole('link').elements()) {
		expect(name.getClientRects(), `${name.textContent} on one line in ${label}`).toHaveLength(1);
	}
	return [...starts, ...ends];
}

// C4 (#2746, #2849): every column starts, and the amounts end, at one place
// whichever rows a view, a search or Load more shows -- a four-figure amount
// in one, only a three-figure one in another, a short name in a third.
describe('the columns across views (#2849)', () => {
	it.each([1280, 1440, 1920, 2560])('holds every column and the amounts’ edge still, every row 40px, at a %ipx frame', async (width) => {
		const placed: Record<string, number[]> = {};
		for (const { name, url, search, items, view } of views) {
			const read = { ...book, items: [...items], view };
			apiFetchWithSession.mockResolvedValue(jsonResponse(read));
			await setupSearch({ search, url, read, width });
			placed[name] = await columnEdges(name);
			cleanup();
		}

		apiFetchWithSession.mockResolvedValue(jsonResponse({ ...book, items: book.items.slice(4) }));
		await setupSearch({ read: { ...book, items: book.items.slice(0, 4), hasMore: true, nextCursor: 'cursor-1' }, width });
		const firstPage = await columnEdges('the first page');
		await testPage.getByRole('button', { name: 'Load more' }).click();
		await expect.element(testPage.getByRole('link', { name: book.items.at(-1)!.clientName })).toBeVisible();
		placed['the first page'] = firstPage;
		placed['after Load more'] = await columnEdges('after Load more');

		for (const [name, edges] of Object.entries(placed)) {
			expect(edges, `${name} against all`).toStrictEqual(placed.all);
		}
	});
});
