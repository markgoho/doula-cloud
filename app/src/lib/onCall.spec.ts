import { describe, expect, it, vi } from 'vitest';
import {
	clearGap,
	defaultRange,
	describeCoverage,
	describeNoWindow,
	doublyBooked,
	engagementOnCallPath,
	loadEngagementOnCall,
	loadOnCallSettings,
	loadRoster,
	rangeFromParameters,
	rosterHref,
	rosterPath,
	rosterSummary,
	saveGap,
	saveNarrowing,
	saveOnCallSettings,
	toDayValue,
	type Roster,
	type RosterWindow,
} from './onCall.js';
import { jsonResponse } from './testResponse.js';

const today = new Date(2026, 9, 17, 21, 30);

function windowRow(overrides: Partial<RosterWindow> = {}): RosterWindow {
	return {
		engagementId: 'eng-1',
		clientName: 'Ada Whitfield',
		dueDate: '2026-10-30',
		window: { start: '2026-10-09', end: '2026-11-13' },
		onCall: [
			{
				staffId: 'staff-1',
				name: 'Maya Primary',
				from: '2026-10-09',
				to: '2026-11-13',
				narrowed: false,
			},
		],
		gaps: [],
		unstaffedDays: [],
		uncovered: false,
		...overrides,
	};
}

function roster(
	windows: RosterWindow[],
	doulas: Roster['doulas'] = []
): Roster {
	return {
		from: '2026-10-17',
		to: '2026-10-17',
		windows,
		noWindow: [],
		doulas,
	};
}

describe('the range a reader is looking at', () => {
	it('opens on today, and only today, because the question is tonight', () => {
		expect(defaultRange(today)).toEqual({
			from: '2026-10-17',
			to: '2026-10-17',
		});
	});

	it('reads a day from the URL and falls back to today for one it does not name', () => {
		expect(
			rangeFromParameters(new URLSearchParams('from=2026-10-01'), today)
		).toEqual({
			from: '2026-10-01',
			to: '2026-10-17',
		});
	});

	it('names a day in the reader s own zone, never UTC', () => {
		// 9:30pm on the 17th in a zone behind UTC is already the 18th in
		// UTC; the reader is still looking at her own Saturday.
		expect(toDayValue(today)).toBe('2026-10-17');
	});

	it('writes only the days a reader actually chose into the href', () => {
		const base = '/practices/practice-1/on-call';
		expect(
			rosterHref(base, { from: '2026-10-17', to: '2026-10-17' }, today)
		).toBe(base);
		expect(
			rosterHref(base, { from: '2026-10-17', to: '2026-10-31' }, today)
		).toBe(`${base}?to=2026-10-31`);
	});

	it('asks the BFF for exactly those days', () => {
		expect(
			rosterPath('practice-1', { from: '2026-10-01', to: '2026-10-31' })
		).toBe('/api/practices/practice-1/on-call?from=2026-10-01&to=2026-10-31');
	});
});

describe('the words a reader meets', () => {
	it('says why a birth has no window, in her language', () => {
		expect(describeNoWindow('no_due_date')).toContain('No due date');
		expect(describeNoWindow('nobody_attached')).toContain(
			'Nobody is on this birth'
		);
		expect(describeNoWindow('postpartum')).toContain('Postpartum');
		expect(describeNoWindow('not_active')).toContain('Care has not started');
		expect(describeNoWindow('ended_before_start')).toContain('arrived before');
	});

	it('states plainly that there is no window for a reason it has not met', () => {
		expect(describeNoWindow('something_new')).toBe(
			'There is no on-call window for this birth.'
		);
	});

	it('says a covered window is covered, and by how many where more than one', () => {
		expect(describeCoverage(windowRow())).toBe('Covered');
		expect(
			describeCoverage(
				windowRow({
					onCall: [
						{
							staffId: 'a',
							name: 'Maya',
							from: '2026-10-09',
							to: '2026-10-22',
							narrowed: true,
						},
						{
							staffId: 'b',
							name: 'Bo',
							from: '2026-10-23',
							to: '2026-11-13',
							narrowed: true,
						},
					],
				})
			)
		).toBe('Covered by 2');
	});

	it('names an uncovered gap as the hole it is', () => {
		const gap = {
			id: 'gap-1',
			engagementId: 'eng-1',
			staffId: 'staff-1',
			staffName: 'Maya Primary',
			startsAt: '2026-10-17T22:00:00Z',
			endsAt: '2026-10-18T10:00:00Z',
			reason: undefined,
			coveringStaffId: undefined,
			coveringStaffName: undefined,
		};
		expect(describeCoverage(windowRow({ uncovered: true, gaps: [gap] }))).toBe(
			'A gap needs cover'
		);
		expect(
			describeCoverage(
				windowRow({ uncovered: true, gaps: [gap, { ...gap, id: 'gap-2' }] })
			)
		).toBe('2 gaps need cover');
	});

	it('names a run of days nobody is on call for', () => {
		expect(
			describeCoverage(
				windowRow({
					uncovered: true,
					unstaffedDays: [{ start: '2026-10-23', end: '2026-10-24' }],
				})
			)
		).toBe('Nobody on call for part of this');
	});

	it('summarizes the roster in words, never a bare count', () => {
		const empty = roster([]);
		const oneCovered = roster([windowRow()]);
		const oneOfTwoUncovered = roster([windowRow(), windowRow({ uncovered: true })]);

		expect(rosterSummary(empty)).toBe('No births are on call in these days.');
		expect(rosterSummary(oneCovered)).toBe('1 birth on call, all covered.');
		expect(rosterSummary(oneOfTwoUncovered)).toBe('2 births on call, 1 needing cover.');
	});

	it('picks out every Doula carrying two births at once, and nobody else', () => {
		const doulas = [
			{ staffId: 'a', name: 'Maya', available: true, concurrentWindows: 2 },
			{ staffId: 'b', name: 'Bo', available: true, concurrentWindows: 1 },
			{ staffId: 'c', name: 'Cal', available: false },
		];
		expect(doublyBooked(roster([], doulas)).map((doula) => doula.name)).toEqual(
			['Maya']
		);
	});
});

