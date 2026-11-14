import { describe, expect, it, vi } from 'vitest';
import { downloadedWords, exportFilename, invoicesExportedDetail, requestInvoicesExport, EXPORT_FAILED } from './invoiceExport.js';

const scope = { view: 'unpaid', search: 'Quinn', order: 'amount-largest', days: { from: '2026-10-01', to: '2026-10-31' } } as const;

describe('downloadedWords', () => {
	it('names the file and counts its Invoices', () => {
		expect(downloadedWords('invoices-2026-11-14.csv', 17)).toBe('Downloaded invoices-2026-11-14.csv, 17 Invoices.');
		expect(downloadedWords('invoices-2026-11-14.csv', 1)).toBe('Downloaded invoices-2026-11-14.csv, 1 Invoice.');
	});
});

describe('requestInvoicesExport', () => {
	it('posts the applied view, search, order and period and hands back the file', async () => {
		const fetcher = vi.fn(
			async () =>
				new Response('a,b\n', {
					headers: { 'Content-Disposition': 'attachment; filename="invoices-2026-11-14.csv"', 'Invoice-Count': '17' }
				})
		);

		const result = await requestInvoicesExport(fetcher, 'practice-1', scope);

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/invoices/export', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ view: 'unpaid', search: 'Quinn', order: 'amount-largest', sentFrom: '2026-10-01', sentTo: '2026-10-31' })
		});
		expect(result).toMatchObject({ filename: 'invoices-2026-11-14.csv', count: 17 });
		await expect((result as { blob: Blob }).blob.text()).resolves.toBe('a,b\n');
	});

	it('leaves out an order and a period the list does not have', async () => {
		const fetcher = vi.fn(async () => new Response(''));
		await requestInvoicesExport(fetcher, 'practice-1', { view: 'all', search: '', order: '' });
		expect(fetcher).toHaveBeenCalledWith(expect.any(String), expect.objectContaining({ body: JSON.stringify({ view: 'all', search: '' }) }));
	});

	it.each([
		[429, { 'Retry-After': '1200' }, 'You have downloaded Invoices too many times this hour. Try again in 20 minutes.'],
		[429, { 'Retry-After': '30' }, 'You have downloaded Invoices too many times this hour. Try again in 1 minute.'],
		[429, {}, 'You have downloaded Invoices too many times this hour. Try again later.'],
		[403, {}, 'Your role cannot download Invoices.'],
		[500, {}, EXPORT_FAILED]
	])('says a %i in words a person reads', async (status, headers, refusal) => {
		const fetcher = vi.fn(async () => new Response('no', { status, headers }));
		await expect(requestInvoicesExport(fetcher, 'practice-1', scope)).resolves.toEqual({ refusal });
	});

	it('says an absent connection the same way a failed read is', async () => {
		const fetcher = vi.fn(async () => {
			throw new TypeError('offline');
		});
		await expect(requestInvoicesExport(fetcher, 'practice-1', scope)).resolves.toEqual({ refusal: EXPORT_FAILED });
	});
});

describe('exportFilename', () => {
	it('reads the name the server gave, and falls back', () => {
		expect(exportFilename('attachment; filename="invoices-2026-11-14.csv"')).toBe('invoices-2026-11-14.csv');
		expect(exportFilename(undefined)).toBe('invoices.csv');
	});
});

describe('invoicesExportedDetail', () => {
	it.each([
		[{ view: 'unpaid', sentFrom: '2026-10-01', sentTo: '2026-10-31', rows: 14, searched: true }, '14 Invoices: Unpaid, sent in October 2026, with a search'],
		[{ view: 'all', rows: 1, searched: false }, '1 Invoice: All'],
		[{ view: 'overdue', order: 'amount-largest', rows: 6, searched: false }, '6 Invoices: Overdue, largest amount first']
	])('says %j', (diff, line) => {
		expect(invoicesExportedDetail(diff)).toBe(line);
	});

	it('says nothing for a diff it cannot read', () => {
		expect(invoicesExportedDetail({ view: 'bogus', rows: 1 })).toBeUndefined();
		expect(invoicesExportedDetail({ view: 'all' })).toBeUndefined();
		expect(invoicesExportedDetail({ view: 'all', rows: 2, order: 'bogus' })).toBe('2 Invoices: All');
	});
});
