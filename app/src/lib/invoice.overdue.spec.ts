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
import { dueLabel, loadPaymentTerms, practiceInvoicesPath, setPaymentTerms } from './invoice.js';
import { jsonResponse } from './testResponse.js';

describe('dueLabel', () => {
	it('says how late an unpaid invoice is, in the plural it earns', () => {
		expect(dueLabel({ dueOn: '2026-10-01', daysOverdue: 1 })).toBe('Oct 1, 2026 — 1 day overdue');
		expect(dueLabel({ dueOn: '2026-10-01', daysOverdue: 3 })).toBe('Oct 1, 2026 — 3 days overdue');
	});

	// The BFF sends 0 for an Invoice within its terms, on its own due day,
	// and for every settled one, however long ago its day passed.
	it('says only the day when the BFF counted no lateness', () => {
		expect(dueLabel({ dueOn: '2026-10-01', daysOverdue: 0 })).toBe('Oct 1, 2026');
	});

	// `dueOn` is a calendar day, month first (#1958), never "10/1/2026",
	// and never moved by the reader's zone (dates.zone.spec.ts).
	it('prints the due day month first', () => {
		expect(dueLabel({ dueOn: '2026-10-01', daysOverdue: 0 })).toMatch(/^[A-Z][a-z]{2} \d{1,2}, 2026$/);
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
