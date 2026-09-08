import { describe, expect, it, vi } from 'vitest';
import {
	billableContractStatus,
	createInvoice,
	formatAmount,
	invoiceStatusLabel,
	loadInvoices,
	loadPracticeInvoices,
	practiceInvoicesPath,
	unbillableContractMessage
} from './invoice.js';
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
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(invoice));

		const result = await createInvoice(fetcher, 'practice-1', 'eng-1', 15_000);

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/engagements/eng-1/contract/invoices', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ amountCents: 15_000 })
		});
		expect(result).toEqual(invoice);
	});

	// #270: Clients cannot pay this Practice is a standing fact the caller
	// reads before ever calling createInvoice, so a refusal reaching this
	// function is always thrown like any other non-2xx response.
	it('throws with the response body text when Clients cannot pay this Practice', async () => {
		const fetcher = vi.fn().mockResolvedValue(
			jsonResponse('Clients cannot pay this Practice yet. A Practice Owner has to connect Stripe.', 409)
		);

		await expect(createInvoice(fetcher, 'practice-1', 'eng-1', 15_000)).rejects.toThrow(
			'Clients cannot pay this Practice yet. A Practice Owner has to connect Stripe.'
		);
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

describe('invoiceStatusLabel', () => {
	it('reads each Stripe status back in the words a person reads', () => {
		expect(invoiceStatusLabel('draft')).toBe('Draft');
		expect(invoiceStatusLabel('open')).toBe('Open');
		expect(invoiceStatusLabel('paid')).toBe('Paid');
		expect(invoiceStatusLabel('uncollectible')).toBe('Uncollectible');
		expect(invoiceStatusLabel('void')).toBe('Void');
	});

	it('falls through to the status itself rather than a blank when Stripe adds one', () => {
		expect(invoiceStatusLabel('something_new')).toBe('something_new');
	});
});

describe('unbillableContractMessage', () => {
	it('names the voided Contract state specifically', () => {
		expect(unbillableContractMessage('voided')).toBe(
			'This Contract has been voided. Invoicing is unavailable until Staff issues a new Contract.'
		);
	});

	it('gives the same not-yet-signed message for draft and sent', () => {
		expect(unbillableContractMessage('draft')).toBe(
			'Invoicing is unavailable until the Client signs this Contract.'
		);
		expect(unbillableContractMessage('sent')).toBe(
			'Invoicing is unavailable until the Client signs this Contract.'
		);
	});
});

describe('billableContractStatus', () => {
	it('is the one status a Contract may be billed at, matching the BFF', () => {
		expect(billableContractStatus).toBe('signed');
	});
});

describe('practiceInvoicesPath', () => {
	it('addresses the Practice-wide list', () => {
		expect(practiceInvoicesPath('practice-1')).toBe('/api/practices/practice-1/invoices');
	});

	it('carries an encoded cursor when there is one', () => {
		expect(practiceInvoicesPath('practice-1', 'a+b/c=')).toBe(
			'/api/practices/practice-1/invoices?cursor=a%2Bb%2Fc%3D'
		);
	});
});

describe('loadPracticeInvoices', () => {
	const row = {
		id: 'inv-1',
		engagementId: 'eng-1',
		contractId: 'contract-1',
		clientName: 'Ada',
		status: 'open',
		amountCents: 15_000,
		currency: 'usd',
		createdAt: '2026-01-01T00:00:00Z'
	};

	it('fetches the Practice-wide path and returns the page with its whole-book totals', async () => {
		const body = {
			items: [row],
			hasMore: false,
			outstandingCents: 15_000,
			outstandingCount: 1,
			paidCents: 0
		};
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(body));

		const result = await loadPracticeInvoices(fetcher, 'practice-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/invoices');
		expect(result).toEqual(body);
	});

	it('passes a cursor through for the next page', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(
				jsonResponse({ items: [], hasMore: false, outstandingCents: 0, outstandingCount: 0, paidCents: 0 })
			);

		await loadPracticeInvoices(fetcher, 'practice-1', 'cursor-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/invoices?cursor=cursor-1');
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('invalid cursor', 400));

		await expect(loadPracticeInvoices(fetcher, 'practice-1', 'bad')).rejects.toThrow('invalid cursor');
	});
});
