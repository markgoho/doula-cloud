import { describe, expect, it, vi } from 'vitest';
import { loadBalance, purchaseCredits } from './billing.js';

// eslint-disable-next-line unicorn/consistent-boolean-name -- mirrors the native Response.ok property this mock stands in for
function jsonResponse(body: unknown, ok = true): Response {
	return {
		ok,
		text: () => Promise.resolve(typeof body === 'string' ? body : JSON.stringify(body)),
		json: () => Promise.resolve(body)
	} as Response;
}

describe('loadBalance', () => {
	it('fetches the practice billing path and returns the decoded balance', async () => {
		const balance = {
			balance: 8,
			ledger: [
				{ origin: 'purchase', quantity: 5, createdAt: '2026-08-16T00:00:00Z' },
				{ origin: 'signup_bonus', quantity: 3, createdAt: '2026-08-01T00:00:00Z' }
			]
		};
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(balance));

		const result = await loadBalance(fetcher, 'practice-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/billing');
		expect(result).toEqual(balance);
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('forbidden', false));

		await expect(loadBalance(fetcher, 'practice-1')).rejects.toThrow('forbidden');
	});
});

describe('purchaseCredits', () => {
	it('posts the quantity to the practice purchases path and returns the checkout URL', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ checkoutUrl: 'https://checkout.stripe.com/session-1' }));

		const result = await purchaseCredits(fetcher, 'practice-1', 5);

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/billing/purchases', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ quantity: 5 })
		});
		expect(result).toBe('https://checkout.stripe.com/session-1');
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('forbidden: owner only', false));

		await expect(purchaseCredits(fetcher, 'practice-1', 5)).rejects.toThrow('forbidden: owner only');
	});
});
