import { describe, expect, it, vi } from 'vitest';
import {
	clearGap,
	COVER_FIRST,
	daysOnCall,
	daysOnCallText,
	describeCoverage,
	describeNoWindow,
	engagementOnCallPath,
	FROM_BEFORE_TODAY,
	isOnCallNarrowed,
	loadEngagementOnCall,
	onCallEmptyNextStep,
	onCallGroup,
	onCallHref,
	onCallResultRuler,
	onCallResultSummary,
	onCallRuleSentence,
	onCallStep,
	onCallViewFromParameters,
	resolveOnCallRead,
	rosterPath,
	rosterRangeErrors,
	saveGap,
	saveNarrowing,
	TOO_MANY_DAYS,
	tonightLine,
	coveringWords,
	onCallDaysOf,
	personWords,
	sharedDays,
	ownStartWords,
	onCallOrderOptions,
	gapSubject,
	atOnceLabel,
	withDefaultDays,
	type OnCallView,
	type Roster,
	type RosterDoula,
	type RosterWindow
} from './onCall.js';
import { jsonResponse } from './testResponse.js';

const EASTERN = 'America/New_York';
const TOKYO = 'Asia/Tokyo';
// 9:30pm Saturday the 17th in New York, already Sunday the 18th in Tokyo.
const now = new Date('2026-10-18T01:30:00Z');
const TODAY = '2026-10-17';
const BASE = '/practices/practice-1/on-call';

const view = (overrides: Partial<OnCallView> = {}): OnCallView => ({
	preset: 30,
	isMine: false,
	doula: '',
	needsCover: false,
	order: '',
	...overrides
});

function windowRow(overrides: Partial<RosterWindow> = {}): RosterWindow {
	return {
		engagementId: 'eng-1',
		clientName: 'Ada Whitfield',
		dueDate: '2026-10-30',
		window: { start: '2026-10-09', end: '2026-11-13' },
		onCall: [{ staffId: 'staff-1', name: 'Maya Primary', from: '2026-10-09', to: '2026-11-13', narrowed: false }],
		gaps: [],
		unstaffedDays: [],
		uncovered: false,
		...overrides
	};
}

function doula(overrides: Partial<RosterDoula> = {}): RosterDoula {
	return { staffId: 'staff-1', name: 'Maya Primary', births: [], unreachable: [], isOnCallTonight: false, ...overrides };
}

function roster(windows: RosterWindow[], doulas: RosterDoula[] = []): Roster {
	return {
		from: TODAY,
		to: '2026-11-15',
		today: TODAY,
		rule: { startRule: 'gestational_week', startWeek: 37, graceDays: 14 },
		windows,
		noWindow: [],
		doulas
	};
}

const days = { from: TODAY, to: '2026-11-15' };
const nobodyTonight = { holders: [], isNobody: true };
const tonightHeld = { holders: [{ staffId: 'staff-1', name: 'Maya Primary' }], isNobody: false };

const read = (query: string, canMine = true, withoutPicker?: { staffId: string | undefined }) =>
	onCallViewFromParameters(new URLSearchParams(query), canMine, withoutPicker);

const say = (r: Roster, who = {}, span = days) =>
	onCallResultSummary(r, span, EASTERN, who, now).replaceAll('\u{A0}', ' ');

describe('reading the narrowing off the address', () => {

	it('opens on the next 30 days when the address names none', () => {
		expect(read('')).toEqual(view());
	});

	it('reads tonight, the next 7 days, and a chosen range', () => {
		expect(read('days=tonight').preset).toBe(1);
		expect(read('days=7').preset).toBe(7);
		expect(read('days=90').preset).toBe(30);
		expect(read('from=2026-10-20&to=2026-10-22').chosen).toEqual({ from: '2026-10-20', to: '2026-10-22' });
		expect(read('from=2026-10-20').chosen).toEqual({ from: '2026-10-20', to: '' });
		expect(read('to=2026-10-22').chosen).toEqual({ from: '', to: '2026-10-22' });
	});

	it('reads Mine only for a reader who is offered it', () => {
		expect(read('mine=true').isMine).toBe(true);
		expect(read('mine=true', false).isMine).toBe(false);
	});

	it('reads a Doula, Needing cover and the order', () => {
		expect(read('doula=staff-2&cover=needed&order=cover')).toEqual(
			view({ doula: 'staff-2', needsCover: true, order: COVER_FIRST })
		);
		expect(read('order=soonest').order).toBe('');
	});

	it('gives a reader with no Doula picker her own id as Mine and ignores any other', () => {
		const own = { staffId: 'staff-1' };
		expect(read('doula=staff-1', true, own)).toEqual(view({ isMine: true }));
		expect(read('doula=staff-2&cover=needed', true, own)).toEqual(view());
	});
});

