/**
 * A contractor Doula's accepted fee on one Engagement, and the Fee Payment
 * (GLOSSARY.md, #2271) that records the Practice paid it outside
 * DoulaCloud. A record only: nothing here moves money.
 */

import type { Fetcher } from './fetcher.js';

import { apiErrorMessage } from './apiErrorMessage.js';
import { formatCalendarDay } from './dates.js';
import { refusalError } from './formErrors.js';
import { formatAmount, type PaymentMethod, type RecordPaymentInput } from './invoice.js';

/**
The Fee Payment that stands on a fee: recorded and not reversed.
*/
export interface DoulaFeePaid {
	id: string;
	amountCents: number;
	method: PaymentMethod;
	note?: string;
	paidOn: string;
	recordedBy: string;
	recordedAt: string;
}

/**
One contractor's accepted fee on the Engagement, open or ended.
*/
export interface DoulaFee {
	attachmentId: string;
	doulaName: string;
	amountCents: number;
	acceptedOn: string;
	endedOn?: string;
	paid?: DoulaFeePaid;
}

/**
What she reads of her own Fee Payment on Your offers.
*/
export interface FeePaid {
	amountCents: number;
	method: PaymentMethod;
	paidOn: string;
}

function feesPath(practiceId: string, engagementId: string): string {
	return `/api/practices/${practiceId}/engagements/${engagementId}/doula-fees`;
}

/** Every contractor fee on the Engagement, or undefined for a reader who
 * is not an Owner or Admin: the BFF refuses her (403), and the section is
 * then left out rather than shown broken. Any other failure throws. */
export async function loadDoulaFees(
	fetcher: Fetcher,
	practiceId: string,
	engagementId: string
): Promise<DoulaFee[] | undefined> {
	const response = await fetcher(feesPath(practiceId, engagementId));
	if (response.status === 403) {
		return undefined;
	}
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	const body = (await response.json()) as { items: DoulaFee[] };
	return body.items;
}

/** Records that the Practice paid attachmentId's whole fee. Throws a
 * RefusalError whose `details` the form reads onto its fields. */
export async function recordDoulaFeePaid(
	fetcher: Fetcher,
	practiceId: string,
	engagementId: string,
	attachmentId: string,
	input: RecordPaymentInput
): Promise<void> {
	const response = await fetcher(`${feesPath(practiceId, engagementId)}/${attachmentId}/payments`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(input)
	});
	if (!response.ok) {
		throw await refusalError(response);
	}
}

/**
Undoes a mistaken Fee Payment with an additive reversal (#945's shape).
*/
export async function removeDoulaFeePaid(
	fetcher: Fetcher,
	practiceId: string,
	engagementId: string,
	attachmentId: string,
	paymentId: string,
	reason: string
): Promise<void> {
	const response = await fetcher(
		`${feesPath(practiceId, engagementId)}/${attachmentId}/payments/${paymentId}/reverse`,
		{
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ reason })
		}
	);
	if (!response.ok) {
		throw await refusalError(response);
	}
}

/**
Each method as the end of a sentence a person would say.
*/
const paidHow: Readonly<Record<PaymentMethod, string>> = {
	check: 'by check',
	bank_transfer: 'by bank transfer',
	cash: 'in cash',
	other: 'another way'
};

/** "$450.00 on Mar 3, 2027, by bank transfer", under a Paid label: read by the
 * Owner on the Engagement and by the Doula on Your offers. */
export function feePaidLine(paid: { amountCents: number; method: PaymentMethod; paidOn: string }): string {
	return `${formatAmount(paid.amountCents)} on ${formatCalendarDay(paid.paidOn)}, ${paidHow[paid.method]}`;
}
