import { describe, expect, it, vi } from 'vitest';
import {
	invoiceOrderedBy,
	invoiceOrderOf,
	invoiceOrderOptions,
	invoicesResultSummary,
	invoicesViewQuery,
	invoiceViewOrder,
	loadPracticeInvoices,
	orderForView,
	practiceInvoicesPath
} from './invoice.js';

const counts = { invoiceCount: 40, outstandingCount: 20, overdueCount: 5 };

describe('the order of the Invoices list (#2869)', () => {
	it.each([
		['order=amount-largest', 'all', 'amount-largest'],
		['order=sent-newest', 'all', ''],
		['order=sent-newest', 'unpaid', 'sent-newest'],
		['order=due-earliest', 'overdue', ''],
		['order=bogus', 'all', ''],
		['', 'all', '']
	] as const)('reads "%s" under %s as "%s"', (query, view, order) => {
		expect(invoiceOrderOf(new URLSearchParams(query), view)).toBe(order);
	});

	it('knows the order each view reads in without being told', () => {
		expect(invoiceViewOrder('all')).toBe('sent-newest');
		expect(invoiceViewOrder('unpaid')).toBe('due-earliest');
		expect(orderForView('due-earliest', 'overdue')).toBe('');
		expect(orderForView('due-earliest', 'all')).toBe('due-earliest');
	});

	it('labels the empty option with the view’s own order and offers the other five', () => {
		expect(invoiceOrderOptions('all').map((option) => option.label)).toEqual([
			'Sent, newest first',
			'Sent, oldest first',
			'Payment due, earliest first',
			'Payment due, latest first',
			'Largest amount first',
			'Smallest amount first'
		]);
		expect(invoiceOrderOptions('unpaid').map((option) => option.value)).toEqual([
			'',
			'sent-newest',
			'sent-oldest',
			'due-latest',
			'amount-largest',
			'amount-smallest'
		]);
	});

	it('names the column an order sorts, the view’s own by default', () => {
		expect(invoiceOrderedBy('', 'all')).toEqual({ column: 'Sent', direction: 'descending' });
		expect(invoiceOrderedBy('', 'overdue')).toEqual({ column: 'Payment', direction: 'ascending' });
		expect(invoiceOrderedBy('amount-smallest', 'all')).toEqual({ column: 'Money', direction: 'ascending' });
	});

	it('writes the view and the order into the address, and nothing for the view’s own', () => {
		expect(invoicesViewQuery('all', 'sent-oldest')).toBe('?order=sent-oldest');
		expect(invoicesViewQuery('overdue', 'amount-largest')).toBe('?show=overdue&order=amount-largest');
		expect(invoicesViewQuery('unpaid')).toBe('?show=unpaid');
	});

	it('says an order in the result line, in its own words', () => {
		expect(invoicesResultSummary('all', 30, true, counts, '', 'sent-oldest')).toBe('Showing the first 30 of 40 Invoices, oldest first.');
		expect(invoicesResultSummary('unpaid', 5, false, counts, '', 'amount-largest')).toBe(
			'Showing 5 unpaid Invoices, largest amount first.'
		);
		expect(invoicesResultSummary('all', 5, false, counts, 'Quinn', 'due-latest')).toBe(
			'Showing 5 Invoices matching "Quinn", latest payment due first.'
		);
	});

	it('asks the BFF for an order in the path and in a search’s body', async () => {
		expect(practiceInvoicesPath('practice-1', 'c-1', 'unpaid', 'due-latest')).toBe(
			'/api/practices/practice-1/invoices?unpaid=true&order=due-latest&cursor=c-1'
		);
		const fetcher = vi.fn().mockResolvedValue(new Response('{}'));
		await loadPracticeInvoices(fetcher, 'practice-1', undefined, 'all', 'Quinn', 'amount-smallest');
		expect(JSON.parse(fetcher.mock.calls[0][1].body)).toEqual({ view: 'all', search: 'Quinn', cursor: '', order: 'amount-smallest' });
	});
});
