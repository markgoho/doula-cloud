import { describe, expect, it, vi } from 'vitest';
import { centsToDollars, dollarsToCents, loadRates, saveRate } from './rates.js';
import { jsonResponse } from './testResponse.js';

describe('loadRates', () => {
	it('fetches the practice path and returns the decoded rates', async () => {
		const rates = {
			rates: [
				{ kind: 'birth', amountCents: 15_000 },
				// eslint-disable-next-line unicorn/no-null -- a real API response's JSON null, for a kind with no rate set
				{ kind: 'postpartum', amountCents: null }
			]
		};
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(rates));

		const result = await loadRates(fetcher, 'practice-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/rates');
		expect(result).toEqual(rates);
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('internal error', 500));

		await expect(loadRates(fetcher, 'practice-1')).rejects.toThrow('internal error');
	});
});

describe('saveRate', () => {
	it('PUTs amountCents as JSON to the practice/kind path', async () => {
		const saved = { kind: 'birth', amountCents: 15_000 };
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(saved));

		const result = await saveRate(fetcher, 'practice-1', 'birth', 15_000);

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/rates/birth', {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ amountCents: 15_000 })
		});
		expect(result).toEqual(saved);
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('only a Practice Owner or Admin can do that', 403));

		await expect(saveRate(fetcher, 'practice-1', 'birth', 15_000)).rejects.toThrow(
			'only a Practice Owner or Admin can do that'
		);
	});
});

describe('dollarsToCents', () => {
	it('converts a whole-dollar string to cents', () => {
		expect(dollarsToCents('150')).toBe(15_000);
	});

	it('converts a fractional-dollar string to cents, rounding', () => {
		expect(dollarsToCents('150.005')).toBe(15_001);
	});

	it('rejects zero', () => {
		expect(dollarsToCents('0')).toBeUndefined();
	});

	it('rejects a negative amount', () => {
		expect(dollarsToCents('-5')).toBeUndefined();
	});

	it('rejects a non-numeric string', () => {
		expect(dollarsToCents('not a number')).toBeUndefined();
	});
});

describe('centsToDollars', () => {
	it('renders a whole-dollar amount with two decimal places', () => {
		expect(centsToDollars(15_000)).toBe('150.00');
	});

	it('renders a fractional-dollar amount', () => {
		expect(centsToDollars(15_050)).toBe('150.50');
	});
});
