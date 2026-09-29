/*
 * The Client's Invoices, as the continuum check sees it (#1564).
 *
 * Every section renders, and each renders differently, so the row set
 * holds every state a field reads differently (ADR-0025): an open Stripe
 * Invoice with the busiest amount, a paid by-hand Invoice that was
 * partly returned to her and carries the longest reference a Practice
 * could type (#537's hostile value), a paid Stripe Invoice with nothing
 * returned, and a void one so the "No longer owed" section is swept too.
 * The settled-state sentences and the refund sentence are the spec's,
 * where the content departs from this one.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { ClientInvoice, ClientInvoicesPage } from '#lib/clientInvoice.js';
import type { RouteFixture } from '../../../../../routeFixture.js';
import Page from './+page.svelte';

export const practiceName = 'Riverside Doula Collective';

export const openStripe: ClientInvoice = {
	id: 'invoice-open',
	status: 'open',
	amountCents: 123_456_789,
	currency: 'usd',
	reference: 'A4B2-0011',
	billingMode: 'stripe',
	createdAt: '2027-02-01T15:00:00Z',
	refundedCents: 0
};

export const paidByHandReturned: ClientInvoice = {
	id: 'invoice-paid-returned',
	status: 'paid',
	amountCents: 90_000,
	currency: 'usd',
	reference: 'INV-2027-Persephone-Ochieng-Whitfield-0007',
	billingMode: 'by_hand',
	createdAt: '2026-11-02T15:00:00Z',
	paidAt: '2026-11-04T15:00:00Z',
	paidMethod: 'bank_transfer',
	refundedCents: 25_000
};

export const paidStripe: ClientInvoice = {
	id: 'invoice-paid',
	status: 'paid',
	amountCents: 45_000,
	currency: 'usd',
	reference: 'A4B2-0009',
	billingMode: 'stripe',
	createdAt: '2026-10-02T15:00:00Z',
	paidAt: '2026-10-03T15:00:00Z',
	refundedCents: 0
};

export const voided: ClientInvoice = {
	id: 'invoice-void',
	status: 'void',
	amountCents: 12_000,
	currency: 'usd',
	reference: 'A4B2-0008',
	billingMode: 'stripe',
	createdAt: '2026-09-02T15:00:00Z',
	refundedCents: 0
};

export const data: ClientInvoicesPage = {
	items: [openStripe, paidByHandReturned, paidStripe, voided],
	hasMore: false,
	totalToPayCents: openStripe.amountCents,
	totalToPayCount: 1
};

export const fixture: RouteFixture = {
	name: 'The Client-portal Invoices',
	component: Page,
	params: { engagementId: 'engagement-1' },
	url: 'https://example.test/portal/engagements/engagement-1/invoices',
	pageData: { practiceName },
	respond: () => jsonResponse(data),
	readyText: 'Invoices'
};
