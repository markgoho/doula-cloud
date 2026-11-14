/*
 * #2869: the order a reader chooses for the Invoices list -- the Order
 * select in the bar, an applied tag, the result line's words, the sorted
 * column, and the order kept by every move that stays on the list.
 *
 * Its own file for the same reason the search's is: it sets a page state
 * (the address's `?order=`, the history entry's search) the others do not.
 */
import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import type { PracticeInvoiceListData } from '#lib/invoice.js';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import '#lib/styles/app.css';
import Page from './InvoicesScreen.svelte';
import { toPageState } from '../../../routeFixture.js';
// jscpd:ignore-start -- vi.mock is hoisted per file, so each spec of this screen restates the same module mocks
import { data, fixture } from './page.fixture.js';
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
// jscpd:ignore-end

const { practiceId } = fixture.params;
const path = `/practices/${practiceId}/invoices`;
const listPath = `${path}/list`.replace('/practices/', '/api/practices/');

beforeEach(() => {
	apiFetchWithSession.mockReset();
	goto.mockReset();
	goto.mockResolvedValue(undefined);
	Object.assign(pageState, toPageState(fixture));
});

async function setupOrder(query = '', read: Partial<PracticeInvoiceListData> = {}, search = '') {
	pageState.url = new URL(`${fixture.url}${query}`);
	pageState.state = search ? { invoicesSearch: search } : {};
	await testPage.viewport(1440, 900);
	return render(Page, { data: { ...data, ...read } });
}

const orderSelect = () => testPage.getByRole('combobox', { name: 'Order' });
const optionLabels = () => [...orderSelect().element().querySelectorAll('option')].map((option) => option.textContent);

describe('the Order select (#2869)', () => {
	it('says the whole book’s own order on its empty option, and offers the other five', async () => {
		await setupOrder();

		await expect.element(orderSelect()).toHaveValue('');
		await expect.element(orderSelect()).toHaveDisplayValue('Sent, newest first');
		expect(optionLabels()).toEqual([
			'Sent, newest first',
			'Sent, oldest first',
			'Payment due, earliest first',
			'Payment due, latest first',
			'Largest amount first',
			'Smallest amount first'
		]);
	});

	it('says payment due, earliest first on Unpaid and Overdue, and offers sent newest first instead', async () => {
		await setupOrder('?show=overdue', { view: 'overdue' });

		await expect.element(orderSelect()).toHaveDisplayValue('Payment due, earliest first');
		expect(optionLabels()).toEqual([
			'Payment due, earliest first',
			'Sent, newest first',
			'Sent, oldest first',
			'Payment due, latest first',
			'Largest amount first',
			'Smallest amount first'
		]);
	});

	it('follows the radio before Apply, and drops a pick that the new view already does', async () => {
		await setupOrder();

		await orderSelect().selectOptions('Payment due, earliest first');
		await testPage.getByRole('radio', { name: 'Unpaid' }).click();
		await expect.element(orderSelect()).toHaveDisplayValue('Payment due, earliest first');
		await expect.element(orderSelect()).toHaveValue('');
		await testPage.getByRole('button', { name: 'Apply' }).click();

		expect(goto).toHaveBeenLastCalledWith(`${path}?show=unpaid`, { state: { invoicesSearch: '' } });
	});

	it('goes to ?order= on Apply beside the view and the search, and leaves the view’s own order out', async () => {
		await setupOrder();

		await testPage.getByRole('radio', { name: 'Overdue' }).click();
		await orderSelect().selectOptions('Largest amount first');
		await testPage.getByRole('searchbox', { name: 'Reference or Client' }).fill('Anne');
		await testPage.getByRole('button', { name: 'Apply' }).click();
		expect(goto).toHaveBeenLastCalledWith(`${path}?show=overdue&order=amount-largest`, {
			state: { invoicesSearch: 'Anne' }
		});

		await orderSelect().selectOptions('Payment due, earliest first');
		await testPage.getByRole('button', { name: 'Apply' }).click();
		expect(goto).toHaveBeenLastCalledWith(`${path}?show=overdue`, { state: { invoicesSearch: 'Anne' } });
	});

	it('holds the order in the select from the address', async () => {
		await setupOrder('?order=sent-oldest', { order: 'sent-oldest' });

		await expect.element(orderSelect()).toHaveValue('sent-oldest');
	});
});

