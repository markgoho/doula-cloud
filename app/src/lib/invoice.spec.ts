import { describe, expect, it, vi } from 'vitest';
import {
	billableContractStatus,
	createInvoice,
	formatAmount,
	invoiceStatusLabel,
	loadBillingMode,
	loadInvoices,
	loadPracticeInvoices,
	practiceInvoicesPath,
	recordPayment,
	reversePayment,
	setBillingMode,
	unbillableContractMessage,
	voidInvoice,
	writeOffInvoice
} from './invoice.js';
import { RefusalError } from './formErrors.js';
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
	// #947: the amount is derived from the Contract on the BFF side, so
	// this call never sends one -- there is nothing left to assert about
	// an amount reaching the wire.
	it('POSTs with no amount and returns the created invoice', async () => {
		const invoice = {
			id: 'inv-1',
			contractId: 'contract-1',
			status: 'open',
			amountCents: 15_000,
			currency: 'usd',
			createdAt: '2026-01-01T00:00:00Z'
		};
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(invoice));

		const result = await createInvoice(fetcher, 'practice-1', 'eng-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/engagements/eng-1/contract/invoices', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({})
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

		await expect(createInvoice(fetcher, 'practice-1', 'eng-1')).rejects.toThrow(
			'Clients cannot pay this Practice yet. A Practice Owner has to connect Stripe.'
		);
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('engagement not found', 404));

		await expect(createInvoice(fetcher, 'practice-1', 'eng-1')).rejects.toThrow('engagement not found');
	});

	it('carries billingMode when the caller supplies it (the inline "ask once")', async () => {
		const invoice = {
			id: 'inv-1',
			contractId: 'contract-1',
			status: 'open',
			amountCents: 15_000,
			currency: 'usd',
			createdAt: '2026-01-01T00:00:00Z',
			reference: 'INV-0001',
			billingMode: 'by_hand'
		};
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(invoice));

		await createInvoice(fetcher, 'practice-1', 'eng-1', 'by_hand');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/engagements/eng-1/contract/invoices', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ billingMode: 'by_hand' })
		});
	});
});

describe('loadBillingMode', () => {
	it('returns the chosen mode', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ billingMode: 'stripe' }));

		const result = await loadBillingMode(fetcher, 'practice-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/payments/billing-mode');
		expect(result).toBe('stripe');
	});

	it('returns undefined when the Practice has never chosen one', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({}));

		const result = await loadBillingMode(fetcher, 'practice-1');

		expect(result).toBeUndefined();
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('internal error', 500));

		await expect(loadBillingMode(fetcher, 'practice-1')).rejects.toThrow('internal error');
	});
});

describe('setBillingMode', () => {
	it('PUTs the new mode and returns it', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ billingMode: 'by_hand' }));

		const result = await setBillingMode(fetcher, 'practice-1', 'by_hand');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/payments/billing-mode', {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ billingMode: 'by_hand' })
		});
		expect(result).toBe('by_hand');
	});

	it('throws with the response body text when a non-Owner is refused', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('only a Practice Owner can do that', 403));

		await expect(setBillingMode(fetcher, 'practice-1', 'by_hand')).rejects.toThrow(
			'only a Practice Owner can do that'
		);
	});
});

describe('recordPayment', () => {
	it('POSTs the payment details and returns the recorded Payment', async () => {
		const payment = {
			id: 'pay-1',
			invoiceId: 'inv-1',
			amountCents: 15_000,
			method: 'check',
			note: 'check #204',
			paidAt: '2026-01-01T00:00:00Z',
			createdAt: '2026-01-01T00:00:00Z'
		};
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(payment));

		const result = await recordPayment(fetcher, 'practice-1', 'inv-1', {
			method: 'check',
			note: 'check #204',
			paidOn: '2026-01-01'
		});

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/invoices/inv-1/payments', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ method: 'check', note: 'check #204', paidOn: '2026-01-01' })
		});
		expect(result).toEqual(payment);
	});

	it('throws with the response body text when the Invoice is not open', async () => {
		const fetcher = vi.fn().mockResolvedValue(
			jsonResponse('This Invoice is not open, so nothing can be recorded or changed against it.', 409)
		);

		await expect(
			recordPayment(fetcher, 'practice-1', 'inv-1', { method: 'cash', paidOn: '2026-01-01' })
		).rejects.toThrow('This Invoice is not open, so nothing can be recorded or changed against it.');
	});

	// #1038: InvoiceSection reads a field refusal off RefusalError.details,
	// so this has to throw that type rather than a plain Error, or the
	// details map PostManualPaymentHandler sends (#1037) never reaches it.
	it('throws a RefusalError carrying the refused field, per PostManualPaymentHandler\'s details map', async () => {
		const fetcher = vi.fn().mockResolvedValue(
			jsonResponse(
				{
					code: 'INVALID_ARGUMENT',
					message: 'note is required when method is "other"',
					details: { note: 'Enter a note for "Other"' }
				},
				400
			)
		);

		const rejection = recordPayment(fetcher, 'practice-1', 'inv-1', { method: 'other', paidOn: '2026-01-01' });

		await expect(rejection).rejects.toBeInstanceOf(RefusalError);
		await expect(rejection).rejects.toMatchObject({ details: { note: 'Enter a note for "Other"' } });
	});
});

