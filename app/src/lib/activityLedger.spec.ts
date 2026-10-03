import { describe, expect, it, vi } from 'vitest';

import { formatCalendarDay } from './dates.js';
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

// eslint-disable-next-line unicorn/no-null -- the write side records an empty side of a change as JSON null, never an absent key
const recordedNull = null;

describe('activityLedgerColumns: an Engagement fact, from what to what (#1423)', () => {
	const what = activityLedgerColumns()[1];

	it.each([
		['care_phase_changed', { statusBefore: 'intake', statusAfter: 'active' }, 'Care phase changed: Intake to Active'],
		[
			'engagement_completed',
			{
				statusBefore: 'active',
				statusAfter: 'completed',
				endingReasonBefore: recordedNull,
				endingReasonAfter: 'care_complete',
				endingNoteBefore: recordedNull,
				endingNoteAfter: 'Baby arrived safely'
			},
			'Engagement completed: Active to Completed. Reason: The work finished as agreed'
		],
		[
			'engagement_completed',
			{ statusBefore: 'intake', statusAfter: 'completed' },
			'Engagement completed: Intake to Completed'
		],
		[
			'engagement_reopened',
			{ statusBefore: 'completed', statusAfter: 'active', endingReasonBefore: 'client_withdrew', endingReasonAfter: recordedNull },
			'Engagement reopened: Completed to Active'
		],
		['kind_changed', { kindBefore: 'birth', kindAfter: 'postpartum' }, 'Kind changed: Birth to Postpartum'],
		[
			'birth_outcome_recorded',
			{
				birthOutcomeBefore: recordedNull,
				birthOutcomeAfter: 'unknown',
				pregnancyEndedOnBefore: recordedNull,
				pregnancyEndedOnAfter: recordedNull
			},
			'Birth outcome recorded: Not recorded to The Practice never learned what happened'
		],
		[
			'birth_outcome_recorded',
			{
				birthOutcomeBefore: 'loss',
				birthOutcomeAfter: recordedNull,
				pregnancyEndedOnBefore: '2027-03-04',
				pregnancyEndedOnAfter: recordedNull
			},
			`Birth outcome recorded: The pregnancy ended without a living baby, ${formatCalendarDay('2027-03-04')} to Not recorded`
		]
	])('reads %s off its diff', (action, diff, want) => {
		expect(what.accessor(entry({ action, diff }))).toBe(want);
	});

	it('shows an unlabeled value as it is stored, rather than dropping the change', () => {
		expect(what.accessor(entry({ action: 'care_phase_changed', diff: { statusBefore: 'intake', statusAfter: 'paused' } }))).toBe(
			'Care phase changed: Intake to paused'
		);
		expect(
			what.accessor(
				entry({
					action: 'engagement_completed',
					diff: { statusBefore: 'active', statusAfter: 'completed', endingReasonAfter: 'moved_away' }
				})
			)
		).toBe('Engagement completed: Active to Completed. Reason: moved_away');
	});

	it.each([
		['no diff at all, as the practice-wide feed sends', 'engagement_completed', undefined],
		['no before side', 'kind_changed', { kindAfter: 'birth' }],
		['no after side', 'kind_changed', { kindBefore: 'birth' }],
		['no status move, only a reason', 'engagement_completed', { endingReasonAfter: 'care_complete' }]
	])('falls back to the action alone given %s', (_case, action, diff) => {
		expect(what.accessor(entry({ action, diff }))).toBe(describeActivityAction(action));
	});

	it('reads no diff for an action that is not one of these facts', () => {
		expect(what.accessor(entry({ action: 'visit_scheduled', diff: { scheduledAtBefore: recordedNull } }))).toBe(
			'Visit scheduled'
		);
	});

	it('never puts the ending note in the sentence', () => {
		const diff = { statusBefore: 'active', statusAfter: 'completed', endingNoteAfter: 'private words' };

		expect(what.accessor(entry({ action: 'engagement_completed', diff }))).not.toContain('private words');
	});
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

	it('names the person a roster change happened to (#1148)', () => {
		const what = activityLedgerColumns()[1];

		expect(
			what.accessor(
				entry({ action: 'roles_changed', subjectKind: 'membership', subjectName: 'Renata Alvarez' })
			)
		).toBe('Roles changed — Renata Alvarez');
	});

	it('leaves an entry with no subject name exactly as it reads today (#1148)', () => {
		const what = activityLedgerColumns()[1];

		expect(what.accessor(entry({ action: 'invoice_raised', subjectKind: 'engagement' }))).toBe(
			'Invoice raised'
		);
	});

	it("does not append a subject name to the server's own detail sentence (#1148)", () => {
		const what = activityLedgerColumns()[1];

		expect(
			what.accessor(
				entry({ detail: 'Visit reassigned from Ana Silva to Mira Osei', subjectName: 'Renata Alvarez' })
			)
		).toBe('Visit reassigned from Ana Silva to Mira Osei');
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

	it('never names a Staff member a subject name would carry (#1148)', () => {
		const what = clientActivityLedgerColumns()[1];

		expect(
			what.accessor(entry({ action: 'plan_instance_edited', subjectName: 'Renata Alvarez' }))
		).toBe('Your Birth Plan was updated.');
	});

	it('refuses an action the register does not phrase', () => {
		const what = clientActivityLedgerColumns()[1];

		expect(() => what.accessor(entry({ action: 'offer_sent' }))).toThrow('no Client wording');
	});
});