describe('the address for a view', () => {
	it('writes only what a reader chose', () => {
		expect(onCallHref(BASE, view())).toBe(BASE);
		expect(onCallHref(BASE, view({ preset: 1 }))).toBe(`${BASE}?days=tonight`);
		expect(onCallHref(BASE, view({ preset: 7 }))).toBe(`${BASE}?days=7`);
		expect(onCallHref(BASE, view({ chosen: { from: '2026-10-20', to: '2026-10-22' } }))).toBe(
			`${BASE}?from=2026-10-20&to=2026-10-22`
		);
		expect(onCallHref(BASE, view({ isMine: true, doula: 'staff-2', needsCover: true, order: COVER_FIRST }))).toBe(
			`${BASE}?mine=true&doula=staff-2&cover=needed&order=cover`
		);
	});

	it('goes back to the default days, keeping the rest', () => {
		expect(withDefaultDays(view({ preset: 7, isMine: true }))).toEqual(view({ isMine: true }));
	});

	it('says whether anything beyond the default is named', () => {
		expect(isOnCallNarrowed(view())).toBe(false);
		for (const change of [
			{ preset: 7 },
			{ chosen: { from: '', to: '' } },
			{ isMine: true },
			{ doula: 'a' },
			{ needsCover: true },
			{ order: COVER_FIRST }
		] as Partial<OnCallView>[]) {
			expect(isOnCallNarrowed(view(change))).toBe(true);
		}
	});

	it('asks the BFF for exactly those births', () => {
		expect(rosterPath('practice-1', { from: '2026-10-01', to: '2026-10-31' })).toBe(
			'/api/practices/practice-1/on-call?from=2026-10-01&to=2026-10-31'
		);
		expect(
			rosterPath('practice-1', { from: TODAY, to: TODAY, staffId: 'staff-2', needsCover: true, order: COVER_FIRST })
		).toBe(`/api/practices/practice-1/on-call?from=${TODAY}&to=${TODAY}&staffId=staff-2&cover=needed&order=cover`);
	});
});

describe('the days a view reads', () => {
	it('is today in the Practice’s zone, not the reader’s (ADR-0036)', () => {
		expect(resolveOnCallRead(view(), EASTERN, undefined, now).days).toEqual({ from: TODAY, to: '2026-11-15' });
		expect(resolveOnCallRead(view(), TOKYO, undefined, now).days).toEqual({ from: '2026-10-18', to: '2026-11-16' });
		expect(resolveOnCallRead(view({ preset: 1 }), EASTERN, undefined, now).days).toEqual({ from: TODAY, to: TODAY });
	});

	it('refuses a From before today, and a range over 92 days, in the BFF’s words', () => {
		const early = view({ chosen: { from: '2026-10-16', to: '2026-10-20' } });
		expect(resolveOnCallRead(early, EASTERN, undefined, now).errors).toEqual({ from: FROM_BEFORE_TODAY });
		const long = view({ chosen: { from: TODAY, to: '2027-03-01' } });
		expect(resolveOnCallRead(long, EASTERN, undefined, now)).toEqual({ errors: { to: TOO_MANY_DAYS }, isEmpty: false });
		expect(resolveOnCallRead(view({ chosen: { from: '', to: '' } }), EASTERN, undefined, now).filters).toBeUndefined();
	});

	it('reads Mine as her own id, a Doula as hers, and no Doula as everyone', () => {
		expect(resolveOnCallRead(view({ isMine: true }), EASTERN, 'staff-1', now).filters?.staffId).toBe('staff-1');
		expect(resolveOnCallRead(view({ doula: 'staff-2' }), EASTERN, 'staff-1', now).filters?.staffId).toBe('staff-2');
		expect(resolveOnCallRead(view(), EASTERN, 'staff-1', now).filters?.staffId).toBeUndefined();
	});

	it('carries Needing cover and the order to the BFF', () => {
		const { filters } = resolveOnCallRead(view({ needsCover: true, order: COVER_FIRST }), EASTERN, 'staff-1', now);
		expect(filters).toMatchObject({ needsCover: true, order: COVER_FIRST });
	});

	it('reads Mine beside a different Doula as nothing to show', () => {
		expect(resolveOnCallRead(view({ isMine: true, doula: 'staff-2' }), EASTERN, 'staff-1', now).isEmpty).toBe(true);
		expect(resolveOnCallRead(view({ isMine: true, doula: 'staff-1' }), EASTERN, 'staff-1', now).isEmpty).toBe(false);
	});
});

