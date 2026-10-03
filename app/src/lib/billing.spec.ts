import { describe, expect, it, vi } from 'vitest';
import {
	creditCount,
	formatSignedQuantity,
	loadBalance,
	loadLedgerPage,
	originLabel,
	purchaseCredits,
	type LedgerEntry
} from './billing.js';
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

describe('loadLedgerPage', () => {
	it('fetches the practice billing path with the cursor and returns the ledger page', async () => {
		const ledger = {
			items: [{ origin: 'purchase', quantity: 5, createdAt: '2026-08-16T00:00:00Z' }],
			hasMore: true,
			nextCursor: 'cursor-2'
		};
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ balance: 8, ledger }));

		const result = await loadLedgerPage(fetcher, 'practice-1', 'cursor-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/billing?cursor=cursor-1');
		expect(result).toEqual(ledger);
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('the practice is gone', 403));

		await expect(loadLedgerPage(fetcher, 'practice-1', 'cursor-1')).rejects.toThrow('the practice is gone');
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

describe('formatSignedQuantity', () => {
	it('prefixes a credit with +', () => {
		expect(formatSignedQuantity(20)).toBe('+20');
	});

	it('leaves a debit as its own negative sign', () => {
		expect(formatSignedQuantity(-1)).toBe('-1');
	});

	it('adds no sign to zero', () => {
		expect(formatSignedQuantity(0)).toBe('0');
	});
});

describe('creditCount', () => {
	const bonus: LedgerEntry = { origin: 'signup_bonus', quantity: 3, createdAt: '2026-10-01T00:00:00Z' };
	const spent: LedgerEntry = { origin: 'consumption', quantity: -1, createdAt: '2026-10-02T00:00:00Z' };
	const onePage = (items: LedgerEntry[]) => ({ items, hasMore: false });

	it('calls the signup bonus alone Welcome credits', () => {
		expect(creditCount({ balance: 3, ledger: onePage([bonus]) })).toBe('3 Welcome credits');
	});

	it('still calls it Welcome credits once some of it is spent', () => {
		expect(creditCount({ balance: 2, ledger: onePage([spent, bonus]) })).toBe('2 Welcome credits');
		expect(creditCount({ balance: 1, ledger: onePage([spent, spent, bonus]) })).toBe('1 Welcome credit');
	});

	it('says Credits once a founding grant or a purchase is in the balance', () => {
		const grant: LedgerEntry = { origin: 'founding_grant', quantity: 3, createdAt: '2026-10-03T00:00:00Z' };
		const purchase: LedgerEntry = { origin: 'purchase', quantity: 5, createdAt: '2026-10-03T00:00:00Z' };
		expect(creditCount({ balance: 6, ledger: onePage([grant, bonus]) })).toBe('6 Credits');
		expect(creditCount({ balance: 8, ledger: onePage([purchase, bonus]) })).toBe('8 Credits');
	});

	it('says Credits where the ledger is longer than its first page, rather than guess', () => {
		expect(creditCount({ balance: 3, ledger: { items: [bonus], hasMore: true } })).toBe('3 Credits');
	});

	it('says Credits where no signup bonus is on the ledger, or nothing is left of it', () => {
		expect(creditCount({ balance: 1284, ledger: onePage([]) })).toBe('1284 Credits');
		expect(creditCount({ balance: 1, ledger: onePage([]) })).toBe('1 Credit');
		expect(creditCount({ balance: 0, ledger: onePage([spent, spent, spent, bonus]) })).toBe('0 Credits');
	});
});