describe('an applied order (#2869)', () => {
	it('draws a tag whose × removes only it, keeping the view and the search', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ ...data, items: [data.items[0]] }));
		await setupOrder('?show=overdue&order=amount-smallest', { view: 'overdue', order: 'amount-smallest' }, 'Anne');

		const tag = testPage.getByRole('link', { name: 'Remove filter: Order: Smallest amount first' });
		await expect.element(tag).toHaveAttribute('href', `${path}?show=overdue`);
		await expect
			.element(testPage.getByRole('link', { name: 'Remove filter: Overdue Invoices' }))
			.toHaveAttribute('href', `${path}?order=amount-smallest`);
		await expect
			.element(testPage.getByRole('link', { name: 'Remove filter: Matching Anne' }))
			.toHaveAttribute('href', `${path}?show=overdue&order=amount-smallest`);
		await expect.element(testPage.getByRole('link', { name: 'Clear filters' })).toHaveAttribute('href', path);

		await tag.click();
		expect(goto).toHaveBeenLastCalledWith(`${path}?show=overdue`, { state: { invoicesSearch: 'Anne' } });
	});

	it('is no tag at all for the view’s own order', async () => {
		await setupOrder();

		await expect.element(testPage.getByRole('list', { name: 'Applied filters' })).not.toBeInTheDocument();
	});

	it('ends the result line on its words', async () => {
		await setupOrder('?order=sent-oldest', { order: 'sent-oldest' });
		await expect.element(testPage.getByRole('status')).toHaveTextContent('Showing 3 Invoices, oldest first.');
	});

	it('marks the sorted column, the view’s own by default', async () => {
		await setupOrder('?order=amount-largest', { order: 'amount-largest' });
		await expect.element(testPage.getByRole('columnheader', { name: 'Money' })).toHaveAttribute('aria-sort', 'descending');
	});

	it('marks Sent descending when no order is named on the whole book', async () => {
		await setupOrder();
		await expect.element(testPage.getByRole('columnheader', { name: 'Sent' })).toHaveAttribute('aria-sort', 'descending');
	});

	it('asks Load more for the same order', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ ...data, items: [{ ...data.items[0], id: 'inv-later', clientName: 'Cleo' }], hasMore: false }));
		await setupOrder('?order=due-latest', { order: 'due-latest', hasMore: true, nextCursor: 'cursor-1', invoiceCount: 4 });

		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect.element(testPage.getByRole('link', { name: 'Cleo' })).toBeVisible();
		expect(apiFetchWithSession).toHaveBeenLastCalledWith(`/api/practices/${practiceId}/invoices?order=due-latest&cursor=cursor-1`);
	});

	it('asks a search for the same order in its POST', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ ...data, items: [data.items[0]] }));
		await setupOrder('?order=due-latest', { order: 'due-latest' }, 'Anne');

		await expect.element(testPage.getByRole('status')).toHaveTextContent('matching "Anne"');
		const call = apiFetchWithSession.mock.calls.at(-1)!;
		expect(call[0]).toBe(listPath);
		expect(JSON.parse((call[1] as RequestInit).body as string)).toEqual({ view: 'all', search: 'Anne', cursor: '', order: 'due-latest' });
	});

	it('keeps the order on the link an empty list offers', async () => {
		await setupOrder('?show=overdue&order=sent-oldest', { view: 'overdue', order: 'sent-oldest', items: [], overdueCount: 0 });

		await expect
			.element(testPage.getByRole('link', { name: 'Show all Invoices' }))
			.toHaveAttribute('href', `${path}?order=sent-oldest`);
	});

	it('reads an unknown order in the address as the view’s own', async () => {
		await setupOrder('?order=bogus');

		await expect.element(orderSelect()).toHaveValue('');
		await expect.element(testPage.getByRole('list', { name: 'Applied filters' })).not.toBeInTheDocument();
	});
});

const applyBox = () => testPage.getByRole('button', { name: 'Apply' }).element().getBoundingClientRect();
const spot = (box: DOMRect) => `${Math.round(box.left)},${Math.round(box.top)},${Math.round(box.width)}`;

describe('the Order select’s place (#2869)', () => {
	it.each([1280, 1024, 375])('holds Apply and Order where they were under every order and view, at %ipx', async (width) => {
		const spots: string[] = [];
		for (const [query, view] of [
			['', 'all'],
			['?show=unpaid', 'unpaid'],
			['?show=overdue', 'overdue']
		] as const) {
			pageState.url = new URL(`${fixture.url}${query}`);
			pageState.state = {};
			await testPage.viewport(width, 900);
			await render(Page, { data: { ...data, view } });
			if (width < 672) await testPage.getByText('Order', { exact: true }).first().click();

			for (const label of optionLabels()) {
				await orderSelect().selectOptions(label!);
				spots.push([spot(applyBox()), spot(orderSelect().element().getBoundingClientRect())].join(' '));
			}
			cleanup();
		}

		expect(new Set(spots).size, `Apply and Order by order and view: ${[...new Set(spots)].join(' | ')}`).toBe(1);
	});

	it('puts the first Invoice whole on a 375px first screen, with Order folded beside Apply', async () => {
		await setupOrder();
		await testPage.viewport(375, 740);

		const apply = applyBox();
		const summary = testPage.getByText('Order', { exact: true }).first().element().getBoundingClientRect();
		expect(Math.abs(apply.top - summary.top), 'the summary shares Apply’s line').toBeLessThanOrEqual(2);
		const first = document.querySelector('.invoices-table .record')!.getBoundingClientRect();
		expect(first.bottom, `first card ${Math.round(first.top)}-${Math.round(first.bottom)}`).toBeLessThanOrEqual(740);
	});
});