describe('stepping the days', () => {
	it('steps later by the days’ own length, landing on a preset when it is one', () => {
		const later = onCallStep(view({ preset: 1 }), 1, EASTERN, now);
		expect(later?.days).toEqual({ from: '2026-10-18', to: '2026-10-18' });
		expect(later?.view.chosen).toEqual(later?.days);
	});

	it('stops Earlier at today: from today there is none, and a step that would pass it lands on it', () => {
		expect(onCallStep(view(), -1, EASTERN, now)).toBeUndefined();
		const chosen = view({ chosen: { from: '2026-10-20', to: '2026-10-26' } });
		const earlier = onCallStep(chosen, -1, EASTERN, now);
		expect(earlier?.days).toEqual({ from: TODAY, to: '2026-10-23' });
		expect(earlier?.view.chosen).toBeUndefined();
	});

	it('has no step where the days are not a range', () => {
		expect(onCallStep(view({ chosen: { from: '', to: '' } }), 1, EASTERN, now)).toBeUndefined();
	});
});

describe('the way on from an empty result', () => {
	it('goes back to the opening days first, then to every Doula, then to every birth', () => {
		expect(onCallEmptyNextStep(view({ preset: 7, isMine: true }))).toEqual({
			label: 'Show the next 30 days',
			view: view({ isMine: true })
		});
		expect(onCallEmptyNextStep(view({ chosen: { from: TODAY, to: TODAY } }))?.label).toBe('Show the next 30 days');
		expect(onCallEmptyNextStep(view({ doula: 'staff-2', needsCover: true }))).toEqual({
			label: "Show every Doula's births",
			view: view({ needsCover: true })
		});
		expect(onCallEmptyNextStep(view({ isMine: true }))?.label).toBe("Show every Doula's births");
		expect(onCallEmptyNextStep(view({ needsCover: true }))).toEqual({ label: 'Show every birth', view: view() });
		expect(onCallEmptyNextStep(view({ order: COVER_FIRST }))).toBeUndefined();
	});
});

const refused = (details: Record<string, string>, status = 400) =>
	jsonResponse({ code: 'INVALID_ARGUMENT', message: 'The days could not be shown.', details }, status);

describe('refusals the BFF sends for the days', () => {

	it('maps details.from and details.to onto the fields', async () => {
		await expect(rosterRangeErrors(refused({ from: FROM_BEFORE_TODAY, to: TOO_MANY_DAYS }))).resolves.toEqual({
			from: FROM_BEFORE_TODAY,
			to: TOO_MANY_DAYS
		});
		await expect(rosterRangeErrors(refused({ to: TOO_MANY_DAYS }))).resolves.toEqual({ to: TOO_MANY_DAYS });
	});

	it('says nothing for a refusal that is not about the days', async () => {
		await expect(rosterRangeErrors(refused({ order: 'Choose an order this list offers.' }))).resolves.toBeUndefined();
		await expect(rosterRangeErrors(refused({ from: 'x' }, 500))).resolves.toBeUndefined();
	});
});

