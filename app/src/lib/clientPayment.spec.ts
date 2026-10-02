import { describe, expect, it, vi } from 'vitest';
import { loadClientPayment } from './clientPayment.js';

describe('loadClientPayment', () => {
	it("asks for this Invoice's own payment and returns what the Element mounts with", async () => {
		const body = { clientSecret: 's', stripeAccountId: 'acct_1', publishableKey: 'pk_1' };
		const fetcher = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve(body) });

		await expect(loadClientPayment(fetcher, 'e1', 'i1')).resolves.toEqual(body);
		expect(fetcher).toHaveBeenCalledWith('/api/portal/engagements/e1/invoices/i1/payment');
	});

	it("throws the server's own sentence when it refuses", async () => {
		const fetcher = vi.fn().mockResolvedValue({ ok: false, text: () => Promise.resolve('Your Practice cannot take this payment yet.') });

		await expect(loadClientPayment(fetcher, 'e1', 'i1')).rejects.toThrow('Your Practice cannot take this payment yet.');
	});
});
