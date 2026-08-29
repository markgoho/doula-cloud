import { describe, expect, it, vi } from 'vitest';
import { createInvoice, formatAmount, loadInvoices } from './invoice.js';
import { jsonResponse } from './testResponse.js';

describe('loadInvoices', () => {
	it('fetches the practice+engagement invoices path and returns the decoded items', async () => {
		const invoice = {
			id: 'inv-1',
			contractId: 'contract-1',
			status: 'open',
			amountCents: 15_000,
			currency: 'usd',
			createdAt: '2026-01-01T00:00:00Z'
		};
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ items: [invoice], hasMore: false }));

		const result = await loadInvoices(fetcher, 'practice-1', 'eng-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/engagements/eng-1/contract/invoices');
		expect(result).toEqual([invoice]);
	});

	it('returns an empty list when the response has none yet (no Contract required to list)', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ items: [], hasMore: false }));

		const result = await loadInvoices(fetcher, 'practice-1', 'eng-1');

		expect(result).toEqual([]);
	});

	it('throws with the response body text on a 404 (unlike loadContract, this always means a real error)', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('engagement not found', 404));

		await expect(loadInvoices(fetcher, 'practice-1', 'eng-1')).rejects.toThrow('engagement not found');
	});

	it('throws with the response body text on any other non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('server error', 500));

		await expect(loadInvoices(fetcher, 'practice-1', 'eng-1')).rejects.toThrow('server error');
	});
});

describe('createInvoice', () => {
	it('POSTs the amount and returns the created invoice', async () => {
		const invoice = {
			id: 'inv-1',
			contractId: 'contract-1',
			status: 'open',
			amountCents: 15_000,
			currency: 'usd',
			createdAt: '2026-01-01T00:00:00Z'
		};
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ connectRequired: false, invoice }));

		const result = await createInvoice(fetcher, 'practice-1', 'eng-1', 15_000);

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/engagements/eng-1/contract/invoices', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ amountCents: 15_000 })
		});
		expect(result).toEqual({ connectRequired: false, invoice });
	});

	it('returns the connect-gate state when the Practice is not connected', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ connectRequired: true, isOwner: true }));

		const result = await createInvoice(fetcher, 'practice-1', 'eng-1', 15_000);

		expect(result).toEqual({ connectRequired: true, isOwner: true });
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('amountCents must be greater than zero', 400));

		await expect(createInvoice(fetcher, 'practice-1', 'eng-1', 0)).rejects.toThrow(
			'amountCents must be greater than zero'
		);
	});
});

describe('formatAmount', () => {
	it('formats cents as a USD currency string', () => {
		expect(formatAmount(15_000)).toBe('$150.00');
	});

	it('formats a non-whole-dollar amount', () => {
		expect(formatAmount(1050)).toBe('$10.50');
	});
});