describe('the words a reader meets', () => {
	it('says why a birth has no days, in her language', () => {
		expect(describeNoWindow('no_due_date')).toBe('No due date yet, so there are no on-call days.');
		expect(describeNoWindow('no_due_date_no_doula')).toBe('No due date and no Doula yet.');
		expect(describeNoWindow('nobody_attached')).toBe('No Doula is on this birth yet.');
		expect(describeNoWindow('postpartum')).toContain('Postpartum');
		expect(describeNoWindow('completed')).toContain('Care has ended');
	});

	// #2220: the same reason is given after a loss and after an early live
	// birth, so the words are true for both and name no baby.
	it('says the pregnancy ended, not that a baby arrived, when it ended before on call started', () => {
		expect(describeNoWindow('ended_before_start')).toBe('The pregnancy ended before on call would have started.');
	});

	it('names no baby in any reason that can follow a loss, and never says window', () => {
		for (const reason of ['no_due_date', 'nobody_attached', 'completed', 'ended_before_start', 'something_new']) {
			expect(describeNoWindow(reason)).not.toMatch(/\b(baby|arrived|window)\b/i);
		}
	});

	it('states plainly that there are no days for a reason it has not met', () => {
		expect(describeNoWindow('something_new')).toBe('There are no on-call days for this work.');
	});

	it('says a birth is covered, or needs cover', () => {
		expect(describeCoverage(windowRow())).toBe('Covered');
		expect(describeCoverage(windowRow({ uncovered: true }))).toBe('Needs cover');
	});

	it('states the rule in force', () => {
		const rule = { startRule: 'gestational_week', startWeek: 37, graceDays: 14 } as const;
		expect(onCallRuleSentence(rule)).toBe('On call opens at week 37 of pregnancy and runs 14 days past the due date.');
		expect(onCallRuleSentence({ ...rule, startRule: 'attachment_granted' })).toBe(
			'On call opens on the day a Doula takes the birth and runs 14 days past the due date.'
		);
		expect(onCallRuleSentence({ ...rule, graceDays: 0 })).toBe('On call opens at week 37 of pregnancy and ends on the due date.');
		expect(onCallRuleSentence({ ...rule, graceDays: 1 })).toContain('runs 1 day past');
	});

	it('says two-at-once on a badge by the births held', () => {
		expect(atOnceLabel(1)).toBe('On 2 births at once');
		expect(atOnceLabel(2)).toBe('On 3 births at once');
	});
});

describe('a birth’s days', () => {
	const range = { start: '2026-10-09', end: '2026-11-13' };

	it('are "Until" the last day once they began on or before today', () => {
		expect(daysOnCall(range, TODAY)).toEqual({ until: '2026-11-13' });
		expect(daysOnCallText(range, TODAY, EASTERN, now)).toBe('Until Fri Nov 13');
	});

	it('are the two days when they start later, one day alone when they are one', () => {
		expect(daysOnCallText({ start: '2026-10-23', end: '2026-11-13' }, TODAY, EASTERN, now)).toBe('Fri Oct 23 to Fri Nov 13');
		expect(daysOnCall({ start: '2026-10-23', end: '2026-10-23' }, TODAY)).toEqual({ day: '2026-10-23' });
		expect(daysOnCallText({ start: '2026-10-23', end: '2026-10-23' }, TODAY, EASTERN, now)).toBe('Fri Oct 23');
	});

	it('name a year only when it is not this one', () => {
		expect(daysOnCallText({ start: '2026-12-30', end: '2027-01-05' }, TODAY, EASTERN, now)).toBe('Wed Dec 30 to Tue Jan 5, 2027');
	});
});

