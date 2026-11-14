import { describe, expect, it, vi } from 'vitest';
import { clientsDownloadedWords, clientsExportedDetail, requestClientsExport } from './clientExport.js';
import { clientOrderPhrase } from './client.js';

const scope = {
	all: false,
	mine: false,
	owes: true,
	noContract: false,
	doula: 'staff-2',
	name: 'Anne',
	due: '2027-03',
	dueFrom: '',
	dueTo: '',
	order: 'overdue'
} as const;

describe('clientsDownloadedWords', () => {
	it('names the file and counts its Clients', () => {
		expect(clientsDownloadedWords('clients-2026-11-14.csv', 16)).toBe('Downloaded clients-2026-11-14.csv, 16 Clients.');
		expect(clientsDownloadedWords('clients-2026-11-14.csv', 1)).toBe('Downloaded clients-2026-11-14.csv, 1 Client.');
	});
});

describe('requestClientsExport', () => {
	it('posts the list’s own request and hands back the file', async () => {
		const fetcher = vi.fn(
			async () =>
				new Response('a,b\n', { headers: { 'Content-Disposition': 'attachment; filename="clients-2026-11-14.csv"', 'Client-Count': '16' } })
		);

		const result = await requestClientsExport(fetcher, 'practice-1', scope);

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/clients/export', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(scope)
		});
		expect(result).toMatchObject({ filename: 'clients-2026-11-14.csv', count: 16 });
	});

	it('falls back to a file name when the server gave none', async () => {
		const fetcher = vi.fn(async () => new Response(''));
		await expect(requestClientsExport(fetcher, 'practice-1', scope)).resolves.toMatchObject({ filename: 'clients.csv' });
	});

	it.each([
		[429, { 'Retry-After': '1200' }, 'You have downloaded Clients too many times this hour. Try again in 20 minutes.'],
		[429, {}, 'You have downloaded Clients too many times this hour. Try again later.'],
		[403, {}, 'Your role cannot download Clients.'],
		[500, {}, 'Your Clients did not download. Nothing you saved is lost.']
	])('says a %i in words a person reads', async (status, headers, refusal) => {
		const fetcher = vi.fn(async () => new Response('no', { status, headers }));
		await expect(requestClientsExport(fetcher, 'practice-1', scope)).resolves.toEqual({ refusal });
	});
});

describe('clientsExportedDetail', () => {
	it.each([
		[{ owes: true, doulaId: 'd', rows: 16, searched: true }, "16 Clients: who owe money, one Doula's, with a search"],
		[{ rows: 1, searched: false }, '1 Client'],
		[{ noContract: true, mine: true, due: '2027-03', all: true, order: 'overdue', rows: 4 }, "4 Clients: with no Contract, the downloader's own, due in March 2027, including anyone not yet in care, most overdue first"],
		[{ dueFrom: '2027-03-01', dueTo: '2027-03-15', rows: 2 }, '2 Clients: due Mar 1 to Mar 15, 2027']
	])('says %j', (diff, line) => {
		expect(clientsExportedDetail(diff)).toBe(line);
	});

	it('says nothing for a diff it cannot read, and skips an order it does not know', () => {
		expect(clientsExportedDetail({ owes: true })).toBeUndefined();
		expect(clientsExportedDetail({ rows: 2, order: 'bogus' })).toBe('2 Clients');
		expect(clientsExportedDetail({ rows: 2, order: '' })).toBe('2 Clients');
	});
});

describe('clientOrderPhrase', () => {
	it('words an order the list offers', () => {
		expect(clientOrderPhrase('name')).toBe('by name, A to Z');
	});
});
