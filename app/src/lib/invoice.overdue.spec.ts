/*
 * #768. "Overdue" is derived, never stored: an Invoice is late when its
 * own due date has passed and it is still awaiting payment. These tests
 * move the comparison time rather than waiting for one to arrive, which
 * is the only way the fact can be proven -- Stripe emits no event when a
 * due date passes, and nothing in this product runs on a schedule.
 *
 * Its own file rather than more of `invoice.spec.ts`: that file is the
 * Invoice module's request surface, and this is the derivation over what
 * comes back from it.
 */
import { describe, expect, it, vi } from 'vitest';
import {
	daysOverdue,
	dueLabel,
	loadPaymentTerms,
	practiceInvoicesPath,
	setPaymentTerms,
	type Invoice
} from './invoice.js';
import { jsonResponse } from './testResponse.js';

const openInvoice: Invoice = {
	id: 'inv-1',
	contractId: 'contract-1',
	status: 'open',
	amountCents: 15_000,
	currency: 'usd',
	createdAt: '2026-09-01T00:00:00Z',
	dueAt: '2026-10-01T00:00:00Z',
	reference: 'INV-0001',
	billingMode: 'by_hand'
};

describe('daysOverdue', () => {
	it('counts whole days once the due instant has passed', () => {
		expect(daysOverdue(openInvoice.dueAt, new Date('2026-10-06T00:00:00Z'))).toBe(5);
	});

	it('is zero before the due date, and on the due instant itself', () => {
		expect(daysOverdue(openInvoice.dueAt, new Date('2026-09-30T23:59:00Z'))).toBe(0);
		expect(daysOverdue(openInvoice.dueAt, new Date('2026-10-01T00:00:00Z'))).toBe(0);
	});

	it('does not round a partial day up: an hour late is not a day late', () => {
		expect(daysOverdue(openInvoice.dueAt, new Date('2026-10-01T01:00:00Z'))).toBe(0);
	});
});

describe('dueLabel', () => {
	it('says how late an unpaid invoice is, in the plural it earns', () => {
		expect(dueLabel(openInvoice, new Date('2026-10-02T00:00:00Z'))).toContain('1 day overdue');
		expect(dueLabel(openInvoice, new Date('2026-10-04T00:00:00Z'))).toContain('3 days overdue');
	});

	it('calls an invoice overdue from the instant it falls due, not a day later', () => {
		// The BFF counts this Invoice in the Practice's overdue total the
		// moment the date passes, so the row it lists must not read as if
		// nothing had happened for the rest of that day.
		expect(dueLabel(openInvoice, new Date('2026-10-01T01:00:00Z'))).toContain('overdue');
		expect(dueLabel(openInvoice, new Date('2026-10-01T01:00:00Z'))).not.toContain('day');
	});

	it('says only the date while an invoice is still within its terms', () => {
		expect(dueLabel(openInvoice, new Date('2026-09-15T00:00:00Z'))).not.toContain('overdue');
	});

	it('never calls a settled invoice late, however long ago its date passed', () => {
		for (const status of ['paid', 'void', 'uncollectible']) {
			expect(dueLabel({ ...openInvoice, status }, new Date('2027-01-01T00:00:00Z'))).not.toContain('overdue');
		}
	});
});

describe('practiceInvoicesPath with the overdue narrowing', () => {
	it('asks for the narrowing, and carries it alongside a cursor', () => {
		expect(practiceInvoicesPath('practice-1', undefined, true)).toBe(
			'/api/practices/practice-1/invoices?overdue=true'
		);
		expect(practiceInvoicesPath('practice-1', 'cursor-1', true)).toContain('cursor=cursor-1');
		expect(practiceInvoicesPath('practice-1', 'cursor-1', true)).toContain('overdue=true');
	});
});

describe('payment terms', () => {
	it("reads a practice's terms, and whether they are the default", async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ netDays: 30, isDefault: true }));

		const terms = await loadPaymentTerms(fetcher, 'practice-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/payments/payment-terms');
		expect(terms).toEqual({ netDays: 30, isDefault: true });
	});

	it('throws with the response body text when the read is refused', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('not permitted to read this', 403));

		await expect(loadPaymentTerms(fetcher, 'practice-1')).rejects.toThrow('not permitted to read this');
	});

	it("sets a practice's terms and returns what the BFF stored", async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ netDays: 45, isDefault: false }));

		const terms = await setPaymentTerms(fetcher, 'practice-1', 45);

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/payments/payment-terms', {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ netDays: 45 })
		});
		expect(terms).toEqual({ netDays: 45, isDefault: false });
	});

	it('throws with the response body text when the BFF refuses', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(jsonResponse('netDays must be a whole number of days between 1 and 365', 400));

		await expect(setPaymentTerms(fetcher, 'practice-1', 0)).rejects.toThrow('netDays must be');
	});
});
