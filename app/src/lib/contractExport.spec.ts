import { describe, expect, it, vi } from 'vitest';
import { contractsDownloadedWords, contractsExportedDetail, requestContractsExport } from './contractExport.js';

describe('contractsDownloadedWords', () => {
	it('names the file and counts its Contracts', () => {
		expect(contractsDownloadedWords('contracts-2026-11-14.csv', 17)).toBe('Downloaded contracts-2026-11-14.csv, 17 Contracts.');
		expect(contractsDownloadedWords('contracts-2026-11-14.csv', 1)).toBe('Downloaded contracts-2026-11-14.csv, 1 Contract.');
	});
});

describe('requestContractsExport', () => {
	it('posts the applied view, search, order, Mine, Doula and period', async () => {
		const fetcher = vi.fn(async () => new Response('a,b\n', { headers: { 'Content-Disposition': 'attachment; filename="contracts-2026-11-14.csv"', 'Contract-Count': '4' } }));

		const result = await requestContractsExport(fetcher, 'practice-1', {
			view: 'signed',
			search: 'Quinn',
			order: 'client',
			isMine: true,
			doula: 'staff-2',
			days: { from: '2026-10-01', to: '2026-10-31' }
		});

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/contracts/export', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ view: 'signed', search: 'Quinn', mine: true, doula: 'staff-2', order: 'client', sentFrom: '2026-10-01', sentTo: '2026-10-31' })
		});
		expect(result).toMatchObject({ filename: 'contracts-2026-11-14.csv', count: 4 });
	});

	it('sends the default view as the empty name the list sends, and leaves out an order and a period it does not have', async () => {
		const fetcher = vi.fn(async () => new Response(''));
		await requestContractsExport(fetcher, 'practice-1', { view: 'not-signed', search: '', order: '', isMine: false, doula: '' });
		expect(fetcher).toHaveBeenCalledWith(
			expect.any(String),
			expect.objectContaining({ body: JSON.stringify({ view: '', search: '', mine: false, doula: '' }) })
		);
	});
});

describe('contractsExportedDetail', () => {
	it('says how many left and which slice, never the search', () => {
		const diff = { view: 'signed', sentFrom: '2026-10-01', sentTo: '2026-10-31', order: 'newest', mine: true, doulaId: 'staff-2', rows: 12, searched: true };
		expect(contractsExportedDetail(diff)).toBe(
			"12 Contracts: signed, the downloader's own, one Doula's, sent in October 2026, newest first, with a search"
		);
	});

	it('says one Contract in the singular, and the default view in its words', () => {
		expect(contractsExportedDetail({ view: 'not-signed', rows: 1, searched: false })).toBe('1 Contract: not signed');
	});

	it('says nothing for a diff it cannot read', () => {
		expect(contractsExportedDetail({ nope: true })).toBeUndefined();
		expect(contractsExportedDetail({ view: 'elsewhere', rows: 2 })).toBeUndefined();
	});
});
