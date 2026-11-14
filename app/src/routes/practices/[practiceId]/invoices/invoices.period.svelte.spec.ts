/*
 * #2871: the Sent period and the Download CSV button, in their own file for
 * the reason the order's is.
 */
import { page as testPage } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render } from 'vitest-browser-svelte';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import '#lib/styles/app.css';
import Page from './InvoicesScreen.svelte';
import { practiceSession, toPageState } from '../../../routeFixture.js';
import { jsonResponse } from '#lib/testResponse.js';
// jscpd:ignore-start -- vi.mock is hoisted per file, so each spec of this screen restates the same module mocks
import { data, fixture, inPeriod } from './page.fixture.js';
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

const path = `/practices/${fixture.params.practiceId}/invoices`;
const settled = () => new Promise((resolve) => setTimeout(resolve, 450));
const october = { from: '2026-10-01', to: '2026-10-31' };

beforeEach(() => {
	apiFetchWithSession.mockReset();
	goto.mockReset();
	goto.mockResolvedValue(undefined);
	Object.assign(pageState, toPageState(fixture));
});

afterEach(() => {
	vi.restoreAllMocks();
});

async function setupPeriod(query = '', landed: Record<string, unknown> = {}) {
	pageState.url = new URL(`${fixture.url}${query}`);
	pageState.state = {};
	await testPage.viewport(1440, 900);
	return render(Page, { data: { ...data, ...landed } });
}

const sentSelect = () => testPage.getByRole('combobox', { name: 'Sent' });

describe('the Sent select (#2871)', () => {
	it('offers the periods and starts on any time', async () => {
		await setupPeriod();

		await expect.element(sentSelect()).toHaveValue('');
		for (const label of ['Any time', 'This month', 'Last month', 'This year', 'Last year', 'Choose days']) {
			await expect.element(sentSelect().getByRole('option', { name: label })).toBeInTheDocument();
		}
	});

	it('applies a preset when changed, with its name in the address', async () => {
		await setupPeriod();

		await sentSelect().selectOptions('Last month');
		await settled();

		expect(goto).toHaveBeenCalledOnce();
		expect(goto).toHaveBeenCalledWith(`${path}?sent=last-month`, expect.objectContaining({ state: { invoicesSearch: '' } }));
	});

	it('opens From and To for Choose days and sends nothing until Apply', async () => {
		await setupPeriod();

		await sentSelect().selectOptions('Choose days');
		await settled();
		expect(goto).not.toHaveBeenCalled();
		await expect.element(testPage.getByText('Both days are included.')).toBeVisible();

		await testPage.getByLabelText('From', { exact: true }).fill('2026-10-03');
		await testPage.getByLabelText('To', { exact: true }).fill('2026-10-17');
		await settled();
		expect(goto).not.toHaveBeenCalled();

		await testPage.getByRole('button', { name: 'Apply' }).click();
		expect(goto).toHaveBeenCalledWith(`${path}?sentFrom=2026-10-03&sentTo=2026-10-17`, { state: { invoicesSearch: '' } });
	});

	it('says two days that are not a range in place and sends nothing', async () => {
		await setupPeriod();
		await sentSelect().selectOptions('Choose days');
		await testPage.getByLabelText('From', { exact: true }).fill('2026-10-17');
		await testPage.getByLabelText('To', { exact: true }).fill('2026-10-03');

		await testPage.getByRole('button', { name: 'Apply' }).click();

		await expect.element(testPage.getByRole('alert')).toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});

	it('says an empty pair in place when another control changes while Choose days is open', async () => {
		await setupPeriod('?sentFrom=2026-10-03&sentTo=2026-10-17');
		await testPage.getByLabelText('From', { exact: true }).fill('');

		await testPage.getByRole('combobox', { name: 'Order' }).selectOptions('Largest amount first');
		await settled();

		await expect.element(testPage.getByRole('alert')).toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});

	it('sends chosen days with another control that changes', async () => {
		await setupPeriod('?sentFrom=2026-10-03&sentTo=2026-10-17');

		await testPage.getByRole('radio', { name: 'Unpaid' }).click();
		await settled();

		expect(goto).toHaveBeenCalledWith(`${path}?show=unpaid&sentFrom=2026-10-03&sentTo=2026-10-17`, expect.anything());
	});

	it('seeds the select and its days from the address', async () => {
		await setupPeriod('?sentFrom=2026-10-03&sentTo=2026-10-17', { period: { days: { from: '2026-10-03', to: '2026-10-17' } }, days: { from: '2026-10-03', to: '2026-10-17' } });

		await expect.element(sentSelect()).toHaveValue('choose');
		await expect.element(testPage.getByLabelText('From', { exact: true })).toHaveValue('2026-10-03');
		await expect.element(testPage.getByLabelText('To', { exact: true })).toHaveValue('2026-10-17');
	});
});

