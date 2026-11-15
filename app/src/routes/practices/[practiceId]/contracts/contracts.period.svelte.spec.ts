/*
 * #2954: the Sent period and the Download CSV button, in their own file for
 * the reason the Invoices list's are (#2871).
 */
import { page as testPage } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render } from 'vitest-browser-svelte';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import '#lib/styles/app.css';
import Page from './ContractsScreen.svelte';
import { toPageState } from '../../../routeFixture.js';
import { data, doulas, fixture, items, queue } from './page.fixture.js';
import { pageState } from '#testing/routePage.js';
import { goto } from '$app/navigation';

// jscpd:ignore-start -- vi.mock is hoisted per file, so each spec of this screen restates the same module mocks
vi.mock('$app/state', () => import('#testing/routePage.js'));
if (!customElements.get('center-l')) registerLayoutPrimitives();
const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));
vi.mock('$app/navigation', () => ({
	invalidate: vi.fn(),
	goto: vi.fn(),
	afterNavigate: vi.fn(),
	beforeNavigate: vi.fn(),
	replaceState: vi.fn(),
	snapshot: vi.fn()
}));
// jscpd:ignore-end

const path = '/practices/practice-1/contracts';
const october = { from: '2026-10-01', to: '2026-10-31' };
const settled = () => new Promise((resolve) => setTimeout(resolve, 450));
const sessionOf = (roles: string[]) => ({ session: { ...(fixture.pageData as { session: object }).session, roles } });
const ownerProperties = { data, isQueueExpected: true, queue, doulas, isRosterLanded: true };

async function setup(query = '?show=all', landed: Record<string, unknown> = {}, roles = ['owner'], state: object = {}) {
	Object.assign(pageState, toPageState(fixture));
	pageState.url = new URL(`https://example.test${path}${query}`);
	pageState.data = sessionOf(roles) as Record<string, unknown>;
	pageState.state = state;
	await testPage.viewport(1440, 900);
	return render(Page, { ...ownerProperties, data: { ...data, ...landed } });
}

const sentSelect = () => testPage.getByRole('combobox', { name: 'Sent' });
const downloadButton = () => testPage.getByRole('button', { name: 'Download CSV' });

beforeEach(() => {
	apiFetchWithSession.mockReset();
	vi.mocked(goto).mockReset();
	vi.mocked(goto).mockResolvedValue(undefined);
});

afterEach(() => {
	vi.restoreAllMocks();
});

describe('the Sent select (#2954)', () => {
	// jscpd:ignore-start -- the same control as the Invoices list's, so the same assertion
	it('offers the Invoices list’s periods and starts on any time', async () => {
		await setup();

		await expect.element(sentSelect()).toHaveValue('');
		for (const label of ['Any time', 'This month', 'Last month', 'This year', 'Last year', 'Choose days']) {
			await expect.element(sentSelect().getByRole('option', { name: label })).toBeInTheDocument();
		}
	});
	// jscpd:ignore-end

	it('applies a preset when changed, with its name in the address', async () => {
		await setup();

		await sentSelect().selectOptions('Last month');
		await settled();

		expect(goto).toHaveBeenCalledWith(`${path}?show=all&sent=last-month`, expect.objectContaining({ state: { contractsSearch: '' } }));
	});

	it('sends chosen days only on Apply, and says two days that are not a range in place', async () => {
		await setup();
		await sentSelect().selectOptions('Choose days');
		await testPage.getByLabelText('From', { exact: true }).fill('2026-10-17');
		await testPage.getByLabelText('To', { exact: true }).fill('2026-10-03');
		await settled();
		expect(goto).not.toHaveBeenCalled();

		await testPage.getByRole('button', { name: 'Apply' }).click();
		await expect.element(testPage.getByRole('alert')).toBeVisible();
		expect(goto).not.toHaveBeenCalled();

		await testPage.getByLabelText('From', { exact: true }).fill('2026-10-03');
		await testPage.getByLabelText('To', { exact: true }).fill('2026-10-17');
		await testPage.getByRole('button', { name: 'Apply' }).click();
		expect(goto).toHaveBeenCalledWith(`${path}?show=all&sentFrom=2026-10-03&sentTo=2026-10-17`, { state: { contractsSearch: '' } });
	});

	it('sends chosen days with another control that changes, and not while they are wrong', async () => {
		await setup('?show=all&sentFrom=2026-10-03&sentTo=2026-10-17');
		await testPage.getByRole('radio', { name: 'Signed', exact: true }).click();
		await settled();
		expect(goto).toHaveBeenCalledWith(`${path}?show=signed&sentFrom=2026-10-03&sentTo=2026-10-17`, expect.anything());

		vi.mocked(goto).mockClear();
		await testPage.getByLabelText('From', { exact: true }).fill('');
		await testPage.getByRole('radio', { name: 'Voided', exact: true }).click();
		await settled();
		expect(goto).not.toHaveBeenCalled();
	});

	it('is not offered in the Draft view, which holds nothing that was sent', async () => {
		await setup('?show=draft', { view: 'draft' });

		await expect.element(sentSelect()).not.toBeInTheDocument();
	});
});

