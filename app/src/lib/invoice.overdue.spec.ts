/*
 * #768. "Overdue" is derived, never stored. Since #1965 the BFF derives
 * it: an Invoice is late from the start of the day after its `dueOn` in
 * the Practice's own timezone, and each row of the Practice-wide list
 * carries `daysOverdue`, counted at the same instant as the page's
 * overdue figure. The screen prints what it was sent.
 *
 * Its own file rather than more of `invoice.spec.ts`: that file is the
 * Invoice module's request surface, and this is what the screen makes of
 * what comes back from it.
 */
import { describe, expect, it, vi } from 'vitest';
import {
	INVOICES_RESULT_RULER,
	invoiceDateLine,
	invoiceStateWord,
	invoicesResultSummary,
	invoicesViewOf,
	invoicesViewQuery,
	loadPaymentTerms,
	practiceInvoicesPath,
	setPaymentTerms
} from './invoice.js';

const counts = { invoiceCount: 35, outstandingCount: 12, overdueCount: 7 };

describe('invoicesResultSummary', () => {
	it.each([
		['all', 30, true, 'Showing the first 30 of 35 Invoices, newest first.'],
		['all', 35, false, 'Showing 35 Invoices, newest first.'],
		['all', 0, false, 'No Invoices yet.'],
		['unpaid', 12, false, 'Showing 12 unpaid Invoices, earliest payment due first.'],
		['unpaid', 0, false, 'No Invoice is unpaid.'],
		['overdue', 7, false, 'Showing 7 overdue Invoices, longest overdue first.'],
		['overdue', 1, false, 'Showing 1 overdue Invoice.'],
		['overdue', 0, false, 'No Invoice is overdue.']
	] as const)('%s with %i shown (more: %s) reads "%s"', (view, shown, hasMore, text) => {
		expect(invoicesResultSummary(view, shown, hasMore, counts)).toBe(text);
	});

	// A total read at a later instant than the rows never says fewer than are shown.
	it('never names a total below the rows on screen', () => {
		expect(invoicesResultSummary('overdue', 8, true, counts)).toBe(
			'Showing the first 8 of 8 overdue Invoices, longest overdue first.'
		);
	});

	it('reserves room for its longest wording', () => {
		expect(INVOICES_RESULT_RULER).toBe(
			'Showing the first 999 of 9999 overdue Invoices, earliest payment due first.'
		);
	});
});

describe('invoicesViewOf', () => {
	it.each([
		['', 'all'],
		['show=all', 'all'],
		['show=unpaid', 'unpaid'],
		['show=overdue', 'overdue'],
		['show=anything', 'all'],
		['overdue=true', 'all']
	] as const)('reads "%s" as %s', (query, view) => {
		expect(invoicesViewOf(new URLSearchParams(query))).toBe(view);
	});

	it('names a view as the address the screen links to', () => {
		expect(invoicesViewQuery('all')).toBe('');
		expect(invoicesViewQuery('unpaid')).toBe('?show=unpaid');
	});
});
import { jsonResponse } from './testResponse.js';

describe('invoiceStateWord', () => {
	it('says how late an unpaid invoice is, in the plural it earns', () => {
		expect(invoiceStateWord({ status: 'open', daysOverdue: 1 })).toBe('1 day overdue');
		expect(invoiceStateWord({ status: 'open', daysOverdue: 3 })).toBe('3 days overdue');
	});

	// The BFF sends 0 for an Invoice within its terms, on its own due day,
	// and for every settled one, however long ago its day passed.
	it('says the state in words when the BFF counted no lateness', () => {
		expect(invoiceStateWord({ status: 'open', daysOverdue: 0 })).toBe('owed');
		expect(invoiceStateWord({ status: 'paid', daysOverdue: 0 })).toBe('paid');
	});
});

describe('invoiceDateLine', () => {
	// `dueOn` is a calendar day, month first (#1958), never "10/1/2026",
	// and never moved by the reader's zone (dates.zone.spec.ts).
	it('says the day payment is due while the Invoice can be paid', () => {
		expect(invoiceDateLine({ status: 'open', dueOn: '2026-10-01' })).toEqual({ lead: 'Payment due', day: 'Oct 1, 2026', on: '2026-10-01' });
		expect(invoiceDateLine({ status: 'draft', dueOn: '2026-10-01' })?.lead).toBe('Payment due');
	});

	it('says the day it was paid once it is', () => {
		expect(invoiceDateLine({ status: 'paid', dueOn: '2026-10-01', paidOn: '2026-09-28' })).toEqual({ lead: 'Paid', day: 'Sep 28, 2026', on: '2026-09-28' });
	});

	it('says no day for an Invoice nobody will pay', () => {
		expect(invoiceDateLine({ status: 'void', dueOn: '2026-10-01' })).toBeUndefined();
		expect(invoiceDateLine({ status: 'uncollectible', dueOn: '2026-10-01' })).toBeUndefined();
	});
});

describe('practiceInvoicesPath with a view', () => {
	it('asks for the narrowing, and carries it alongside a cursor', () => {
		expect(practiceInvoicesPath('practice-1', undefined, 'overdue')).toBe(
			'/api/practices/practice-1/invoices?overdue=true'
		);
		expect(practiceInvoicesPath('practice-1', 'cursor-1', 'unpaid')).toContain('cursor=cursor-1');
		expect(practiceInvoicesPath('practice-1', 'cursor-1', 'unpaid')).toContain('unpaid=true');
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
