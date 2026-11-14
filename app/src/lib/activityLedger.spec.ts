import { describe, expect, it, vi } from 'vitest';

import { formatCalendarDay, formatInstantWithClock, formatInstantWithSeconds } from './dates.js';
import { jsonResponse } from './testResponse.js';
import {
	activityLedgerColumns,
	clientActivityLedgerColumns,
	describeActivityAction,
	loadEngagementActivityPage,
	loadPortalActivityPage,
	hasLedgerRowClockTime,
	loadPracticeActivityPage,
	type ActivityEntry
} from './activityLedger.js';
import type { EngagementReference } from './engagementDetail.js';
import { staffActivityPhrase } from './staffActivityPhrases.js';

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
	const what = activityLedgerColumns('America/New_York')[1];

	it.each([
		['care_phase_changed', { statusBefore: 'intake', statusAfter: 'active' }, 'Care phase changed for the Client: Intake to Active'],
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
			'Care for the Client ended: Active to Completed. Reason: The work finished as agreed'
		],
		[
			'engagement_completed',
			{ statusBefore: 'intake', statusAfter: 'completed' },
			'Care for the Client ended: Intake to Completed'
		],
		[
			'engagement_reopened',
			{ statusBefore: 'completed', statusAfter: 'active', endingReasonBefore: 'client_withdrew', endingReasonAfter: recordedNull },
			'Care for the Client reopened: Completed to Active'
		],
		['kind_changed', { kindBefore: 'birth', kindAfter: 'postpartum' }, 'Type of care changed for the Client: Birth to Postpartum'],
		[
			'due_date_changed',
			{ dueDateBefore: '2027-03-01', dueDateAfter: '2027-03-15' },
			`Due date changed for the Client: ${formatCalendarDay('2027-03-01')} to ${formatCalendarDay('2027-03-15')}`
		],
		// #2118: a first due date reads from "Not given", the word the
		// summary card's Due row and the Request pages use for the same gap.
		[
			'due_date_changed',
			{ dueDateBefore: recordedNull, dueDateAfter: '2027-03-15' },
			`Due date changed for the Client: Not given to ${formatCalendarDay('2027-03-15')}`
		],
		[
			'birth_outcome_recorded',
			{
				birthOutcomeBefore: recordedNull,
				birthOutcomeAfter: 'unknown',
				pregnancyEndedOnBefore: recordedNull,
				pregnancyEndedOnAfter: recordedNull
			},
			'Birth outcome recorded for the Client: Not recorded to The Practice never learned what happened'
		],
		[
			'birth_outcome_recorded',
			{
				birthOutcomeBefore: 'loss',
				birthOutcomeAfter: recordedNull,
				pregnancyEndedOnBefore: '2027-03-04',
				pregnancyEndedOnAfter: recordedNull
			},
			`Birth outcome recorded for the Client: The pregnancy ended without a living baby, ${formatCalendarDay('2027-03-04')} to Not recorded`
		]
	])('reads %s off its diff', (action, diff, want) => {
		expect(what.accessor(entry({ action, diff }))).toBe(want);
	});

	it('shows an unlabeled value as it is stored, rather than dropping the change', () => {
		expect(what.accessor(entry({ action: 'care_phase_changed', diff: { statusBefore: 'intake', statusAfter: 'paused' } }))).toBe(
			'Care phase changed for the Client: Intake to paused'
		);
		expect(
			what.accessor(
				entry({
					action: 'engagement_completed',
					diff: { statusBefore: 'active', statusAfter: 'completed', endingReasonAfter: 'moved_away' }
				})
			)
		).toBe('Care for the Client ended: Active to Completed. Reason: moved_away');
	});

	it.each([
		['no diff at all, as the practice-wide feed sends', 'engagement_completed', undefined],
		['no before side', 'kind_changed', { kindAfter: 'birth' }],
		['no after side', 'kind_changed', { kindBefore: 'birth' }],
		['no status move, only a reason', 'engagement_completed', { endingReasonAfter: 'care_complete' }]
	])('falls back to the action alone given %s', (_case, action, diff) => {
		expect(what.accessor(entry({ action, diff }))).toBe(staffActivityPhrase(action));
	});

	it('reads no diff for an action that is not one of these facts', () => {
		expect(what.accessor(entry({ action: 'visit_scheduled', diff: { scheduledAtBefore: recordedNull } }))).toBe(
			'Visit time for the Client updated'
		);
	});

	// #2066: a plan save says which plan from its action alone.
	it('names the plan a save changed', () => {
		expect(what.accessor(entry({ action: 'care_plan_edited' }))).toBe('Care Plan for the Client changed');
		expect(what.accessor(entry({ action: 'birth_plan_edited', subjectName: 'Amara' }))).toBe(
			'Birth Plan for Amara changed'
		);
	});

	it('never puts the ending note in the sentence', () => {
		const diff = { statusBefore: 'active', statusAfter: 'completed', endingNoteAfter: 'private words' };

		expect(what.accessor(entry({ action: 'engagement_completed', diff }))).not.toContain('private words');
	});
});