describe('a period applied (#2954)', () => {
	it('draws its tag in fixed words, and removing it keeps the view', async () => {
		await setup('?show=signed&sentFrom=2026-10-01&sentTo=2026-10-31', { view: 'signed', period: { days: october }, days: october });

		await expect.element(testPage.getByRole('link', { name: 'Remove filter: Sent in October 2026' })).toHaveAttribute('href', `${path}?show=signed`);
		await expect.element(testPage.getByRole('status')).toHaveTextContent('sent in October 2026');
	});

	it('offers the list without the period when it finds nothing', async () => {
		await setup('?show=signed&sentFrom=2026-10-01&sentTo=2026-10-31', { view: 'signed', items: [], total: 0, period: { days: october }, days: october });

		await expect.element(testPage.getByRole('link', { name: 'Show Contracts sent any time' })).toHaveAttribute('href', `${path}?show=signed`);
		await expect.element(testPage.getByRole('button', { name: 'Apply' })).toBeVisible();
	});

	it('keeps the bar when a period leaves the whole book empty', async () => {
		await setup('?show=all&sentFrom=2026-10-01&sentTo=2026-10-31', { items: [], total: 0, period: { days: october }, days: october });

		await expect.element(testPage.getByRole('button', { name: 'Apply' })).toBeVisible();
	});

	it('pages under the same days the first page was read under', async () => {
		apiFetchWithSession.mockResolvedValue(Response.json({ items: [], hasMore: false, total: 0 }, { headers: { 'Content-Type': 'application/json' } }));
		await setup('?show=signed&sentFrom=2026-10-01&sentTo=2026-10-31', {
			view: 'signed',
			hasMore: true,
			nextCursor: 'c1',
			period: { days: october },
			days: october
		});

		await testPage.getByRole('button', { name: /load more/i }).first().click();

		await vi.waitFor(() => expect(apiFetchWithSession).toHaveBeenCalled());
		expect(apiFetchWithSession.mock.calls[0]![0]).toContain('sentFrom=2026-10-01&sentTo=2026-10-31');
	});
});

describe('the Download CSV button (#2954)', () => {
	async function download(response: Response) {
		apiFetchWithSession.mockResolvedValue(response);
		vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:x');
		vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {});
		const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
		await downloadButton().click();
		return click;
	}

	it('posts the applied view, search, order, Mine, Doula and period and saves the file the server named', async () => {
		await setup(
			'?show=signed&order=client&doula=staff-2&sentFrom=2026-10-01&sentTo=2026-10-31',
			{ view: 'signed', order: 'client', doula: 'staff-2', period: { days: october }, days: october, hasMore: true, total: 35 }
		);

		const click = await download(
			new Response('Client\n', { headers: { 'Content-Disposition': 'attachment; filename="contracts-2026-11-14.csv"', 'Contract-Count': '35' } })
		);

		const [url, init] = apiFetchWithSession.mock.calls.at(-1)!;
		expect(url).toBe('/api/practices/practice-1/contracts/export');
		expect(JSON.parse((init as RequestInit).body as string)).toEqual({
			view: 'signed',
			search: '',
			mine: false,
			doula: 'staff-2',
			order: 'client',
			sentFrom: '2026-10-01',
			sentTo: '2026-10-31'
		});
		expect(click).toHaveBeenCalledOnce();
		await expect.element(testPage.getByText('Downloaded contracts-2026-11-14.csv, 35 Contracts.')).toBeVisible();
	});

	it('clears its words when the list changes', async () => {
		const view = await setup();
		await download(
			new Response('Client\n', { headers: { 'Content-Disposition': 'attachment; filename="contracts-2026-11-14.csv"', 'Contract-Count': '1' } })
		);
		await expect.element(testPage.getByText('Downloaded contracts-2026-11-14.csv, 1 Contract.')).toBeVisible();

		pageState.url.searchParams.set('show', 'signed');
		await view.rerender({ ...ownerProperties, data: { ...data, view: 'signed' } });
		await expect.element(testPage.getByText('Downloaded contracts-2026-11-14.csv', { exact: false })).not.toBeInTheDocument();
	});

	it('says what it downloads, counting the whole list while more are to come', async () => {
		await setup('?show=all', { hasMore: true, nextCursor: 'c', total: 35 });
		await expect.element(downloadButton()).toHaveAccessibleDescription('A spreadsheet of the 35 Contracts this list shows.');
	});

	it('counts the rows on screen when no more are to come', async () => {
		await setup('?show=all', { items: [items[0]], total: 1 });
		await expect.element(downloadButton()).toHaveAccessibleDescription('A spreadsheet of the 1 Contract this list shows.');
	});

	it.each([
		[new Response('', { status: 429, headers: { 'Retry-After': '600' } }), 'Try again in 10 minutes.'],
		[new Response('', { status: 403 }), 'Your role cannot download Contracts.'],
		[new Response('', { status: 500 }), 'Your Contracts did not download. Nothing you saved is lost.']
	])('says a refusal in place and keeps the button for another try', async (response, words) => {
		await setup();

		const click = await download(response);

		await expect.element(testPage.getByText(words, { exact: false })).toBeVisible();
		expect(click).not.toHaveBeenCalled();
		await expect.element(downloadButton()).toBeVisible();
	});

	it('is drawn for an Admin and for neither a Doula nor a contractor', async () => {
		await setup('?show=all', {}, ['admin']);
		await expect.element(downloadButton()).toBeVisible();
		cleanup();

		await setup('?show=all', {}, ['doula']);
		await expect.element(downloadButton()).not.toBeInTheDocument();
	});

	it('is drawn but unavailable while the list is empty, so the title row never moves', async () => {
		await setup('?show=signed', { view: 'signed', items: [], total: 0 });
		await expect.element(downloadButton()).toHaveAttribute('aria-disabled', 'true');
	});
});
