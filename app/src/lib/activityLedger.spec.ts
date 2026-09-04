import { describe, expect, it, vi } from 'vitest';

import { jsonResponse } from './testResponse.js';
import {
	describeActivityAction,
	loadEngagementActivityPage,
	loadPortalActivityPage,
	loadPracticeActivityPage
} from './activityLedger.js';
import type { EngagementReference } from './engagementDetail.js';

describe('describeActivityAction', () => {
	it.each([
		['invoice_raised', 'Invoice raised'],
		['contract_signed', 'Contract signed'],
		['created', 'Created'],
		['engagement_created', 'Engagement created']
	])('turns %s into %s', (action, want) => {
		expect(describeActivityAction(action)).toBe(want);
	});
});

const reference: EngagementReference = { practiceId: 'practice-1', engagementId: 'engagement-1' };

describe('loadPracticeActivityPage', () => {
	it('reads the first page without a cursor parameter', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ items: [], hasMore: false }));

		await loadPracticeActivityPage(fetcher, 'practice-1', '');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/activity');
	});

	it('encodes a cursor onto the next page', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ items: [], hasMore: false }));

		await loadPracticeActivityPage(fetcher, 'practice-1', 'a b/c');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/activity?cursor=a%20b%2Fc');
	});

	it('throws a refusal, so PaginatedList can catch it', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('nope', 403));

		await expect(loadPracticeActivityPage(fetcher, 'practice-1', '')).rejects.toThrow('nope');
	});
});

describe('loadEngagementActivityPage', () => {
	it('reads the record-scoped path', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ items: [], hasMore: false }));

		await loadEngagementActivityPage(fetcher, reference, '');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/engagements/engagement-1/activity');
	});

	it('encodes a cursor onto the next page', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ items: [], hasMore: false }));

		await loadEngagementActivityPage(fetcher, reference, 'a b/c');

		expect(fetcher).toHaveBeenCalledWith(
			'/api/practices/practice-1/engagements/engagement-1/activity?cursor=a%20b%2Fc'
		);
	});

	it('throws a refusal, so PaginatedList can catch it', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('nope', 403));

		await expect(loadEngagementActivityPage(fetcher, reference, '')).rejects.toThrow('nope');
	});
});

describe('loadPortalActivityPage', () => {
	it('reads the portal path', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ items: [], hasMore: false }));

		await loadPortalActivityPage(fetcher, 'engagement-1', '');

		expect(fetcher).toHaveBeenCalledWith('/api/portal/engagements/engagement-1/activity');
	});

	it('encodes a cursor onto the next page', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ items: [], hasMore: false }));

		await loadPortalActivityPage(fetcher, 'engagement-1', 'a b/c');

		expect(fetcher).toHaveBeenCalledWith('/api/portal/engagements/engagement-1/activity?cursor=a%20b%2Fc');
	});

	it('throws a refusal, so PaginatedList can catch it', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('nope', 403));

		await expect(loadPortalActivityPage(fetcher, 'engagement-1', '')).rejects.toThrow('nope');
	});
});