describe('activityLedgerColumns', () => {
	it('shows the reassignment as a move between two named people', () => {
		const what = activityLedgerColumns('America/New_York')[1];

		expect(what.accessor(entry({ detail: 'Visit reassigned from Ana Silva to Mira Osei' }))).toBe(
			'Visit reassigned from Ana Silva to Mira Osei'
		);
	});

	describe('which Visit a reassignment moved (#2666)', () => {
		const detail = 'Visit reassigned from Ana Silva to Mira Osei';
		const scheduledAt = '2026-11-13T21:00:00Z';
		const addedAt = '2026-11-02T10:11:00Z';

		it('names a dated Visit by its date, on the Engagement ledger from the diff', () => {
			const what = activityLedgerColumns('America/New_York')[1];

			expect(what.accessor(entry({ detail, diff: { visitScheduledAt: scheduledAt, visitCreatedAt: addedAt } }))).toBe(
				`Visit on ${formatInstantWithClock(scheduledAt, 'America/New_York')} reassigned from Ana Silva to Mira Osei`
			);
		});

		it('names a Visit with no date by when it was added', () => {
			const what = activityLedgerColumns('America/New_York')[1];

			expect(what.accessor(entry({ detail, diff: { visitScheduledAt: recordedNull, visitCreatedAt: addedAt } }))).toBe(
				`Visit not yet scheduled (added ${formatInstantWithSeconds(addedAt, 'America/New_York')}) reassigned from Ana Silva to Mira Osei`
			);
		});

		it('keeps a row written before the Visit was recorded as it was', () => {
			const what = activityLedgerColumns('America/New_York')[1];

			expect(what.accessor(entry({ detail, diff: { assignedStaffIdBefore: 'a', assignedStaffIdAfter: 'b' } }))).toBe(detail);
			expect(what.accessor(entry())).toBe('Visit for the Client given to another doula');
		});

		it('names the Visit on the Practice-wide feed from its two fields', () => {
			const what = activityLedgerColumns('America/New_York')[1];

			expect(what.accessor(entry({ subjectName: 'Grace Whitfield', visitScheduledAt: scheduledAt, visitAddedAt: addedAt }))).toBe(
				`Visit on ${formatInstantWithClock(scheduledAt, 'America/New_York')} for Grace Whitfield given to another doula`
			);
			expect(what.accessor(entry({ subjectName: 'Grace Whitfield', visitAddedAt: addedAt }))).toBe(
				`Visit not yet scheduled (added ${formatInstantWithSeconds(addedAt, 'America/New_York')}) for Grace Whitfield given to another doula`
			);
		});

		it('names both Doulas on the Practice-wide feed when the row carries them (#2672)', () => {
			const what = activityLedgerColumns('America/New_York')[1];
			const move = { subjectName: 'Grace Whitfield', reassignedFromName: 'Delia Marchetti', reassignedToName: 'Priya Raman' };

			expect(what.accessor(entry({ ...move, visitScheduledAt: scheduledAt, visitAddedAt: addedAt }))).toBe(
				`Visit on ${formatInstantWithClock(scheduledAt, 'America/New_York')} for Grace Whitfield reassigned from Delia Marchetti to Priya Raman`
			);
			expect(what.accessor(entry({ ...move, visitAddedAt: addedAt }))).toBe(
				`Visit not yet scheduled (added ${formatInstantWithSeconds(addedAt, 'America/New_York')}) for Grace Whitfield reassigned from Delia Marchetti to Priya Raman`
			);
			expect(what.accessor(entry({ ...move, reassignedFromName: 'a former colleague' }))).toBe(
				'Visit for Grace Whitfield reassigned from a former colleague to Priya Raman'
			);
		});

		it('leaves any other action alone', () => {
			const what = activityLedgerColumns('America/New_York')[1];

			expect(what.accessor(entry({ action: 'visit_logged', visitScheduledAt: scheduledAt }))).toBe('Visit added for the Client');
		});
	});

	it("says the row's own words for an entry carrying no detail (#2006)", () => {
		const what = activityLedgerColumns('America/New_York')[1];

		expect(what.accessor(entry({ action: 'invoice_raised' }))).toBe('Invoice created for the Client');
	});

	it('names the person a roster change happened to (#1148)', () => {
		const what = activityLedgerColumns('America/New_York')[1];

		expect(
			what.accessor(
				entry({ action: 'roles_changed', subjectKind: 'membership', subjectName: 'Renata Alvarez' })
			)
		).toBe('Roles changed for Renata Alvarez');
	});

	it('names the Client an Engagement row is about (#2006)', () => {
		const what = activityLedgerColumns('America/New_York')[1];

		expect(
			what.accessor(entry({ action: 'invoice_raised', subjectKind: 'engagement', subjectName: 'Amara' }))
		).toBe('Invoice created for Amara');
	});

	it('wraps only the What column, on every ledger (#2098, #2112)', () => {
		expect(activityLedgerColumns('America/New_York').map((column) => Boolean(column.wraps))).toEqual([false, true, false]);
		expect(clientActivityLedgerColumns('America/New_York').map((column) => Boolean(column.wraps))).toEqual([false, true, false]);
	});

	it("does not append a subject name to the server's own detail sentence (#1148)", () => {
		const what = activityLedgerColumns('America/New_York')[1];

		expect(
			what.accessor(
				entry({ detail: 'Visit reassigned from Ana Silva to Mira Osei', subjectName: 'Renata Alvarez' })
			)
		).toBe('Visit reassigned from Ana Silva to Mira Osei');
	});
});

