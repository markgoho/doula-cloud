import { describe, expect, it, vi } from 'vitest';
import { eraseClient, loadEraseEligibility } from './clientErasure.js';
import { jsonResponse as response } from './testResponse.js';

describe('loadEraseEligibility', () => {
	it('reads the erasure precheck for a Client with nothing blocking her', async () => {
		const body = { unsettledInvoices: [] };
		const fetcher = vi.fn().mockResolvedValue(response(body));

		const result = await loadEraseEligibility(fetcher, 'practice-1', 'client-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/clients/client-1/erasure');
		expect(result).toEqual(body);
	});

	it('names the invoices standing in the way', async () => {
		const body = {
			unsettledInvoices: [
				{ invoiceId: 'invoice-1', status: 'open', amountCents: 150_000, currency: 'usd', createdAt: '2027-01-05T00:00:00Z' }
			]
		};
		const fetcher = vi.fn().mockResolvedValue(response(body));

		const result = await loadEraseEligibility(fetcher, 'practice-1', 'client-1');

		expect(result).toEqual(body);
	});

	it('reports a Client already erased', async () => {
		const body = { erasedAt: '2027-02-01T00:00:00Z', unsettledInvoices: [] };
		const fetcher = vi.fn().mockResolvedValue(response(body));

		const result = await loadEraseEligibility(fetcher, 'practice-1', 'client-1');

		expect(result).toEqual(body);
	});

	it('throws with the response body text on a refusal', async () => {
		const fetcher = vi.fn().mockResolvedValue(response('only a Practice Owner can do that', 403));

		await expect(loadEraseEligibility(fetcher, 'practice-1', 'client-1')).rejects.toThrow(
			'only a Practice Owner can do that'
		);
	});
});

describe('eraseClient', () => {
	it('posts to the erasure path and returns the outcome', async () => {
		const outcome = {
			erasedAt: '2027-02-01T00:00:00Z',
			stripeCustomersQueued: 0,
			portalAccountQueued: false
		};
		const fetcher = vi.fn().mockResolvedValue(response(outcome));

		const result = await eraseClient(fetcher, 'practice-1', 'client-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/clients/client-1/erasure', {
			method: 'POST'
		});
		expect(result).toEqual(outcome);
	});

	it('carries the Stripe redaction eligibility date through unchanged', async () => {
		const outcome = {
			erasedAt: '2027-02-01T00:00:00Z',
			stripeRedactionEligibleAt: '2027-05-02T00:00:00Z',
			stripeCustomersQueued: 1,
			portalAccountQueued: true
		};
		const fetcher = vi.fn().mockResolvedValue(response(outcome));

		const result = await eraseClient(fetcher, 'practice-1', 'client-1');

		expect(result).toEqual(outcome);
	});

	it('throws with the response body text on a 409 -- already erased, or a race on an unsettled invoice', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(response("this client's data has already been erased", 409));

		await expect(eraseClient(fetcher, 'practice-1', 'client-1')).rejects.toThrow(
			"this client's data has already been erased"
		);
	});
});
