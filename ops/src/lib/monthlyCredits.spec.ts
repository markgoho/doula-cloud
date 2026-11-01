import { describe, expect, it, vi } from 'vitest';
import { monthName, readMonthlyCredits } from './monthlyCredits.ts';

function answering(status: number, body: unknown) {
	return vi.fn<typeof fetch>(async () => Response.json(body, { status }));
}

const read = {
	month: '2026-09',
	months: ['2026-09', '2026-08'],
	practices: [{ id: 'p-1', name: 'Rooted Birth Collective', creditsBought: 10, creditsSpent: 2, boughtBefore: true }],
	totals: { creditsBought: 10, creditsSpent: 2, practicesBoughtAgain: 1 }
};

describe('monthName', () => {
	it('writes a month as its name and year', () => {
		expect(monthName('2026-09')).toBe('September 2026');
		expect(monthName('2027-01')).toBe('January 2027');
	});
});

describe('readMonthlyCredits', () => {
	it('asks for the last closed month when none is chosen', async () => {
		const fetcher = answering(200, read);

		await expect(readMonthlyCredits(undefined, fetcher)).resolves.toEqual({ kind: 'ok', value: read });
		expect(fetcher).toHaveBeenCalledWith('/api/credits/monthly', { credentials: 'include' });
	});

	it('asks for the month chosen', async () => {
		const fetcher = answering(200, read);

		await readMonthlyCredits('2026-08', fetcher);

		expect(fetcher).toHaveBeenCalledWith('/api/credits/monthly?month=2026-08', { credentials: 'include' });
	});

	it('reports a refused month without a field, since no month list is shown', async () => {
		const message = 'Choose a month that has ended, written as YYYY-MM.';
		const fetcher = answering(400, { code: 'INVALID_ARGUMENT', message, details: { month: message } });

		await expect(readMonthlyCredits('2026-11', fetcher)).resolves.toEqual({
			kind: 'refused',
			errors: [{ message, field: undefined }]
		});
	});

	it('reports a session that has ended', async () => {
		await expect(readMonthlyCredits(undefined, answering(401, {}))).resolves.toEqual({ kind: 'signed-out' });
	});
});