describe('clientActivityLedgerColumns (#708)', () => {
	it('keeps the staff column set everywhere but the event text', () => {
		const staff = activityLedgerColumns('America/New_York');
		const client = clientActivityLedgerColumns('America/New_York');

		expect(client.map((column) => column.label)).toEqual(staff.map((column) => column.label));
		expect(client[0].accessor(entry())).toBe(staff[0].accessor(entry()));
		expect(client[2].accessor(entry())).toBe(staff[2].accessor(entry()));
	});

	it('says the register phrase, not the raw action, for the row the ticket names', () => {
		const what = clientActivityLedgerColumns('America/New_York')[1];

		expect(what.accessor(entry({ action: 'birth_plan_edited' }))).toBe('Your birth plan was updated.');
	});

	it("ignores a detail sentence, which is the staff register's own prose", () => {
		const what = clientActivityLedgerColumns('America/New_York')[1];

		expect(
			what.accessor(
				entry({ action: 'visit_logged', detail: 'Visit reassigned from Ana Silva to Mira Osei' })
			)
		).toBe('A visit was added to your care.');
	});

	it('never names a Staff member a subject name would carry (#1148)', () => {
		const what = clientActivityLedgerColumns('America/New_York')[1];

		expect(
			what.accessor(entry({ action: 'birth_plan_edited', subjectName: 'Renata Alvarez' }))
		).toBe('Your birth plan was updated.');
	});

	it('refuses an action the register does not phrase', () => {
		const what = clientActivityLedgerColumns('America/New_York')[1];

		expect(() => what.accessor(entry({ action: 'offer_sent' }))).toThrow('no Client wording');
	});
});

describe('hasLedgerRowClockTime', () => {
	const now = new Date('2026-08-14T20:12:00Z');
	const row: ActivityEntry = {
		action: 'invoice_raised',
		actorKind: 'staff',
		actorName: 'Mark Goho',
		subjectName: 'Amara',
		createdAt: '2026-08-14T18:12:00Z'
	};

	it('is false for a row that reads "2 hours ago"', () => {
		expect(hasLedgerRowClockTime(row, now)).toBe(false);
	});

	it('is true for a row old enough to read a clock time', () => {
		expect(hasLedgerRowClockTime({ ...row, createdAt: '2026-08-10T18:12:00Z' }, now)).toBe(true);
	});

	it('is true for a recent row that names the time of the Visit it moved', () => {
		expect(
			hasLedgerRowClockTime({ ...row, action: 'visit_reassigned', visitScheduledAt: '2026-08-20T14:00:00Z' }, now)
		).toBe(true);
	});
});
