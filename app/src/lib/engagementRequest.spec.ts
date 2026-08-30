import { describe, expect, it, vi } from 'vitest';
import { requestEngagement, type NewEngagementRequest } from './engagementRequest.js';
import { jsonResponse as response } from './testResponse.js';

const body: NewEngagementRequest = { kind: 'birth', dueDate: '2027-03-01', note: 'Referred by the hospital' };

describe('requestEngagement', () => {
	it('posts the kind, due date and note to the client engagement-requests path', async () => {
		const outcome = { requestId: 'request-1', state: 'pending' };
		const fetcher = vi.fn().mockResolvedValue(response(outcome));

		const result = await requestEngagement(fetcher, 'practice-1', 'client-1', body);

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/clients/client-1/engagement-requests', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(body)
		});
		expect(result).toEqual({ noCredits: false, outcome });
	});

	it('reports the collapsed approved state and its Engagement id', async () => {
		const outcome = { requestId: 'request-1', state: 'approved', engagementId: 'engagement-1' };
		const fetcher = vi.fn().mockResolvedValue(response(outcome));

		const result = await requestEngagement(fetcher, 'practice-1', 'client-1', body);

		expect(result).toEqual({ noCredits: false, outcome });
	});

	it('carries the second-live-Engagement warning through unchanged', async () => {
		const outcome = { requestId: 'request-1', state: 'pending', warning: 'this client already has a live engagement' };
		const fetcher = vi.fn().mockResolvedValue(response(outcome));

		const result = await requestEngagement(fetcher, 'practice-1', 'client-1', body);

		expect(result).toEqual({ noCredits: false, outcome });
	});

	it('decodes a 402 into a named noCredits outcome rather than throwing', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(response('no credits remaining, ask a practice owner or admin to buy more', 402));

		const result = await requestEngagement(fetcher, 'practice-1', 'client-1', body);

		expect(result).toEqual({ noCredits: true });
	});

	it('throws with the response body text on a non-402, non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(response('client not found', 404));

		await expect(requestEngagement(fetcher, 'practice-1', 'client-1', body)).rejects.toThrow('client not found');
	});

	it('throws on a duplicate-pending-request conflict', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(response('a pending request for this client and kind already exists', 409));

		await expect(requestEngagement(fetcher, 'practice-1', 'client-1', body)).rejects.toThrow(
			'a pending request for this client and kind already exists'
		);
	});
});
