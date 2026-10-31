import { describe, expect, it, vi } from 'vitest';
import {
	havePaid,
	invoiceFacts,
	invoiceStatusText,
	loadClientInvoice,
	loadClientInvoicesPage,
	noLongerOwed,
	stillOwed,
	totalReturnedCents,
	type ClientInvoice
} from './clientInvoice.js';

const base: ClientInvoice = {
	id: 'i',
	status: 'open',
	amountCents: 1000,
	currency: 'usd',
	reference: 'INV-1',
	billingMode: 'stripe',
	createdAt: '2027-01-01T12:00:00Z',
	raisedOn: '2027-01-01',
	refundedCents: 0
};

describe('the money sections', () => {
	const all = [
		{ ...base, id: 'a' },
		{ ...base, id: 'b', status: 'paid', refundedCents: 300 },
		{ ...base, id: 'c', status: 'void' },
		{ ...base, id: 'd', status: 'uncollectible' }
	];

	it('splits one list by status, so no second array can drift', () => {
		expect(stillOwed(all).map((each) => each.id)).toEqual(['a']);
		expect(havePaid(all).map((each) => each.id)).toEqual(['b']);
		expect(noLongerOwed(all).map((each) => each.id)).toEqual(['c', 'd']);
	});

	it('sums what went back to her across Invoices', () => {
		expect(totalReturnedCents(all)).toBe(300);
	});
});

describe('invoiceStatusText', () => {
	it('reads an open Invoice as not yet paid', () => {
		expect(invoiceStatusText(base)).toBe('Not yet paid');
	});

	// #2225: the label states what the model holds -- her payment is on
	// its way -- and only while the Invoice is still open.
	it('reads Payment on its way while her payment settles', () => {
		expect(invoiceStatusText({ ...base, paymentProcessing: true })).toBe('Payment on its way');
	});
});

describe('invoiceFacts', () => {
	it('lists only the facts an open Invoice has', () => {
		expect(invoiceFacts(base).map((fact) => fact.label)).toEqual(['Amount', 'Invoice number', 'Status', 'Sent']);
	});

	it("prints Sent as the Practice's day the BFF sent, not the day of the instant (#2205)", () => {
		// 00:03Z on Oct 28 is 8:03pm on Oct 27 in the Practice's New York.
		const facts = invoiceFacts({ ...base, createdAt: '2026-10-28T00:03:00Z', raisedOn: '2026-10-27' });
		expect(facts.find((fact) => fact.label === 'Sent')?.value).toBe('Oct 27, 2026');
	});

	it('adds when it was paid and how, where a manual record exists', () => {
		const facts = invoiceFacts({ ...base, status: 'paid', paidOn: '2027-01-02', paidMethod: 'cash' });
		expect(facts.map((fact) => fact.label)).toEqual([
			'Amount',
			'Invoice number',
			'Status',
			'Sent',
			'Paid',
			'How you paid'
		]);
		expect(facts.at(-1)?.value).toBe('Cash');
	});
});

describe('the loaders', () => {
	it('asks for the next page by cursor and returns the envelope', async () => {
		const fetcher = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({ items: [] }) });
		await loadClientInvoicesPage(fetcher, 'e1', 'a b');
		expect(fetcher).toHaveBeenCalledWith('/api/portal/engagements/e1/invoices?cursor=a%20b');
	});

	it('throws the server message when a read is refused', async () => {
		const fetcher = vi.fn().mockResolvedValue({ ok: false, text: () => Promise.resolve('refused') });
		await expect(loadClientInvoicesPage(fetcher, 'e1', '')).rejects.toThrow('refused');
		await expect(loadClientInvoice(fetcher, 'e1', 'i1')).rejects.toThrow('refused');
	});
});