describe('a period applied (#2871)', () => {
	it('draws its tag in fixed words, and removing it keeps the view', async () => {
		pageState.url = new URL(inPeriod.url!);
		pageState.state = {};
		await testPage.viewport(1440, 900);
		await render(Page, inPeriod.props);

		await expect.element(testPage.getByRole('link', { name: 'Remove filter: Sent Aug 1 to Nov 30, 2026' })).toHaveAttribute('href', `${path}?show=unpaid`);
		await expect.element(testPage.getByRole('link', { name: 'Remove filter: Unpaid Invoices' })).toHaveAttribute('href', `${path}?sentFrom=2026-08-01&sentTo=2026-11-30`);
	});

	it('names a whole month, and the totals say whose figures they are', async () => {
		await setupPeriod('?sentFrom=2026-10-01&sentTo=2026-10-31', { period: { days: october }, days: october });

		await expect.element(testPage.getByRole('link', { name: 'Remove filter: Sent in October 2026' })).toBeVisible();
		await expect.element(testPage.getByRole('status')).toHaveTextContent('Invoices sent in October 2026');
		const note = testPage.getByText('Totals for Invoices sent in October 2026.');
		await expect.element(note).toBeVisible();
		for (const list of testPage.getByRole('definition').elements()) {
			expect(list.closest('dl')?.getAttribute('aria-describedby')).toBe(note.element().id);
		}
	});

	it('draws no line over the totals with no period', async () => {
		await setupPeriod();

		await expect.element(testPage.getByText(/^Totals for Invoices sent/)).not.toBeInTheDocument();
	});

	it('asks the BFF for the two days, in a search and for a later page', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ ...data, items: [data.items[0]], hasMore: false }));
		pageState.url = new URL(`${fixture.url}?sent=last-month`);
		pageState.state = { invoicesSearch: 'Anne' };
		await testPage.viewport(1440, 900);
		await render(Page, { data: { ...data, period: { preset: 'last-month' }, days: october } });

		await expect.element(testPage.getByRole('status')).toHaveTextContent('matching "Anne"');
		const body = JSON.parse((apiFetchWithSession.mock.calls.at(-1)![1] as RequestInit).body as string);
		expect(body).toMatchObject({ search: 'Anne', sentFrom: expect.stringMatching(/^\d{4}-\d{2}-01$/), sentTo: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/) });
	});

	it('offers the list without the period when it finds nothing', async () => {
		await setupPeriod('?show=unpaid&sentFrom=2026-10-01&sentTo=2026-10-31', {
			view: 'unpaid',
			items: [],
			period: { days: october },
			days: october
		});

		await expect.element(testPage.getByRole('status')).toHaveTextContent('No unpaid Invoices sent in October 2026.');
		await expect.element(testPage.getByRole('link', { name: 'Show Invoices sent any time' })).toHaveAttribute('href', `${path}?show=unpaid`);
		await expect.element(testPage.getByRole('button', { name: 'Apply' })).toBeVisible();
	});

	it('keeps the bar when a period leaves a whole-book view empty', async () => {
		await setupPeriod('?sentFrom=2026-10-01&sentTo=2026-10-31', { items: [], period: { days: october }, days: october });

		await expect.element(testPage.getByRole('button', { name: 'Apply' })).toBeVisible();
		await expect.element(testPage.getByText('No Invoices yet.')).not.toBeInTheDocument();
	});
});

