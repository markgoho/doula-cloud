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

import { apiErrorMessage } from './api.js';

export interface ClientPayment {
	clientSecret: string;
	/**
	`stripeAccount` for the Stripe.js instance -- an identifier, not a credential.
	*/
	stripeAccountId: string;
	publishableKey: string;
}

/** Throws the server's own sentence when it refuses ("Your Practice cannot
 * take this payment yet."), which is written for the Client to read. */
export async function loadClientPayment(
	fetcher: Fetcher,
	engagementId: string,
	invoiceId: string
): Promise<ClientPayment> {
	const response = await fetcher(`/api/portal/engagements/${engagementId}/invoices/${invoiceId}/payment`);
	if (!response.ok) throw new Error(await apiErrorMessage(response));
	return (await response.json()) as ClientPayment;
}
