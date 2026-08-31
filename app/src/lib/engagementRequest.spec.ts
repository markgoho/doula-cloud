import { describe, expect, it, vi } from 'vitest';
import {
	approveRequest,
	formatCalendarDay,
	formatInstant,
	kindLabel,
	loadApprovalDetail,
	loadPendingRequests,
	refuseRequest,
	requestEngagement,
	withdrawRequest,
	type NewEngagementRequest
} from './engagementRequest.js';
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

describe('loadApprovalDetail', () => {
	const detail = {
		requestId: 'request-1',
		state: 'pending',
		kind: 'birth',
		dueDate: '2027-03-01',
		requestedBy: 'staff-1',
		requestedByName: 'Ada Doula',
		requestedAt: '2026-08-01T10:00:00Z',
		client: {
			clientId: 'client-1',
			givenName: 'Mara',
			familyName: 'Quinn',
			preferredName: '',
			isNewToPractice: true
		},
		creditCost: 1,
		balance: 3,
		balanceAfter: 2,
		engagements: []
	};

	it('reads one Request from the practice-scoped engagement-requests path', async () => {
		const fetcher = vi.fn().mockResolvedValue(response(detail));

		await expect(loadApprovalDetail(fetcher, 'practice-1', 'request-1')).resolves.toEqual(detail);

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/engagement-requests/request-1');
	});

	it('throws with the response body text when the Request has already been decided', async () => {
		const fetcher = vi.fn().mockResolvedValue(response('that request is no longer pending -- it is approved', 409));

		await expect(loadApprovalDetail(fetcher, 'practice-1', 'request-1')).rejects.toThrow(
			'that request is no longer pending -- it is approved'
		);
	});
});

describe('approveRequest', () => {
	it('posts to the approve path and reports the Engagement created', async () => {
		const outcome = { requestId: 'request-1', engagementId: 'engagement-1', state: 'approved' };
		const fetcher = vi.fn().mockResolvedValue(response(outcome));

		const result = await approveRequest(fetcher, 'practice-1', 'request-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/engagement-requests/request-1/approve', {
			method: 'POST'
		});
		expect(result).toEqual({ noCredits: false, outcome });
	});

	it('decodes a 402 into a named noCredits outcome rather than throwing', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(response('no credits remaining, ask a practice owner or admin to buy more', 402));

		await expect(approveRequest(fetcher, 'practice-1', 'request-1')).resolves.toEqual({ noCredits: true });
	});

	it('throws with the response body text on any other refusal', async () => {
		const fetcher = vi.fn().mockResolvedValue(response('that request is no longer pending', 409));

		await expect(approveRequest(fetcher, 'practice-1', 'request-1')).rejects.toThrow(
			'that request is no longer pending'
		);
	});
});

describe('refuseRequest', () => {
	it('posts the reason to the refuse path', async () => {
		const fetcher = vi.fn().mockResolvedValue(response({ requestId: 'request-1', state: 'refused' }));

		await refuseRequest(fetcher, 'practice-1', 'request-1', 'We have no capacity in March');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/engagement-requests/request-1/refuse', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ reason: 'We have no capacity in March' })
		});
	});

	it('throws with the response body text on a refusal', async () => {
		const fetcher = vi.fn().mockResolvedValue(response('reason is required', 400));

		await expect(refuseRequest(fetcher, 'practice-1', 'request-1', '')).rejects.toThrow('reason is required');
	});
});

describe('withdrawRequest', () => {
	it('posts to the withdraw path', async () => {
		const fetcher = vi.fn().mockResolvedValue(response({ requestId: 'request-1', state: 'withdrawn' }));

		await withdrawRequest(fetcher, 'practice-1', 'request-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/engagement-requests/request-1/withdraw', {
			method: 'POST'
		});
	});

	it('throws with the response body text on a refusal', async () => {
		const fetcher = vi.fn().mockResolvedValue(response('only the staff member who made this request may withdraw it', 403));

		await expect(withdrawRequest(fetcher, 'practice-1', 'request-1')).rejects.toThrow(
			'only the staff member who made this request may withdraw it'
		);
	});
});


describe('loadPendingRequests', () => {
	const pendingPage = {
		items: [
			{
				requestId: 'request-1',
				clientId: 'client-1',
				clientName: 'Marguerite Ashworth-Delacroix',
				kind: 'birth',
				dueDate: '2027-03-01',
				requestedByName: 'Tasha Bell',
				requestedAt: '2026-08-01T10:00:00Z'
			}
		],
		hasMore: false
	};

	it('reads the Practice inbox with no cursor on the first page', async () => {
		const fetcher = vi.fn().mockResolvedValue(response(pendingPage));

		await expect(loadPendingRequests(fetcher, 'practice-1')).resolves.toEqual(pendingPage);
		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/engagement-requests');
	});

	it('carries a cursor through, encoded, so its base64 padding survives', async () => {
		const fetcher = vi.fn().mockResolvedValue(response(pendingPage));

		await loadPendingRequests(fetcher, 'practice-1', 'MjAyNnxhYmM=');

		expect(fetcher).toHaveBeenCalledWith(
			'/api/practices/practice-1/engagement-requests?cursor=MjAyNnxhYmM%3D'
		);
	});

	it('throws with the response body text when the reader may not decide Requests', async () => {
		const fetcher = vi.fn().mockResolvedValue(response('forbidden', 403));

		await expect(loadPendingRequests(fetcher, 'practice-1')).rejects.toThrow('forbidden');
	});
});

describe('the words and dates both Request screens print', () => {
	it.each([
		['birth', 'Birth'],
		['postpartum', 'Postpartum'],
		['something-new', 'something-new']
	])('labels the %s kind as %s', (kind, expected) => {
		expect(kindLabel(kind)).toBe(expected);
	});

	// The one that matters: a due date is a calendar day, so it must read
	// as the day it was typed no matter which zone the reader sits in.
	it('reads a calendar day as its own parts, not as UTC midnight', () => {
		expect(formatCalendarDay('2027-03-01')).toBe(
			new Date(2027, 2, 1).toLocaleDateString(undefined, {
				year: 'numeric',
				month: 'short',
				day: 'numeric'
			})
		);
	});

	it('reads a timestamp as an instant in the reader own zone', () => {
		expect(formatInstant('2026-08-01T10:00:00Z')).toBe(
			new Date('2026-08-01T10:00:00Z').toLocaleDateString(undefined, {
				year: 'numeric',
				month: 'short',
				day: 'numeric'
			})
		);
	});
});