describe('who holds a birth tonight', () => {
	it('says nothing for a birth not on call tonight, and names the holder of a one-Doula birth', () => {
		expect(tonightLine(windowRow())).toBeUndefined();
		expect(tonightLine(windowRow({ tonight: tonightHeld }))).toBe('Tonight: Maya Primary');
	});

	it('says nobody when nobody does', () => {
		expect(tonightLine(windowRow({ tonight: nobodyTonight }))).toBe('Tonight: nobody');
	});

	it('says "you" where the holder is the reader, and where the reader is the one covered', () => {
		expect(tonightLine(windowRow({ tonight: tonightHeld }), 'staff-1')).toBe('Tonight: you');
		const covering = { staffId: 'staff-2', name: 'Lena Vasquez', coveringFor: 'Maya Primary' };
		const gap = { id: 'gap-1', engagementId: 'eng-1', staffId: 'staff-1', staffName: 'Maya Primary', startsAt: '', endsAt: '', coveringStaffId: 'staff-2' };
		const row = windowRow({ tonight: { holders: [covering], isNobody: false }, gaps: [gap] });

		expect(tonightLine(row, 'staff-1')).toBe('Tonight: Lena Vasquez, covering for you');
		expect(tonightLine(row, 'staff-2')).toBe('Tonight: you, covering for Maya Primary');
	});

	it('names the covering Doula and whom she covers', () => {
		const covering = { staffId: 'staff-2', name: 'Lena Vasquez', coveringFor: 'Marisol Terrazas' };
		const row = windowRow({ tonight: { holders: [covering], isNobody: false } });

		expect(tonightLine(row)).toBe('Tonight: Lena Vasquez, covering for Marisol Terrazas');
	});

	it('names the holder when more than one Doula is on the birth', () => {
		const two = windowRow({
			onCall: [
				{ staffId: 'staff-1', name: 'Maya Primary', from: '2026-10-09', to: '2026-10-20', narrowed: true },
				{ staffId: 'staff-2', name: 'Bo Ng', from: '2026-10-21', to: '2026-11-13', narrowed: true }
			],
			tonight: tonightHeld
		});

		expect(tonightLine(two)).toBe('Tonight: Maya Primary');
	});
});

describe('the reader and a gap’s cover in words', () => {
	it('is "you" for the reader, capitalized at the start of a line, and a name for anyone else', () => {
		expect(personWords('staff-1', 'Maya Primary', 'staff-1')).toBe('you');
		expect(personWords('staff-1', 'Maya Primary', 'staff-1', true)).toBe('You');
		expect(personWords('staff-1', 'Maya Primary', 'staff-2')).toBe('Maya Primary');
		expect(personWords('staff-1', 'Maya Primary', undefined)).toBe('Maya Primary');
	});

	it('says who covers in one set of words, and "covered" when her name did not arrive', () => {
		expect(coveringWords(false, undefined, 'staff-1')).toBe('nobody covering');
		expect(coveringWords(true, { staffId: 'staff-2', name: 'Lena Vasquez' }, 'staff-1')).toBe('Lena Vasquez covering');
		expect(coveringWords(true, { staffId: 'staff-1', name: 'Maya Primary' }, 'staff-1')).toBe('you covering');
		expect(coveringWords(true, undefined, 'staff-1')).toBe('covered');
		expect(coveringWords(true, { staffId: 'staff-2' }, 'staff-1')).toBe('covered');
	});

	it('says the days two births share from today when they began on or before it', () => {
		expect(sharedDays({ start: '2026-10-09', end: '2026-10-22' }, TODAY)).toEqual({ from: 'today', to: '2026-10-22' });
		expect(sharedDays({ start: '2026-12-08', end: '2026-12-18' }, TODAY)).toEqual({ from: '2026-12-08', to: '2026-12-18' });
		expect(sharedDays({ start: TODAY, end: TODAY }, TODAY)).toEqual({ isToday: true });
		expect(sharedDays({ start: '2026-10-20', end: '2026-10-20' }, TODAY)).toEqual({ day: '2026-10-20' });
	});
});

