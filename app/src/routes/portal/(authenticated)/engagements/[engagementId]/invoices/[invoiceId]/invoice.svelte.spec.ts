import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { toApiResponder, toPageState } from '../../../../../../routeFixture.js';
import { asByHand, asPaidAndReturned, byHandInvoice, fixture, invoice, payment, paymentRefusal } from './page.fixture.js';
import Page from './+page.svelte';
// The loading Skeleton reserves space with `var(--text-body-size)`, which
// only exists once the tokens are loaded -- the real app loads them in the
// root layout. See invoices.svelte.spec.ts's identical import.
import '#lib/styles/app.css';

/*
 * One Client-portal Invoice (#1564, #1020). Each branch is the fixture's
 * own variant, so this spec and the continuum sweep describe the same
 * screens. Stripe.js is a double: the real one is a third party's script
 * and a cross-origin iframe.
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

const stripeDouble = vi.hoisted(() => {
	const element = { on: vi.fn(), mount: vi.fn() };
	const elements = { create: vi.fn(() => element) };
	const stripe = { elements: vi.fn(() => elements), confirmPayment: vi.fn() };
	return { element, elements, stripe, loadStripe: vi.fn() };
});
vi.mock('@stripe/stripe-js', () => ({ loadStripe: stripeDouble.loadStripe }));

function jsonResponse(body: unknown) {
	return { ok: true, json: () => Promise.resolve(body) } as Response;
}

async function setupInvoice(respond: (path: string) => Promise<Response> | Response = toApiResponder(fixture)) {
	apiFetchWithSession.mockReset();
	apiFetchWithSession.mockImplementation(respond);
	stripeDouble.loadStripe.mockReset();
	stripeDouble.loadStripe.mockResolvedValue(stripeDouble.stripe);
	stripeDouble.stripe.confirmPayment.mockReset();
	stripeDouble.element.on.mockReset();
	stripeDouble.element.mount.mockReset();
	await render(Page);
}

/**
A Stripe-rail Invoice the server will give a payment secret for.
*/
const payable = (path: string) => jsonResponse(path.endsWith('/payment') ? payment : invoice);

/**
Fires the Element's `ready` event the way Stripe.js does.
*/
async function markElementReady() {
	await vi.waitFor(() => expect(stripeDouble.element.on).toHaveBeenCalled());
	const handler = stripeDouble.element.on.mock.calls.find(([name]) => name === 'ready')![1] as () => void;
	handler();
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
		// Nothing to ask the server for: it would refuse a by-hand Invoice.
		expect(apiFetchWithSession).not.toHaveBeenCalledWith(expect.stringContaining('/payment'));
	});

	it('shows a paid, partly returned Invoice with its method and no pay area', async () => {
		await setupInvoice(asPaidAndReturned.respond);

		await expect.element(page.getByText('Paid — $250.00 returned to you')).toBeVisible();
		await expect.element(page.getByText('How you paid')).toBeVisible();
		await expect.element(page.getByText('Bank transfer')).toBeVisible();
		expect(document.querySelectorAll('[data-payment-element-mount]')).toHaveLength(0);
		expect(page.getByText(/Quote Invoice/).elements()).toHaveLength(0);
		expect(apiFetchWithSession).not.toHaveBeenCalledWith(expect.stringContaining('/payment'));
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

	// #1574: the Invoice has no empty state -- a missing Invoice is a load
	// error -- so its states are loading, error and loaded. Until it arrives
	// the one <h1> is the Invoices heading, the same name the tab carries.
	it('names the screen with one h1 while it loads', async () => {
		await setupInvoice(() => new Promise<Response>(() => {}));

		await expect.element(page.getByRole('status', { name: 'Loading Invoice' })).toBeVisible();
		await expect.element(page.getByRole('heading', { level: 1, name: 'Invoices' })).toBeVisible();
		expect(page.getByRole('heading', { level: 1 }).elements()).toHaveLength(1);
	});

	it('names the screen with one h1 when the Invoice cannot be read', async () => {
		await setupInvoice(() => Promise.reject('nope'));

		await expect.element(page.getByText('Failed to load Invoice')).toBeVisible();
		await expect.element(page.getByRole('heading', { level: 1, name: 'Invoices' })).toBeVisible();
		expect(page.getByRole('heading', { level: 1 }).elements()).toHaveLength(1);
	});

	it('names the screen with one h1 once loaded', async () => {
		await setupInvoice();

		await expect.element(page.getByRole('heading', { level: 1, name: `Invoice ${invoice.reference}` })).toBeVisible();
		expect(page.getByRole('heading', { level: 1 }).elements()).toHaveLength(1);
	});
});