describe('reversePayment', () => {
	it('POSTs the reason to the reverse action and returns the reversal Payment', async () => {
		const reversal = {
			id: 'pay-2',
			invoiceId: 'inv-1',
			amountCents: -15_000,
			reversedPaymentId: 'pay-1',
			reason: 'logged against the wrong invoice',
			paidAt: '2026-01-02T00:00:00Z',
			createdAt: '2026-01-02T00:00:00Z'
		};
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(reversal));

		const result = await reversePayment(
			fetcher,
			'practice-1',
			'inv-1',
			'pay-1',
			'logged against the wrong invoice'
		);

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/invoices/inv-1/payments/pay-1/reverse', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ reason: 'logged against the wrong invoice' })
		});
		expect(result).toEqual(reversal);
	});

	it('throws with the response body text when the Invoice is not paid', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(jsonResponse('This Invoice is not paid, so there is nothing to reverse.', 409));

		await expect(reversePayment(fetcher, 'practice-1', 'inv-1', 'pay-1', 'reason')).rejects.toThrow(
			'This Invoice is not paid, so there is nothing to reverse.'
		);
	});

	// #1038: matches recordPayment's own RefusalError test above --
	// PostReversePaymentHandler's blank-reason refusal carries a `reason`
	// details entry (#945), and InvoiceSection needs it to reach the
	// Reason field rather than the untargeted summary.
	it('throws a RefusalError carrying the refused field, per PostReversePaymentHandler\'s details map', async () => {
		const fetcher = vi.fn().mockResolvedValue(
			jsonResponse(
				{
					code: 'INVALID_ARGUMENT',
					message: 'reason cannot be blank',
					details: { reason: 'Enter a reason for reversing this payment' }
				},
				400
			)
		);

		const rejection = reversePayment(fetcher, 'practice-1', 'inv-1', 'pay-1', '');

		await expect(rejection).rejects.toBeInstanceOf(RefusalError);
		await expect(rejection).rejects.toMatchObject({
			details: { reason: 'Enter a reason for reversing this payment' }
		});
	});
});

describe('voidInvoice', () => {
	it('POSTs to the void action and returns the new status', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ status: 'void' }));

		const result = await voidInvoice(fetcher, 'practice-1', 'inv-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/invoices/inv-1/void', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: '{}'
		});
		expect(result).toEqual({ status: 'void' });
	});

	it('throws with the response body text on a refusal', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(jsonResponse('A Stripe-backed Invoice cannot be voided or written off here.', 409));

		await expect(voidInvoice(fetcher, 'practice-1', 'inv-1')).rejects.toThrow(
			'A Stripe-backed Invoice cannot be voided or written off here.'
		);
	});
});

describe('writeOffInvoice', () => {
	it('POSTs to the write-off action and returns the new status', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ status: 'uncollectible' }));

		const result = await writeOffInvoice(fetcher, 'practice-1', 'inv-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/invoices/inv-1/write-off', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: '{}'
		});
		expect(result).toEqual({ status: 'uncollectible' });
	});

	it('throws with the response body text on a refusal', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('This Invoice is not open.', 409));

		await expect(writeOffInvoice(fetcher, 'practice-1', 'inv-1')).rejects.toThrow('This Invoice is not open.');
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
