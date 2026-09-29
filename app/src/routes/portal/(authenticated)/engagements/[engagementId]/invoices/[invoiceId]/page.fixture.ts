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
	respond: () => jsonResponse(invoice),
	readyText: 'Invoice INV-2027-Persephone-Ochieng-Whitfield-0011',
	variants: [asByHand, asPaidAndReturned]
};