describe('paying a Stripe-rail Invoice in our own chrome (#1020)', () => {
	it("reads the server's refusal, and offers nothing to press, when the Invoice cannot be paid here", async () => {
		await setupInvoice();

		await expect.element(page.getByText(paymentRefusal)).toBeVisible();
		expect(page.getByRole('button').elements()).toHaveLength(0);
		expect(stripeDouble.loadStripe).not.toHaveBeenCalled();
	});

	it('falls back to a retry sentence when the secret cannot be fetched and no reason came back', async () => {
		await setupInvoice((path) => (path.endsWith('/payment') ? Promise.reject('down') : jsonResponse(invoice)));

		await expect.element(page.getByText('We could not start this payment. Try again.')).toBeVisible();
	});

	it("mounts Stripe's Element on the Practice's connected account, with the figure in the button", async () => {
		await setupInvoice(payable);

		await expect.element(page.getByRole('button', { name: 'Pay $3,350.00' })).toBeVisible();
		expect(apiFetchWithSession).toHaveBeenCalledWith('/api/portal/engagements/engagement-1/invoices/invoice-1/payment');
		expect(stripeDouble.loadStripe).toHaveBeenCalledWith(payment.publishableKey, { stripeAccount: payment.stripeAccountId });
		expect(stripeDouble.stripe.elements).toHaveBeenCalledWith({ clientSecret: payment.clientSecret });
		expect(stripeDouble.element.mount).toHaveBeenCalledTimes(1);
		// The mount point is a bare div with no role and no name.
		expect(document.querySelectorAll('[data-payment-element-mount]')).toHaveLength(1);
	});

	it('cannot be pressed until the Element is ready', async () => {
		await setupInvoice(payable);
		const pay = page.getByRole('button', { name: 'Pay $3,350.00' });

		await expect.element(pay).toBeDisabled();
		await markElementReady();

		await expect.element(pay).toBeEnabled();
	});

	it('confirms the payment and says only that it was sent, never that the Invoice is paid', async () => {
		await setupInvoice(payable);
		stripeDouble.stripe.confirmPayment.mockResolvedValue({});
		await expect.element(page.getByRole('button', { name: 'Pay $3,350.00' })).toBeVisible();
		await markElementReady();

		await page.getByRole('button', { name: 'Pay $3,350.00' }).click();

		await expect
			.element(page.getByText('Your payment was sent. This Invoice will show as Paid once it is confirmed.'))
			.toBeVisible();
		expect(stripeDouble.stripe.confirmPayment).toHaveBeenCalledWith({
			elements: stripeDouble.elements,
			confirmParams: { return_url: fixture.url },
			redirect: 'if_required'
		});
		expect(page.getByRole('button', { name: /Pay/ }).elements()).toHaveLength(0);
		// The label is the model's, and it has not changed yet.
		await expect.element(page.getByText('Not yet paid')).toBeVisible();
	});

	it("shows Stripe's own message when the payment is declined, and lets her try again", async () => {
		await setupInvoice(payable);
		stripeDouble.stripe.confirmPayment.mockResolvedValue({ error: { message: 'Your card was declined.' } });
		await markElementReady();

		await page.getByRole('button', { name: 'Pay $3,350.00' }).click();

		await expect.element(page.getByText('Your card was declined.')).toBeVisible();
		await expect.element(page.getByRole('button', { name: 'Pay $3,350.00' })).toBeEnabled();
	});

	it('falls back to a sentence when a declined payment carries no message', async () => {
		await setupInvoice(payable);
		stripeDouble.stripe.confirmPayment.mockResolvedValue({ error: {} });
		await markElementReady();

		await page.getByRole('button', { name: 'Pay $3,350.00' }).click();

		await expect.element(page.getByText('Your payment did not go through. Try again.')).toBeVisible();
	});

	it('says so when confirming fails outright', async () => {
		await setupInvoice(payable);
		stripeDouble.stripe.confirmPayment.mockRejectedValue(new Error('network'));
		await markElementReady();

		await page.getByRole('button', { name: 'Pay $3,350.00' }).click();

		await expect.element(page.getByText('Your payment did not go through. Try again.')).toBeVisible();
	});

	it('says so when Stripe.js cannot be loaded', async () => {
		apiFetchWithSession.mockReset();
		apiFetchWithSession.mockImplementation(payable);
		stripeDouble.loadStripe.mockReset();
		stripeDouble.loadStripe.mockRejectedValue(new Error('blocked'));
		await render(Page);

		await expect.element(page.getByText('We could not load the payment form. Try again.')).toBeVisible();
	});

	it('says so when Stripe.js loads nothing', async () => {
		apiFetchWithSession.mockReset();
		apiFetchWithSession.mockImplementation(payable);
		stripeDouble.loadStripe.mockReset();
		stripeDouble.loadStripe.mockResolvedValue(undefined);
		await render(Page);

		await expect.element(page.getByText('We could not load the payment form. Try again.')).toBeVisible();
	});
});
