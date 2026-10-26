/*
 * Getting paid's payment terms question (#2046), as the continuum check
 * sees it. A Practice's own three-digit terms, the widest answer the box
 * is sized for (#1216).
 *
 * Two trees: an Owner or an Admin is asked the question, and anyone else
 * reads the terms as a fact with a Notice saying who may change them. The
 * Admin's tree is the Owner's, so it is not declared again.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { PaymentTerms } from '#lib/invoice.js';
import { practiceSession, type RouteFixture, type RouteVariant } from '../../../../../routeFixture.js';
import Page from './+page.svelte';

export const paymentTerms: PaymentTerms = { netDays: 120, isDefault: false };

export const asDoula: RouteVariant = {
	name: 'The payment terms question, as a Doula',
	pageData: practiceSession(['doula']),
	readyText: 'Payment terms'
};

export const fixture: RouteFixture = {
	name: 'The payment terms question, as an Owner',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/settings/payments/payment-terms',
	pageData: practiceSession(['owner']),
	respond: () => jsonResponse(paymentTerms),
	readyText: 'How many days does a Client have to pay an invoice?',
	variants: [asDoula]
};
