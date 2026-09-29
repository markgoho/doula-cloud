import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { toApiResponder, toPageState } from '../../../../../../routeFixture.js';
import { asByHand, asPaidAndReturned, byHandInvoice, fixture, invoice } from './page.fixture.js';
import Page from './+page.svelte';

/*
 * One Client-portal Invoice (#1564). Each branch is the fixture's own
 * variant, so this spec and the continuum sweep describe the same screens.
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

async function setupInvoice(respond: (path: string) => Promise<Response> | Response = toApiResponder(fixture)) {
	apiFetchWithSession.mockReset();
	apiFetchWithSession.mockImplementation(respond);
	await render(Page);
}

describe('Client-portal Invoice (#1564)', () => {
	it("names her Invoice by its number and states its facts in the register's words", async () => {
		await setupInvoice();

		await expect
			.element(page.getByRole('heading', { level: 1, name: `Invoice ${invoice.reference}` }))
			.toBeVisible();
		await expect.element(page.getByText('$3,350.00')).toBeVisible();
		await expect.element(page.getByText('Not yet paid')).toBeVisible();
		expect(apiFetchWithSession).toHaveBeenCalledWith('/api/portal/engagements/engagement-1/invoices/invoice-1');
	});

	it('marks a place for the Payment Element on an open Stripe Invoice, with nothing to press yet', async () => {
		await setupInvoice();

		await expect.element(page.getByText('Not yet paid')).toBeVisible();
		// The mount point is empty markup with no role and no name, so it
		// is the one fact about the document with no accessible signal.
		expect(document.querySelectorAll('[data-payment-element-mount]')).toHaveLength(1);
		expect(page.getByRole('button').elements()).toHaveLength(0);
	});

	it('shows the Invoice-number sentence and nothing pay-like on the by-hand rail', async () => {
		await setupInvoice(asByHand.respond);

		await expect
			.element(
				page.getByText(
					`Your Practice collects this Invoice directly. Quote Invoice ${byHandInvoice.reference} when you pay.`
				)
			)
			.toBeVisible();
		expect(document.querySelectorAll('[data-payment-element-mount]')).toHaveLength(0);
		expect(page.getByRole('button').elements()).toHaveLength(0);
	});

	it('shows a paid, partly returned Invoice with its method and no pay area', async () => {
		await setupInvoice(asPaidAndReturned.respond);

		await expect.element(page.getByText('Paid — $250.00 returned to you')).toBeVisible();
		await expect.element(page.getByText('How you paid')).toBeVisible();
		await expect.element(page.getByText('Bank transfer')).toBeVisible();
		expect(document.querySelectorAll('[data-payment-element-mount]')).toHaveLength(0);
		expect(page.getByText(/Quote Invoice/).elements()).toHaveLength(0);
	});

	it('goes back to the list', async () => {
		await setupInvoice();

		await expect
			.element(page.getByRole('link', { name: 'Back to Invoices' }))
			.toHaveAttribute('href', '/portal/engagements/engagement-1/invoices');
	});

	it('says so when the Invoice cannot be read', async () => {
		await setupInvoice(() => Promise.resolve({ ok: false, text: () => Promise.resolve('nope') } as Response));

		await expect.element(page.getByText('nope')).toBeVisible();
	});

	it('falls back to a generic sentence when the failure is not an Error', async () => {
		await setupInvoice(() => Promise.reject('nope'));

		await expect.element(page.getByText('Failed to load Invoice')).toBeVisible();
	});
});