describe('the Download CSV button (#2871)', () => {
	async function download(response: Response) {
		apiFetchWithSession.mockResolvedValue(response);
		const create = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:x');
		vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {});
		const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
		await testPage.getByRole('button', { name: 'Download CSV' }).click();
		return { create, click };
	}

	it('says what it downloads, using the result line’s count', async () => {
		await setupPeriod('', { invoiceCount: 35, hasMore: true, nextCursor: 'c' });

		await expect.element(testPage.getByRole('button', { name: 'Download CSV' })).toHaveAccessibleDescription('A spreadsheet of the 35 Invoices this list shows.');
	});

	it('counts the rows on screen when no more are to come', async () => {
		await setupPeriod('', { hasMore: false, items: [data.items[0]] });

		await expect.element(testPage.getByRole('button', { name: 'Download CSV' })).toHaveAccessibleDescription('A spreadsheet of the 1 Invoice this list shows.');
	});

	it('says no count for a search with more to come', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ ...data, items: [data.items[0]], hasMore: true, nextCursor: 'c' }));
		pageState.url = new URL(fixture.url);
		pageState.state = { invoicesSearch: 'Anne' };
		await testPage.viewport(1440, 900);
		await render(Page, { data });

		await expect
			.element(testPage.getByRole('button', { name: 'Download CSV' }))
			.toHaveAccessibleDescription('A spreadsheet of every Invoice this list shows.');
	});

	it('posts the applied view, search, order and period and saves the file the server named', async () => {
		pageState.url = new URL(`${fixture.url}?show=unpaid&order=amount-largest&sentFrom=2026-10-01&sentTo=2026-10-31`);
		pageState.state = { invoicesSearch: 'Anne' };
		apiFetchWithSession.mockResolvedValue(jsonResponse({ ...data, items: [data.items[0]], hasMore: true, nextCursor: 'c' }));
		await testPage.viewport(1440, 900);
		await render(Page, { data: { ...data, view: 'unpaid', order: 'amount-largest', period: { days: october }, days: october } });
		await expect.element(testPage.getByRole('status')).toHaveTextContent('matching "Anne"');

		const { create, click } = await download(
			new Response('Client\n', { headers: { 'Content-Disposition': 'attachment; filename="invoices-2026-11-14.csv"' } })
		);

		const [, init] = apiFetchWithSession.mock.calls.at(-1)!;
		expect(apiFetchWithSession.mock.calls.at(-1)![0]).toBe(`/api/practices/${fixture.params.practiceId}/invoices/export`);
		expect(JSON.parse((init as RequestInit).body as string)).toEqual({
			view: 'unpaid',
			search: 'Anne',
			order: 'amount-largest',
			sentFrom: '2026-10-01',
			sentTo: '2026-10-31'
		});
		expect(create).toHaveBeenCalledOnce();
		expect(click).toHaveBeenCalledOnce();
		await expect.element(testPage.getByText('Your Invoices did not download.')).not.toBeInTheDocument();
	});

	it.each([
		[new Response('', { status: 429, headers: { 'Retry-After': '600' } }), 'Try again in 10 minutes.'],
		[new Response('', { status: 403 }), 'Your role cannot download Invoices.'],
		[new Response('', { status: 500 }), 'Your Invoices did not download. Nothing you saved is lost.']
	])('says a refusal in place and keeps the button for another try', async (response, words) => {
		await setupPeriod();

		const { click } = await download(response);

		await expect.element(testPage.getByText(words, { exact: false })).toBeVisible();
		expect(click).not.toHaveBeenCalled();
		await expect.element(testPage.getByRole('button', { name: 'Download CSV' })).toBeVisible();
	});

	it('says an absent connection in place', async () => {
		await setupPeriod();
		apiFetchWithSession.mockRejectedValue(new TypeError('offline'));

		await testPage.getByRole('button', { name: 'Download CSV' }).click();

		await expect.element(testPage.getByText('Your Invoices did not download. Nothing you saved is lost.')).toBeVisible();
	});

	it('is drawn for an Admin and not for a Doula', async () => {
		Object.assign(pageState, toPageState({ ...fixture, pageData: { ...practiceSession(['admin']), clientsCanPay: true } }));
		await setupPeriod();
		await expect.element(testPage.getByRole('button', { name: 'Download CSV' })).toBeVisible();
		cleanup();

		Object.assign(pageState, toPageState({ ...fixture, pageData: { ...practiceSession(['doula']), clientsCanPay: true } }));
		await setupPeriod();
		await expect.element(testPage.getByRole('button', { name: 'Download CSV' })).not.toBeInTheDocument();
	});

	it('is drawn but unavailable while the book is empty, so the title row never moves', async () => {
		await setupPeriod('', { items: [] });

		await expect.element(testPage.getByRole('button', { name: 'Download CSV' })).toHaveAttribute('aria-disabled', 'true');
	});
});
