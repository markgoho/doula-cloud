/*
 * PROTOTYPE -- throwaway. Wayfinder ticket #983, "The Client's money
 * surface: one screen, both rails, from 320px". Not production code, no
 * tests, no error handling: three structurally different answers to
 * "what does this look like" mounted on one route and switched from a
 * URL search param.
 *
 * Every word a Client reads here is #981's, not this file's invention:
 * "What you still owe", "Total to pay", "What you have paid", and one
 * fixed label per invoice_status. Every empty-state sentence is #982's.
 * No form of "balance" appears anywhere, because Credit is the
 * Practice's own platform-billing unit.
 */

import type { Invoice, Payment } from '#lib/invoice.js';

/** The six states the ticket names, plus the "no Invoices at all" case
 * #982 wrote a sentence for. */
export type StateKey =
	| 'nothing-owed'
	| 'one-open'
	| 'deposit-and-balance'
	| 'by-hand'
	| 'no-invoices'
	| 'ended';

export interface MoneyState {
	readonly key: StateKey;
	readonly name: string;
	readonly invoices: Invoice[];
	readonly payments: Payment[];
	/** `completed` is the only terminal Engagement status (#982): the
	 * surface is ending-blind, so this changes nothing a Client reads.
	 * It is here to prove that. */
	readonly engagementStatus: 'active' | 'completed';
}

const deposit: Invoice = {
	id: 'in_deposit',
	contractId: 'c_1',
	status: 'paid',
	amountCents: 90_000,
	currency: 'usd',
	createdAt: '2027-06-02T00:00:00Z',
	paidAt: '2027-06-04T00:00:00Z',
	reference: 'A4B2-0007',
	billingMode: 'stripe'
};

const balance: Invoice = {
	id: 'in_balance',
	contractId: 'c_1',
	status: 'open',
	amountCents: 335_000,
	currency: 'usd',
	createdAt: '2027-11-30T00:00:00Z',
	reference: 'A4B2-0011',
	billingMode: 'stripe'
};

const byHandBalance: Invoice = {
	id: 'in_by_hand',
	contractId: 'c_1',
	status: 'open',
	amountCents: 425_000,
	currency: 'usd',
	createdAt: '2027-11-30T00:00:00Z',
	reference: 'INV-0002',
	billingMode: 'by_hand'
};

/*
 * A finding the fixture forced, and it belongs in the resolution: a
 * `Payment` row is a MANUALLY RECORDED payment (`PaymentMethod` is
 * check | bank_transfer | cash | other -- there is no `card`). On the
 * Stripe rail nobody records one; the paid Invoice's own `paidAt` is
 * the whole evidence that money arrived. So "What you have paid" cannot
 * be a list of Payment rows -- on the Stripe rail it would always be
 * empty. It has to read from PAID INVOICES, and show a Payment's method
 * beside one only where a Payment row exists.
 */
const checkPayment: Payment = {
	id: 'pay_check',
	invoiceId: 'in_deposit',
	amountCents: 90_000,
	method: 'check',
	// #981: she reads the method and never the recorder's note. The note
	// is here so the prototype proves it is not rendered.
	note: 'Left at the front desk, deposited Tuesday',
	paidAt: '2027-06-04T00:00:00Z',
	createdAt: '2027-06-05T00:00:00Z'
};

export const states: readonly MoneyState[] = [
	{
		key: 'nothing-owed',
		name: 'Nothing owed',
		invoices: [deposit],
		payments: [],
		engagementStatus: 'active'
	},
	{
		key: 'one-open',
		name: 'One Invoice open, Stripe rail',
		invoices: [balance],
		payments: [],
		engagementStatus: 'active'
	},
	{
		key: 'deposit-and-balance',
		name: 'Paid deposit + open balance (#741)',
		invoices: [deposit, balance],
		payments: [],
		engagementStatus: 'active'
	},
	{
		key: 'by-hand',
		name: 'By-hand Invoice, nothing to click (#946)',
		invoices: [deposit, byHandBalance],
		payments: [checkPayment],
		engagementStatus: 'active'
	},
	{
		key: 'no-invoices',
		name: 'No Invoices at all',
		invoices: [],
		payments: [],
		engagementStatus: 'active'
	},
	{
		key: 'ended',
		name: 'Care has ended, one Invoice open',
		invoices: [deposit, balance],
		payments: [],
		engagementStatus: 'completed'
	}
];

/** #981: one fixed label per status. `void` and `uncollectible` share
 * "No longer owed" -- they differ only in the Practice's accounting, and
 * "written off" narrates the Practice's loss to her. `draft` never
 * reaches her, so it has no label. */
export function invoiceStatusLabel(status: string): string {
	if (status === 'paid') return 'Paid';
	if (status === 'open') return 'Not yet paid';
	return 'No longer owed';
}

export function invoiceStatusVariant(status: string): 'success' | 'info' | 'neutral' {
	if (status === 'paid') return 'success';
	if (status === 'open') return 'info';
	return 'neutral';
}

export function paymentMethodLabel(method: string): string {
	if (method === 'card') return 'Card';
	if (method === 'check') return 'Check';
	if (method === 'cash') return 'Cash';
	if (method === 'bank_transfer') return 'Bank transfer';
	return 'Other';
}

export function unpaid(state: MoneyState): Invoice[] {
	return state.invoices.filter((invoice) => invoice.status === 'open');
}

/** #981: the model holds no partial Payments, so what she owes is the
 * sum of her open Invoices and nothing subtler. */
export function totalToPayCents(state: MoneyState): number {
	return unpaid(state).reduce((sum, invoice) => sum + invoice.amountCents, 0);
}

/** #982: three sentences, each claiming only what the model holds. */
export const emptyOwed = 'You have paid every Invoice for this care';
export const emptyInvoices = 'There are no Invoices for this care';
export const emptyPayments = 'There are no Payments for this care';
