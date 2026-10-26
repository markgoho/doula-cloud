import { describe, expect, it, vi } from 'vitest';
import { CARD_PAYMENTS_NOT_SET_UP, loadClientPayment } from './clientPayment.js';

function refusal(code: string, message: string) {
	return Response.json({ code, message }, { status: 409 });
}

describe('loadClientPayment', () => {
	it("asks for this Invoice's own payment and returns what the Element mounts with", async () => {
		const body = { clientSecret: 's', stripeAccountId: 'acct_1', publishableKey: 'pk_1' };
		const fetcher = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve(body) });

		await expect(loadClientPayment(fetcher, 'e1', 'i1')).resolves.toEqual({ kind: 'ready', payment: body });
		expect(fetcher).toHaveBeenCalledWith('/api/portal/engagements/e1/invoices/i1/payment');
	});

	// #2039: the Practice's setup is a fact to state, read off the code.
	it('answers notSetUp when the Practice cannot take a card yet', async () => {
		const fetcher = vi.fn().mockResolvedValue(refusal(CARD_PAYMENTS_NOT_SET_UP, 'Your Practice cannot take this payment yet.'));

		await expect(loadClientPayment(fetcher, 'e1', 'i1')).resolves.toEqual({ kind: 'notSetUp' });
	});

	it("throws the server's own sentence for any other refusal", async () => {
		const fetcher = vi.fn().mockResolvedValue(refusal('FAILED_PRECONDITION', 'This Invoice is not waiting for a payment.'));

		await expect(loadClientPayment(fetcher, 'e1', 'i1')).rejects.toThrow('This Invoice is not waiting for a payment.');
	});

	it('throws the raw body when the refusal is not the JSON shape', async () => {
		const fetcher = vi.fn().mockResolvedValue(new Response('We could not start this payment. Try again.', { status: 502 }));

		await expect(loadClientPayment(fetcher, 'e1', 'i1')).rejects.toThrow('We could not start this payment. Try again.');
	});
});
