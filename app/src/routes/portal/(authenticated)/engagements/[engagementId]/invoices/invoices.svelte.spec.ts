import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import '#lib/styles/app.css';
import { toApiResponder, toPageState } from '../../../../../routeFixture.js';
import { data, fixture, openStripe, paidByHandReturned, paidStripe, voided } from './page.fixture.js';
import Page from './+page.svelte';
if (!customElements.get('center-l')) registerLayoutPrimitives();

/*
 * The Client's Invoices (#1564). The fixture holds every state at once;
 * a state it does not hold is a spread of `data`, never a second object.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetchWithSession,
	apiErrorMessage: (response: Response) => response.text()
}));

function jsonResponse(body: unknown) {
	return { ok: true, json: () => Promise.resolve(body) } as Response;
}

async function setupInvoices(respond: (path: string) => Promise<Response> | Response = toApiResponder(fixture)) {
	apiFetchWithSession.mockReset();
	apiFetchWithSession.mockImplementation(respond);
	await render(Page);
}

/**
A page of the list departing from the fixture's own.
*/
const pageOf = (items: unknown[], extra: Record<string, unknown> = {}) =>
	jsonResponse({ ...data, items, totalToPayCents: 0, totalToPayCount: 0, ...extra });

describe('Client-portal Invoices (#1564)', () => {
	it("heads each section with the register's own question", async () => {
		await setupInvoices();

		await expect.element(page.getByRole('heading', { level: 1, name: 'Invoices' })).toBeVisible();
		await expect.element(page.getByRole('heading', { name: 'What you still owe' })).toBeVisible();
		await expect.element(page.getByRole('heading', { name: 'What you have paid' })).toBeVisible();
		await expect.element(page.getByRole('heading', { name: 'No longer owed' })).toBeVisible();
	});

	it("writes the server's total, not a sum of the page", async () => {
		await setupInvoices(() => pageOf([openStripe], { totalToPayCents: 77_700 }));

		await expect.element(page.getByText('Total to pay')).toBeVisible();
		await expect.element(page.getByText('$777.00')).toBeVisible();
	});

	it('gives each Invoice one fixed label, and a returned one its amount', async () => {
		await setupInvoices();

		await expect.element(page.getByText('Not yet paid').last()).toBeVisible();
		await expect.element(page.getByText('Paid', { exact: true }).last()).toBeVisible();
		await expect.element(page.getByText('Paid — $250.00 returned to you').last()).toBeVisible();
		await expect.element(page.getByText('No longer owed', { exact: true }).last()).toBeVisible();
	});

	it('links each row to its own page by Invoice number', async () => {
		await setupInvoices();

		await expect
			.element(page.getByRole('link', { name: paidStripe.reference }))
			.toHaveAttribute('href', `/portal/engagements/engagement-1/invoices/${paidStripe.id}`);
	});

	it('says every Invoice is paid, and how much came back, when nothing is owed', async () => {
		await setupInvoices(() => pageOf([paidByHandReturned, paidStripe]));

		await expect
			.element(page.getByText('You have paid every Invoice for this care. $250.00 of it was returned to you.').last())
			.toBeVisible();
	});

	it('leaves out the returned sentence when nothing went back', async () => {
		await setupInvoices(() => pageOf([paidStripe]));

		await expect.element(page.getByText('You have paid every Invoice for this care.').last()).toBeVisible();
		expect(page.getByText(/returned to you/).elements()).toHaveLength(0);
	});

	it('renders both empty sections when there are no Invoices at all', async () => {
		await setupInvoices(() => pageOf([]));

		await expect.element(page.getByText('There are no Invoices for this care.').last()).toBeVisible();
		await expect.element(page.getByText('There are no Payments for this care.').last()).toBeVisible();
		await expect.element(page.getByRole('heading', { name: 'No longer owed' })).not.toBeInTheDocument();
	});

	it('says there are no Payments while she still owes and has paid nothing', async () => {
		await setupInvoices(() => pageOf([openStripe], { totalToPayCents: openStripe.amountCents }));

		await expect.element(page.getByText('There are no Payments for this care.').last()).toBeVisible();
	});

	it('claims no sentence when every Invoice is void, and shows no "balance" anywhere', async () => {
		await setupInvoices(() => pageOf([voided]));

		await expect.element(page.getByRole('heading', { name: 'No longer owed' })).toBeVisible();
		expect(page.getByText(/every Invoice|no Invoices/i).elements()).toHaveLength(0);
		expect(document.body.textContent).not.toMatch(/balance/i);
	});

	it('says so when the Invoices cannot be read', async () => {
		await setupInvoices(() => Promise.resolve({ ok: false, text: () => Promise.resolve('boom') } as Response));

		await expect.element(page.getByText('boom')).toBeVisible();
	});

	it('falls back to a generic sentence when the failure is not an Error', async () => {
		await setupInvoices(() => Promise.reject('nope'));

		await expect.element(page.getByText('Failed to load Invoices')).toBeVisible();
	});

	it('loads the next page on request', async () => {
		let calls = 0;
		await setupInvoices((path) => {
			calls += 1;
			return path.includes('cursor=')
				? pageOf([paidStripe])
				: pageOf([openStripe], { hasMore: true, nextCursor: 'next', totalToPayCents: openStripe.amountCents });
		});

		await page.getByRole('button', { name: /load more|show more/i }).first().click();

		await expect.element(page.getByRole('link', { name: paidStripe.reference })).toBeVisible();
		expect(calls).toBe(2);
	});
});
