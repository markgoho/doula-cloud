import { describe, expect, it, vi } from 'vitest';
import { loadBalance, originLabel, purchaseCredits } from './billing.js';
import { jsonResponse } from './testResponse.js';

describe('loadBalance', () => {
	it('fetches the practice billing path and returns the decoded balance', async () => {
		const balance = {
			balance: 8,
			ledger: {
				items: [
					{ origin: 'purchase', quantity: 5, createdAt: '2026-08-16T00:00:00Z' },
					{ origin: 'signup_bonus', quantity: 3, createdAt: '2026-08-01T00:00:00Z' }
				],
				hasMore: false
			}
		};
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(balance));

		const result = await loadBalance(fetcher, 'practice-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/billing');
		expect(result).toEqual(balance);
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('forbidden', 403));

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
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('forbidden: owner only', 403));

		await expect(purchaseCredits(fetcher, 'practice-1', 5)).rejects.toThrow('forbidden: owner only');
	});
});

describe('originLabel', () => {
	it('tells a founding grant apart from a signup bonus', () => {
		expect(originLabel('founding_grant')).toBe('Founding member credits');
		expect(originLabel('signup_bonus')).toBe('Welcome credits');
	});

	it('names the other three origins in words rather than enum values', () => {
		expect(originLabel('purchase')).toBe('Purchase');
		expect(originLabel('consumption')).toBe('Engagement started');
		expect(originLabel('refund')).toBe('Refund');
	});

	it('falls back to the raw origin, so an unknown value reads oddly rather than blank', () => {
		expect(originLabel('something_new')).toBe('something_new');
	});
});
