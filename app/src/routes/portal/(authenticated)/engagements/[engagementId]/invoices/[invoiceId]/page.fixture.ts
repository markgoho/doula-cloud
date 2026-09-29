/*
 * One Client-portal Invoice, as the continuum check sees it (#1564).
 *
 * Three trees, because the page branches three ways and only the base
 * would otherwise be swept: an open Stripe-rail Invoice (the marked
 * place for #1020's Payment Element), an open by-hand Invoice (the
 * Invoice-number sentence and nothing to press), and a paid, partly
 * returned by-hand Invoice (every optional fact present at once). The
 * by-hand and paid ones are variants that restate `respond`, since the
 * Invoice itself is what differs. The reference is a long unbreakable
 * value a Practice could type (#537).
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { ClientInvoice } from '#lib/clientInvoice.js';
import type { ClientPayment } from '#lib/clientPayment.js';
import type { RouteFixture, RouteVariant } from '../../../../../../routeFixture.js';
import Page from './+page.svelte';

export const practiceName = 'Riverside Doula Collective';

export const invoice: ClientInvoice = {
	id: 'invoice-1',
	status: 'open',
	amountCents: 335_000,
	currency: 'usd',
	reference: 'INV-2027-Persephone-Ochieng-Whitfield-0011',
	billingMode: 'stripe',
	createdAt: '2027-02-01T15:00:00Z',
	refundedCents: 0
};

export const payment: ClientPayment = {
	clientSecret: 'pi_fixture_secret_fixture',
	stripeAccountId: 'acct_fixture',
	publishableKey: 'pk_test_fixture'
};

/*
 * The Stripe-rail base tree answers the pay-secret read with the server's
 * own refusal rather than a secret, on purpose: with a secret the route
 * would load Stripe.js from js.stripe.com, and a layout sweep that waits on
 * a third party's script measures the network, not the screen. The Element
 * is Stripe's own iframe either way -- what the sweep can measure is our
 * container and the sentence, and the spec drives the mounted path with a
 * Stripe double.
 */
export const paymentRefusal = 'Your Practice cannot take this payment yet.';

export const byHandInvoice: ClientInvoice = { ...invoice, billingMode: 'by_hand' };

export const paidReturnedInvoice: ClientInvoice = {
	...invoice,
	status: 'paid',
	billingMode: 'by_hand',
	paidAt: '2027-02-04T15:00:00Z',
	paidMethod: 'bank_transfer',
	refundedCents: 25_000
};

export const asByHand: RouteVariant = {
	name: 'The Client-portal Invoice, by-hand rail',
	respond: () => jsonResponse(byHandInvoice)
};

export const asPaidAndReturned: RouteVariant = {
	name: 'The Client-portal Invoice, paid and partly returned',
	respond: () => jsonResponse(paidReturnedInvoice)
};

export const fixture: RouteFixture = {
	name: 'The Client-portal Invoice, Stripe rail',
	component: Page,
	params: { engagementId: 'engagement-1', invoiceId: 'invoice-1' },
	url: 'https://example.test/portal/engagements/engagement-1/invoices/invoice-1',
	pageData: { practiceName },
	respond: (path) =>
		path.endsWith('/payment') ? new Response(paymentRefusal, { status: 409 }) : jsonResponse(invoice),
	readyText: 'Invoice INV-2027-Persephone-Ochieng-Whitfield-0011',
	variants: [asByHand, asPaidAndReturned]
};
