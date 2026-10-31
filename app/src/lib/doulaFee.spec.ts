import { describe, expect, it, vi } from 'vitest';
import { feePaidLine, loadDoulaFees, recordDoulaFeePaid, removeDoulaFeePaid, type DoulaFee } from './doulaFee.js';
import { RefusalError } from './formErrors.js';
import { jsonResponse } from './testResponse.js';

const base = '/api/practices/practice-1/engagements/eng-1/doula-fees';
const fee: DoulaFee = { attachmentId: 'att-1', doulaName: 'Lena Vasquez', amountCents: 45_000, acceptedOn: '2026-10-27' };

describe('loadDoulaFees', () => {
	it('returns the list for an Owner or Admin', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ items: [fee] }));

		expect(await loadDoulaFees(fetcher, 'practice-1', 'eng-1')).toEqual([fee]);
		expect(fetcher).toHaveBeenCalledWith(base);
	});

	it('answers undefined for a reader the BFF refuses', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ code: 'FORBIDDEN', message: 'no' }, 403));

		expect(await loadDoulaFees(fetcher, 'practice-1', 'eng-1')).toBeUndefined();
	});

	it('throws any other failure', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ message: 'Something went wrong' }, 500));

		await expect(loadDoulaFees(fetcher, 'practice-1', 'eng-1')).rejects.toThrow('Something went wrong');
	});
});

describe('recordDoulaFeePaid', () => {
	it('posts the record to the Attachment', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({}, 201));
		const input = { method: 'cash' as const, paidOn: '2026-10-01' };

		await recordDoulaFeePaid(fetcher, 'practice-1', 'eng-1', 'att-1', input);

		expect(fetcher).toHaveBeenCalledWith(`${base}/att-1/payments`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(input)
		});
	});

	it('throws a refusal carrying its field details', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(
				jsonResponse({ message: 'paidOn cannot be in the future', details: { paidOn: 'The date cannot be in the future' } }, 400)
			);

		await expect(
			recordDoulaFeePaid(fetcher, 'practice-1', 'eng-1', 'att-1', { method: 'cash', paidOn: '2099-01-01' })
		).rejects.toEqual(
			expect.objectContaining({ name: RefusalError.name, details: { paidOn: 'The date cannot be in the future' } })
		);
	});
});

describe('removeDoulaFeePaid', () => {
	it('posts the reason to the payment', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({}, 201));

		await removeDoulaFeePaid(fetcher, 'practice-1', 'eng-1', 'att-1', 'fp-1', 'Wrong birth');

		expect(fetcher).toHaveBeenCalledWith(`${base}/att-1/payments/fp-1/reverse`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ reason: 'Wrong birth' })
		});
	});

	it('throws a refusal', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ message: 'This payment has already been removed.' }, 409));

		await expect(removeDoulaFeePaid(fetcher, 'practice-1', 'eng-1', 'att-1', 'fp-1', 'x')).rejects.toThrow(
			'This payment has already been removed.'
		);
	});
});

describe('feePaidLine', () => {
	it.each([
		['check', '$450.00 on Mar 3, 2027, by check'],
		['bank_transfer', '$450.00 on Mar 3, 2027, by bank transfer'],
		['cash', '$450.00 on Mar 3, 2027, in cash'],
		['other', '$450.00 on Mar 3, 2027, another way']
	] as const)('says how a %s payment was made', (method, want) => {
		expect(feePaidLine({ amountCents: 45_000, method, paidOn: '2027-03-03' })).toBe(want);
	});
});