describe('this week on the roster', () => {
	const week = view({ preset: 0 });

	it('starts today, since the roster offers no past day, and ends on the Sunday', () => {
		expect(onCallDaysOf(week, EASTERN, now)).toEqual({ days: { from: '2026-10-17', to: '2026-10-18' } });
		expect(resolveOnCallRead(week, EASTERN, 'staff-1', now).filters).toEqual({ from: '2026-10-17', to: '2026-10-18' });
	});

	it('is written and read as days=week', () => {
		expect(onCallHref(BASE, week)).toBe(`${BASE}?days=week`);
		expect(onCallViewFromParameters(new URLSearchParams('days=week'), false).preset).toBe(0);
		expect(isOnCallNarrowed(week)).toBe(true);
	});

	it('steps later by the whole week and has no earlier step', () => {
		expect(onCallStep(week, 1, EASTERN, now)?.days).toEqual({ from: '2026-10-19', to: '2026-10-25' });
		expect(onCallStep(week, -1, EASTERN, now)).toBeUndefined();
	});

	it('leaves chosen days and other presets as they are', () => {
		expect(onCallDaysOf(view({ chosen: { from: '2026-10-20', to: '2026-10-22' } }), EASTERN, now).days).toEqual({ from: '2026-10-20', to: '2026-10-22' });
		expect(onCallDaysOf(view({ preset: 7 }), EASTERN, now).days).toEqual({ from: '2026-10-17', to: '2026-10-23' });
	});
});

describe('the headings over the births', () => {
	it('puts tonight’s births under the day and the rest under "Starting later"', () => {
		const tonight = windowRow({ tonight: nobodyTonight });

		expect(onCallGroup(tonight, '', TODAY, EASTERN, now)).toEqual({ key: 'tonight', label: 'Tonight, Sat Oct 17' });
		expect(onCallGroup(windowRow(), '', TODAY, EASTERN, now)).toEqual({ key: 'later', label: 'Starting later' });
	});

	it('puts the births that need cover first under "Needing cover first"', () => {
		const needing = windowRow({ uncovered: true });

		expect(onCallGroup(needing, COVER_FIRST, TODAY, EASTERN, now).label).toBe('Needing cover');
		expect(onCallGroup(windowRow(), COVER_FIRST, TODAY, EASTERN, now).label).toBe('Covered');
	});
});

describe('the result line', () => {
	const atOnce = [{ days: { start: TODAY, end: '2026-10-22' }, engagementIds: ['eng-1', 'eng-2'] }];
	const needs = windowRow({ uncovered: true });
	const needsTonight = windowRow({ uncovered: true, tonight: nobodyTonight });

	it('says how many births, that every one is covered, the days and the order', () => {
		const line = say(roster([windowRow({ tonight: tonightHeld })]));

		expect(line).toBe('Showing 1 birth on call from Sat Oct 17 to Sun Nov 15, tonight first. Every one is covered.');
	});

	it('says how many need cover and how many of them tonight', () => {
		const rows = [needsTonight, needs, needsTonight, windowRow()];

		expect(say(roster(rows))).toBe(
			'Showing 4 births on call from Sat Oct 17 to Sun Nov 15, tonight first. 3 need cover, 2 of them tonight.'
		);
		const mixed = roster([needs, windowRow()]);

		expect(say(mixed)).toContain('1 needs cover.');
		expect(say(roster([needsTonight]))).toContain('1 needs cover, tonight.');
		expect(say(roster([needs, needs]))).toContain('2 need cover.');
	});

	it('says soonest due first when the days do not include today, and needing cover first under that order', () => {
		const later = { from: '2026-10-20', to: '2026-10-25' };
		const one = roster([windowRow()]);

		expect(say(one, {}, later)).toContain('from Tue Oct 20 to Sun Oct 25, soonest due first.');
		expect(say(one, { order: COVER_FIRST })).toContain('needing cover first.');
	});

	it('names whose births', () => {
		const two = roster([windowRow(), windowRow()]);
		const one = roster([windowRow()]);

		expect(say(two, { isMine: true })).toContain('Showing your 2 births on call');
		expect(say(one, { doulaName: 'Priya Raman' })).toContain("Showing Priya Raman's 1 birth on call");
	});

	it('says nothing of cover under Needing cover', () => {
		expect(say(roster([needs, needs]), { needsCover: true })).toBe(
			'Showing 2 births that need cover from Sat Oct 17 to Sun Nov 15, tonight first.'
		);
		expect(say(roster([needs]), { needsCover: true })).toContain('1 birth that needs cover');
	});

	it('counts the Doulas on two births at once', () => {
		const doulas = [doula({ atOnce }), doula({ staffId: 'staff-2', atOnce }), doula({ staffId: 'staff-3' })];

		const two = roster([windowRow()], doulas);
		const one = roster([windowRow()], [doula({ atOnce })]);

		expect(say(two)).toContain('2 Doulas on two births at once.');
		expect(say(one)).toContain('1 Doula on two births at once.');
	});

	it('says so for one Doula alone', () => {
		const alone = roster([windowRow()], [doula({ staffId: 'staff-2', atOnce }), doula({ staffId: 'staff-3' })]);

		expect(say(alone, { isMine: true, staffId: 'staff-2' })).toContain('You are on two births at once.');
		expect(say(alone, { doulaName: 'Priya Raman', staffId: 'staff-2' })).toContain('Priya Raman is on two births at once.');
		expect(say(alone, { isMine: true, staffId: 'staff-3' })).not.toContain('two births');
	});

	it('says an empty result once, by what narrowed it', () => {
		const none = roster([]);

		expect(say(none)).toBe('No births on call from Sat Oct 17 to Sun Nov 15.');
		expect(say(none, { isMine: true })).toBe('You are on call for no births from Sat Oct 17 to Sun Nov 15.');
		expect(say(none, { doulaName: 'Priya Raman' })).toBe('No births for Priya Raman from Sat Oct 17 to Sun Nov 15.');
		expect(say(none, { needsCover: true })).toBe('No births need cover from Sat Oct 17 to Sun Nov 15.');
		expect(say(none, {}, { from: TODAY, to: TODAY })).toBe('No births on call on Sat Oct 17.');
	});

	it('keeps a room that fits the longest wording it can say', () => {
		const name = 'Persephone Vandermeulen-Achterberg, CD(DONA)';
		const typical = say(roster([needsTonight], [doula({ atOnce })]), { doulaName: name });

		expect(onCallResultRuler(name).length).toBeGreaterThanOrEqual(typical.length);
	});
});