describe('the calls the screens make', () => {
	it('loads the roster for a range', async () => {
		const body = roster([windowRow()]);
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(body));

		await expect(
			loadRoster(fetcher, 'practice-1', {
				from: '2026-10-17',
				to: '2026-10-17',
			})
		).resolves.toEqual(body);
	});

	it('throws the BFF s own message when the roster is refused', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(jsonResponse('to must be after from', 400));

		await expect(
			loadRoster(fetcher, 'practice-1', {
				from: '2026-10-17',
				to: '2026-10-01',
			})
		).rejects.toThrow('to must be after from');
	});

	it('loads one birth s panel', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(jsonResponse({ noWindowReason: 'no_due_date' }));

		await loadEngagementOnCall(fetcher, 'practice-1', 'eng-1');

		expect(fetcher).toHaveBeenCalledWith(
			engagementOnCallPath('practice-1', 'eng-1')
		);
	});

	it('throws a refusal the panel can read a field off', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(jsonResponse('engagement not found', 404));

		await expect(
			loadEngagementOnCall(fetcher, 'practice-1', 'eng-1')
		).rejects.toThrow('engagement not found');
	});

	it('reads and states the Practice s rule', async () => {
		const settings = {
			startRule: 'gestational_week' as const,
			startWeek: 37,
			graceDays: 14,
		};
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(settings));

		await expect(loadOnCallSettings(fetcher, 'practice-1')).resolves.toEqual(
			settings
		);
		await saveOnCallSettings(fetcher, 'practice-1', settings);

		expect(fetcher).toHaveBeenLastCalledWith(
			'/api/practices/practice-1/on-call-settings',
			{
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(settings),
			}
		);
	});

	it('throws the refusal when a rule is refused, either way round', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(jsonResponse('Enter a week from 20 to 42.', 400));

		await expect(loadOnCallSettings(fetcher, 'practice-1')).rejects.toThrow(
			'Enter a week'
		);
		await expect(
			saveOnCallSettings(fetcher, 'practice-1', {
				startRule: 'gestational_week',
				startWeek: 12,
				graceDays: 14,
			})
		).rejects.toThrow('Enter a week');
	});

	it('posts a new gap and puts an edited one', async () => {
		const draft = {
			staffId: 'staff-1',
			startsAt: '2026-10-17T22:00:00.000Z',
			endsAt: '2026-10-18T10:00:00.000Z',
			reason: 'Wedding',
			coveringStaffId: undefined,
		};
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ id: 'gap-1' }));

		await saveGap(fetcher, 'practice-1', 'eng-1', draft);
		expect(fetcher.mock.calls[0][0]).toBe(
			'/api/practices/practice-1/engagements/eng-1/coverage-gaps'
		);
		expect(fetcher.mock.calls[0][1].method).toBe('POST');

		await saveGap(fetcher, 'practice-1', 'eng-1', draft, 'gap-1');
		expect(fetcher.mock.calls[1][0]).toBe(
			'/api/practices/practice-1/engagements/eng-1/coverage-gaps/gap-1'
		);
		expect(fetcher.mock.calls[1][1].method).toBe('PUT');
	});

	it('throws the refusal the gap form puts on its own controls', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(
				jsonResponse('Choose a doula who is on this birth.', 400)
			);

		await expect(
			saveGap(fetcher, 'practice-1', 'eng-1', {
				staffId: 'nobody',
				startsAt: '2026-10-17T22:00:00.000Z',
				endsAt: '2026-10-18T10:00:00.000Z',
				reason: undefined,
				coveringStaffId: undefined,
			})
		).rejects.toThrow('Choose a doula');
	});

	it('clears a gap, and reports a refusal to clear it', async () => {
		const ok = vi.fn().mockResolvedValue(jsonResponse(undefined, 204));
		await clearGap(ok, 'practice-1', 'eng-1', 'gap-1');
		expect(ok).toHaveBeenCalledWith(
			'/api/practices/practice-1/engagements/eng-1/coverage-gaps/gap-1',
			{ method: 'DELETE' }
		);

		const refused = vi
			.fn()
			.mockResolvedValue(jsonResponse('This coverage gap was not found.', 404));
		await expect(
			clearGap(refused, 'practice-1', 'eng-1', 'gap-1')
		).rejects.toThrow('not found');
	});

	it('states and clears a narrowing, and reports a refusal', async () => {
		const ok = vi
			.fn()
			.mockResolvedValue(jsonResponse({ from: undefined, to: '2026-10-22' }));
		await saveNarrowing(ok, 'practice-1', 'eng-1', 'staff-1', {
			from: undefined,
			to: '2026-10-22',
		});
		expect(ok).toHaveBeenCalledWith(
			'/api/practices/practice-1/engagements/eng-1/attachments/staff-1/on-call',
			{
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ from: undefined, to: '2026-10-22' }),
			}
		);

		const refused = vi
			.fn()
			.mockResolvedValue(jsonResponse('This doula is not on this birth.', 404));
		await expect(
			saveNarrowing(refused, 'practice-1', 'eng-1', 'staff-9', {
				from: undefined,
				to: undefined,
			})
		).rejects.toThrow('not on this birth');
	});
});
