/**
 * What the portal's Payment Element mounts with (#1020): the client secret
 * for one open, Stripe-rail Invoice, the Practice's connected account, and
 * the platform's publishable key.
 *
 * Read fresh every time the Invoice page opens and never kept: the secret
 * is a credential for one payment, and the server refuses to hand it out
 * for an Invoice that is by-hand, not open, or not hers.
 */

import type { Fetcher } from './fetcher.js';

import { parseRefusal } from './formErrors.js';

/**
The code the BFF gives the one refusal that is the Practice's setup, not a
failure (#2039): its Stripe account is not connected or cannot take a card
yet. The Invoice page states it as a fact, with a way to pay another way.
*/
export const CARD_PAYMENTS_NOT_SET_UP = 'CARD_PAYMENTS_NOT_SET_UP';

export interface ClientPayment {
	clientSecret: string;
	/**
	`stripeAccount` for the Stripe.js instance -- an identifier, not a credential.
	*/
	stripeAccountId: string;
	publishableKey: string;
}

/**
What the pay-secret read answers: what the Element mounts with, or the
Practice's setup standing in the way, which is not an error (#2039).
*/
export type ClientPaymentRead = { kind: 'ready'; payment: ClientPayment } | { kind: 'notSetUp' };

/** Answers `notSetUp` for the Practice's own setup, read off the
 * refusal's code and never its words (#692). Throws the server's own
 * sentence for every other refusal ("We could not start this payment. Try
 * again."), which is written for the Client to read. */
export async function loadClientPayment(
	fetcher: Fetcher,
	engagementId: string,
	invoiceId: string
): Promise<ClientPaymentRead> {
	const response = await fetcher(`/api/portal/engagements/${engagementId}/invoices/${invoiceId}/payment`);
	if (!response.ok) {
		const refusal = await parseRefusal(response, { opaque5xx: false });
		if (refusal.code === CARD_PAYMENTS_NOT_SET_UP) return { kind: 'notSetUp' };
		throw new Error(refusal.message ?? refusal.text);
	}
	return { kind: 'ready', payment: (await response.json()) as ClientPayment };
}