describe('the calls the screens make', () => {
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

describe('a birth’s own start against the Practice’s rule', () => {
	const week37 = { startRule: 'gestational_week', startWeek: 37, graceDays: 14 } as const;
	const grantDay = { startRule: 'attachment_granted', startWeek: 37, graceDays: 14 } as const;

	it('says the grant-day start against a week rule', () => {
		expect(ownStartWords({ startRule: 'attachment_granted' }, week37)).toBe('On call from the day a Doula took this birth, not from week 37.');
	});

	it('says its own week against a week rule', () => {
		expect(ownStartWords({ startRule: 'gestational_week', startWeek: 38 }, week37)).toBe('On call from week 38 for this birth, not week 37.');
	});

	it('says what the Practice’s rule is when that is the grant day', () => {
		expect(ownStartWords({ startRule: 'gestational_week', startWeek: 38 }, grantDay)).toBe(
			'On call from week 38 for this birth, not from the day a Doula took it.'
		);
		expect(ownStartWords({ startRule: 'attachment_granted' }, grantDay)).toBe(
			'On call from the day a Doula took this birth, not from the day a Doula took it.'
		);
	});
});

describe('the Order control’s first option', () => {
	it('says what the result line says, by whether today is in the days shown', () => {
		expect(onCallOrderOptions(true)[0]!.label).toBe('Tonight first');
		expect(onCallOrderOptions(false)[0]!.label).toBe('Soonest due first');
		expect(onCallOrderOptions(false)[1]!.value).toBe(COVER_FIRST);
	});
});

describe('the subject of a gap line', () => {
	it('is You for the reader’s own gap, and her name for anyone else’s', () => {
		const gap = { staffId: 'staff-1', staffName: 'Priya Raman' };
		expect(gapSubject(gap, 'staff-1')).toBe('You');
		expect(gapSubject(gap, 'staff-2')).toBe('Priya Raman');
		expect(gapSubject(gap, undefined)).toBe('Priya Raman');
	});
});
