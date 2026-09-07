import { describe, expect, it, vi } from 'vitest';
import { initiateDeletion, loadDeletionStatus, restorePractice } from './practiceDeletion.js';
import { jsonResponse as response } from './testResponse.js';

describe('loadDeletionStatus', () => {
	it('reads the status for a Practice with nothing pending', async () => {
		const body = { pending: false, hasUnsettledInvoices: false };
		const fetcher = vi.fn().mockResolvedValue(response(body));

		const result = await loadDeletionStatus(fetcher, 'practice-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/deletion');
		expect(result).toEqual(body);
	});

	it('names an unsettled Invoice standing in the way', async () => {
		const body = { pending: false, hasUnsettledInvoices: true };
		const fetcher = vi.fn().mockResolvedValue(response(body));

		const result = await loadDeletionStatus(fetcher, 'practice-1');

		expect(result).toEqual(body);
	});

	it('reports a pending deletion with its window', async () => {
		const body = {
			pending: true,
			deletionRequestedAt: '2027-01-01T00:00:00Z',
			finalizeAt: '2027-01-31T00:00:00Z',
			hasUnsettledInvoices: false
		};
		const fetcher = vi.fn().mockResolvedValue(response(body));

		const result = await loadDeletionStatus(fetcher, 'practice-1');

		expect(result).toEqual(body);
	});

	it('throws with the response body text on a refusal', async () => {
		const fetcher = vi.fn().mockResolvedValue(response('only a Practice Owner can do that', 403));

		await expect(loadDeletionStatus(fetcher, 'practice-1')).rejects.toThrow(
			'only a Practice Owner can do that'
		);
	});
});

describe('initiateDeletion', () => {
	it('posts to the deletion path with X-Confirmed and returns the window', async () => {
		const outcome = { deletionRequestedAt: '2027-01-01T00:00:00Z', finalizeAt: '2027-01-31T00:00:00Z' };
		const fetcher = vi.fn().mockResolvedValue(response(outcome));

		const result = await initiateDeletion(fetcher, 'practice-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/deletion', {
			method: 'POST',
			headers: { 'X-Confirmed': 'true' }
		});
		expect(result).toEqual(outcome);
	});

	it('throws with the response body text on a 409 -- already pending, already deleted, or an unsettled invoice', async () => {
		const fetcher = vi.fn().mockResolvedValue(response("this practice's deletion is already pending", 409));

		await expect(initiateDeletion(fetcher, 'practice-1')).rejects.toThrow(
			"this practice's deletion is already pending"
		);
	});
});

describe('restorePractice', () => {
	it('sends DELETE to the deletion path with no confirmation header', async () => {
		const fetcher = vi.fn().mockResolvedValue(response(undefined, 204));

		await restorePractice(fetcher, 'practice-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/deletion', { method: 'DELETE' });
	});

	it('throws with the response body text when nothing is pending to restore', async () => {
		const fetcher = vi.fn().mockResolvedValue(response('this practice has no pending deletion to restore', 409));

		await expect(restorePractice(fetcher, 'practice-1')).rejects.toThrow(
			'this practice has no pending deletion to restore'
		);
	});
});
