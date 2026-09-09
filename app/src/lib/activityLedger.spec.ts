import { describe, expect, it, vi } from 'vitest';

import { jsonResponse } from './testResponse.js';
import {
	activityLedgerColumns,
	clientActivityLedgerColumns,
	describeActivityAction,
	loadEngagementActivityPage,
	loadPortalActivityPage,
	loadPracticeActivityPage,
	type ActivityEntry
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

const entry = (overrides: Partial<ActivityEntry> = {}): ActivityEntry => ({
	action: 'visit_reassigned',
	actorKind: 'staff',
	actorName: 'Renata Ruiz',
	createdAt: '2026-03-04T10:00:00Z',
	...overrides
});

describe('activityLedgerColumns', () => {
	it('shows the reassignment as a move between two named people', () => {
		const what = activityLedgerColumns()[1];

		expect(what.accessor(entry({ detail: 'Visit reassigned from Ana Silva to Mira Osei' }))).toBe(
			'Visit reassigned from Ana Silva to Mira Osei'
		);
	});

	it('falls back to the generic description for an entry carrying no detail', () => {
		const what = activityLedgerColumns()[1];

		expect(what.accessor(entry({ action: 'invoice_raised' }))).toBe('Invoice raised');
	});
});

describe('clientActivityLedgerColumns (#708)', () => {
	it('keeps the staff column set everywhere but the event text', () => {
		const staff = activityLedgerColumns();
		const client = clientActivityLedgerColumns();

		expect(client.map((column) => column.label)).toEqual(staff.map((column) => column.label));
		expect(client[0].accessor(entry())).toBe(staff[0].accessor(entry()));
		expect(client[2].accessor(entry())).toBe(staff[2].accessor(entry()));
	});

	it('says the register phrase, not the raw action, for the row the ticket names', () => {
		const what = clientActivityLedgerColumns()[1];

		expect(what.accessor(entry({ action: 'plan_instance_edited' }))).toBe('Your Birth Plan was updated.');
	});

	it("ignores a detail sentence, which is the staff register's own prose", () => {
		const what = clientActivityLedgerColumns()[1];

		expect(
			what.accessor(
				entry({ action: 'visit_logged', detail: 'Visit reassigned from Ana Silva to Mira Osei' })
			)
		).toBe('A visit was added to your care.');
	});

	it('refuses an action the register does not phrase', () => {
		const what = clientActivityLedgerColumns()[1];

		expect(() => what.accessor(entry({ action: 'offer_sent' }))).toThrow('no Client wording');
	});
});
