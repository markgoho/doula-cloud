/**
 * The Client portal's own read of her Invoices (#1011, #1563): the DTO,
 * the loaders, and the pure rules the money screens share.
 *
 * There is deliberately no `contractId`, Stripe id or Payment note here,
 * and no Staff `Invoice` type reused: the server sends a short allowlist
 * (`payments.ClientInvoiceView`) and this module could not render more
 * if it tried.
 */

import type { Fetcher } from './fetcher.js';

import { apiErrorMessage } from './apiErrorMessage.js';
import {
	clientInvoiceStatusLabel,
	clientPaymentMethodLabel,
	paidAndReturnedLabel,
	TOTAL_TO_PAY_LABEL
} from './clientRegister.js';
import { formatCalendarDay, formatInstant } from './dates.js';
import { formatMoney } from './money.js';
import type { CursorPage } from './paginatedList.svelte.js';

export interface ClientInvoice {
	id: string;
	/**
	Never `draft`.
	*/
	status: string;
	amountCents: number;
	currency: string;
	/**
	Her Invoice number (#981).
	*/
	reference: string;
	billingMode: 'stripe' | 'by_hand';
	createdAt: string;
	/**
	The day it was paid, `YYYY-MM-DD`, in the Practice's own timezone (#1965).
	*/
	paidOn?: string;
	/**
	Present only where a manual Payment record exists.
	*/
	paidMethod?: string;
	/**
	Money that went back to her, 0 when none did (#1009).
	*/
	refundedCents: number;
}

/** One page of the list, with the whole-Engagement figure the server
 * repeats on every page -- never summed here, because a sum of the loaded
 * pages is page-scoped and would shrink as she paged. */
export interface ClientInvoicesPage extends CursorPage<ClientInvoice> {
	totalToPayCents: number;
	totalToPayCount: number;
}

export async function loadClientInvoicesPage(
	fetcher: Fetcher,
	engagementId: string,
	cursor: string
): Promise<ClientInvoicesPage> {
	const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : '';
	const response = await fetcher(`/api/portal/engagements/${engagementId}/invoices${query}`);
	if (!response.ok) throw new Error(await apiErrorMessage(response));
	return (await response.json()) as ClientInvoicesPage;
}

export async function loadClientInvoice(
	fetcher: Fetcher,
	engagementId: string,
	invoiceId: string
): Promise<ClientInvoice> {
	const response = await fetcher(`/api/portal/engagements/${engagementId}/invoices/${invoiceId}`);
	if (!response.ok) throw new Error(await apiErrorMessage(response));
	return (await response.json()) as ClientInvoice;
}

/**
"What you still owe": the `open` Invoices.
*/
export const stillOwed = (items: ClientInvoice[]) => items.filter((each) => each.status === 'open');

/** "What you have paid": the `paid` Invoices -- paid Invoices, not
 * Payment rows, because a Stripe payment carries no method to list. */
export const havePaid = (items: ClientInvoice[]) => items.filter((each) => each.status === 'paid');

/**
`void` and `uncollectible`, which read alike (#981).
*/
export const noLongerOwed = (items: ClientInvoice[]) =>
	items.filter((each) => each.status === 'void' || each.status === 'uncollectible');

/**
How much of this care has gone back to her, across every Invoice.
*/
export const totalReturnedCents = (items: ClientInvoice[]) =>
	items.reduce((sum, each) => sum + each.refundedCents, 0);

/** The row's status wording. A refunded paid Invoice stays `paid` and
 * reads it with the amount that came back (#333's #1009 pointer). */
export function invoiceStatusText(invoice: ClientInvoice): string {
	return invoice.status === 'paid' && invoice.refundedCents > 0
		? paidAndReturnedLabel(formatMoney(invoice.refundedCents, invoice.currency))
		: clientInvoiceStatusLabel(invoice.status);
}

/**
The whole-Engagement "Total to pay", as the one summary-list row the hub
and the Invoices page both draw (#2038): one treatment, so the figure
reads the same in both places. Always written, $0.00 included (#982).
*/
export function totalToPayFact(totalToPayCents: number, currency: string): { label: string; value: string } {
	return { label: TOTAL_TO_PAY_LABEL, value: formatMoney(totalToPayCents, currency) };
}

/**
The facts of one Invoice, in the order she reads them.
*/
export function invoiceFacts(invoice: ClientInvoice): { label: string; value: string }[] {
	const facts = [
		{ label: 'Amount', value: formatMoney(invoice.amountCents, invoice.currency) },
		{ label: 'Invoice number', value: invoice.reference },
		{ label: 'Status', value: invoiceStatusText(invoice) },
		{ label: 'Sent', value: formatInstant(invoice.createdAt) }
	];
	if (invoice.paidOn) {
		facts.push({
			label: 'Paid',
			value: formatCalendarDay(invoice.paidOn)
		});
	}
	if (invoice.paidMethod) {
		facts.push({ label: 'How you paid', value: clientPaymentMethodLabel(invoice.paidMethod) });
	}
	return facts;
}
