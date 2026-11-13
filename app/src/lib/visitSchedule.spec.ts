import { afterAll, beforeAll, describe, expect, it, vi } from 'vitest';
import { jsonResponse } from './testResponse.js';
import {
	addDays,
	isCalendarDay,
	isScheduleNarrowed,
	SHORT_SCHEDULE_DAYS,
	loadPracticeSchedule,
	practiceSchedulePath,
	scheduleDaysOf,
	scheduleDaysTag,
	scheduleDayGroup,
	scheduleEmptyNextStep,
	scheduleHref,
	scheduleRangeErrors,
	scheduleResultRuler,
	scheduleResultSummary,
	scheduleStaffId,
	scheduleViewFromParameters,
	toDateInputValue,
	toInstantRange,
	withDefaultDays,
	type ScheduleView
} from './visitSchedule.js';

// A day in the reader's own zone, built from its parts -- never
// `new Date('2026-09-12')`, which is UTC midnight and is the exact
// mistake these helpers exist to prevent.
const localDay = (year: number, month: number, date: number) => new Date(year, month - 1, date);

/**
 * Pins the enclosing block to America/New_York, a zone with a 25-hour
 * and a 23-hour day each year, and puts the machine's own zone back
 * after it (#1605). CI runs in UTC, where every day is 24 hours long, so
 * a daylight-saving case that read the machine's zone would pass there
 * whether the arithmetic was right or not. Node reads `process.env.TZ`
 * again each time it is assigned, so the change reaches `Date` at once.
 */
function inNewYork(): void {
	const machineZone = process.env.TZ;
	beforeAll(() => {
		process.env.TZ = 'America/New_York';
	});
	afterAll(() => {
		if (machineZone === undefined) delete process.env.TZ;
		else process.env.TZ = machineZone;
	});
}

describe('toDateInputValue', () => {
	it('names the reader’s own day, zero-padded', () => {
		expect(toDateInputValue(localDay(2026, 9, 7))).toBe('2026-09-07');
	});

	it('names the local day rather than the UTC one late in the evening', () => {
		// 11pm local on the 7th is already the 8th in UTC for anybody
		// behind it; the control has to say the 7th regardless.
		const lateEvening = new Date(2026, 8, 7, 23, 30);
		expect(toDateInputValue(lateEvening)).toBe('2026-09-07');
	});
});

describe('addDays', () => {
	it('carries across a month end', () => {
		expect(addDays('2026-09-30', 1)).toBe('2026-10-01');
	});

	it('carries across a year end', () => {
		expect(addDays('2026-12-31', 1)).toBe('2027-01-01');
	});

	it('goes backwards for a negative count', () => {
		expect(addDays('2026-03-01', -1)).toBe('2026-02-28');
	});

	describe('in a zone that observes daylight saving', () => {
		inNewYork();

		it('counts calendar days across the end of daylight saving (a 25-hour day)', () => {
			expect(addDays('2026-10-02', 30)).toBe('2026-11-01');
			expect(addDays('2026-11-01', 1)).toBe('2026-11-02');
		});

		it('counts calendar days across the start of daylight saving (a 23-hour day)', () => {
			expect(addDays('2026-03-07', 1)).toBe('2026-03-08');
			expect(addDays('2026-03-08', 1)).toBe('2026-03-09');
		});
	});
});

describe('toInstantRange', () => {
	it('makes the upper bound the start of the day AFTER the one named, so that day is included', () => {
		const range = toInstantRange('2026-09-12', '2026-09-19');
		expect(new Date(range.from)).toEqual(localDay(2026, 9, 12));
		expect(new Date(range.to)).toEqual(localDay(2026, 9, 20));
	});

	describe('in a zone that observes daylight saving', () => {
		inNewYork();

		it('includes the whole of the fall-back day, all 25 hours of it', () => {
			// Clocks in New York go back at 2am on 2026-11-01: that day starts
			// at 04:00 UTC (still EDT) and the next one at 05:00 UTC (EST).
			expect(toInstantRange('2026-11-01', '2026-11-01')).toEqual({
				from: '2026-11-01T04:00:00.000Z',
				to: '2026-11-02T05:00:00.000Z'
			});
		});

		it('includes the whole of the spring-forward day, all 23 hours of it', () => {
			expect(toInstantRange('2026-03-08', '2026-03-08')).toEqual({
				from: '2026-03-08T05:00:00.000Z',
				to: '2026-03-09T04:00:00.000Z'
			});
		});
	});
});

describe('practiceSchedulePath', () => {
	it('sends instants and carries the cursor when there is one', () => {
		const path = practiceSchedulePath(
			'practice-1',
			{ from: '2026-09-12', to: '2026-09-19', staffId: 'staff-1' },
			'cursor-token'
		);
		const url = new URL(path, 'https://example.test');
		expect(url.pathname).toBe('/api/practices/practice-1/visits');
		expect(url.searchParams.get('staffId')).toBe('staff-1');
		expect(url.searchParams.get('cursor')).toBe('cursor-token');
	});

	it('omits the Doula and the cursor when neither was chosen', () => {
		const url = new URL(
			practiceSchedulePath('practice-1', { from: '2026-09-12', to: '2026-09-19' }),
			'https://example.test'
		);
		expect(url.searchParams.has('staffId')).toBe(false);
		expect(url.searchParams.has('cursor')).toBe(false);
	});
});

describe('loadPracticeSchedule', () => {
	const filters = { from: '2026-09-12', to: '2026-09-19' };

	it('returns the page the endpoint answered', async () => {
		const page = { items: [], hasMore: false };
		const fetcher = vi.fn(async () => jsonResponse(page, 200));

		await expect(loadPracticeSchedule(fetcher, 'practice-1', filters)).resolves.toEqual(page);
	});

	it('throws with the response body text on a refusal', async () => {
		const fetcher = vi.fn(async () => new Response('to must be after from', { status: 400 }));

		await expect(loadPracticeSchedule(fetcher, 'practice-1', filters)).rejects.toThrow(
			'to must be after from'
		);
	});
});

const noView: ScheduleView = { preset: 30, isMine: false, doula: '' };
const view = (change: Partial<ScheduleView>): ScheduleView => ({ ...noView, ...change });

describe('isCalendarDay', () => {
	it('accepts a real day and refuses everything else', () => {
		expect(isCalendarDay('2026-11-13')).toBe(true);
		expect(isCalendarDay('')).toBe(false);
		expect(isCalendarDay('2026-02-30')).toBe(false);
		expect(isCalendarDay('11/13/2026')).toBe(false);
	});
});

describe('scheduleRangeErrors', () => {
	it('is empty for a range, one day included', () => {
		expect(scheduleRangeErrors('2026-11-13', '2026-11-13')).toEqual({});
	});

	it('asks for each empty or malformed day by name', () => {
		expect(scheduleRangeErrors('', '')).toEqual({ from: 'Enter a From day', to: 'Enter a To day' });
		expect(scheduleRangeErrors('2026-11-13', 'soon')).toEqual({ to: 'Enter a To day' });
	});

	it('refuses a To day before the From day, on the To field', () => {
		expect(scheduleRangeErrors('2026-11-13', '2026-11-12')).toEqual({
			to: 'Choose a To day that is the same as the From day or later.'
		});
	});

	it('does not compare against a From day it already refused', () => {
		expect(scheduleRangeErrors('', '2026-11-12')).toEqual({ from: 'Enter a From day' });
	});
});

const read = (query: string, canMine = true) => scheduleViewFromParameters(new URLSearchParams(query), canMine);

const withoutPicker = (query: string, canMine = true) =>
	scheduleViewFromParameters(new URLSearchParams(query), canMine, { staffId: 'staff-1' });

describe('scheduleViewFromParameters', () => {
	it('is the next 30 days for a bare address', () => {
		expect(read('')).toEqual({ preset: 30, chosen: undefined, isMine: false, doula: '' });
	});

	it('reads days=7, Mine and a Doula', () => {
		expect(read('days=7&mine=true&doula=staff-1')).toEqual({
			preset: 7,
			chosen: undefined,
			isMine: true,
			doula: 'staff-1'
		});
	});

	describe('for a reader not offered the Doula picker', () => {
		it('reads her own id as Mine, with no Doula', () => {
			expect(withoutPicker('doula=staff-1')).toEqual({ preset: 30, chosen: undefined, isMine: true, doula: '' });
		});

		it('ignores any other Doula', () => {
			expect(withoutPicker('doula=staff-2')).toEqual({ preset: 30, chosen: undefined, isMine: false, doula: '' });
		});

		it('keeps Mine from the address, and gives it to nobody who is not offered it', () => {
			expect(withoutPicker('mine=true&doula=staff-2').isMine).toBe(true);
			expect(withoutPicker('doula=staff-1', false).isMine).toBe(false);
		});

		it('never reads an empty id as her own', () => {
			expect(scheduleViewFromParameters(new URLSearchParams('doula='), true, { staffId: '' }).isMine).toBe(false);
		});
	});

	it('falls back to 30 days for any other days, and never reads days=choose as days', () => {
		expect(read('days=30').preset).toBe(30);
		expect(read('days=choose').preset).toBe(30);
		expect(read('days=90').preset).toBe(30);
	});

	it('reads chosen days, and ignores days beside them', () => {
		expect(read('days=7&from=2026-11-10&to=2026-11-19').chosen).toEqual({ from: '2026-11-10', to: '2026-11-19' });
	});

	it('keeps a half-named range as typed, so the missing day can be asked for', () => {
		expect(read('from=2026-11-10').chosen).toEqual({ from: '2026-11-10', to: '' });
		expect(read('to=2026-11-10').chosen).toEqual({ from: '', to: '2026-11-10' });
	});

	it('gives Mine only to a reader who is offered it', () => {
		expect(read('mine=true', false).isMine).toBe(false);
	});
});

describe('scheduleDaysOf', () => {
	const now = localDay(2026, 11, 13);

	it('counts a preset of N days from today through today plus N - 1, the last day included', () => {
		expect(scheduleDaysOf(noView, now)).toEqual({ days: { from: '2026-11-13', to: '2026-12-12' } });
		expect(scheduleDaysOf(view({ preset: 7 }), now)).toEqual({ days: { from: '2026-11-13', to: '2026-11-19' } });
	});

	it('reads chosen days that are a range', () => {
		const chosen = { from: '2026-11-10', to: '2026-11-19' };
		expect(scheduleDaysOf(view({ chosen }), now)).toEqual({ days: chosen });
	});

	it('carries the errors of chosen days that are not a range, and no days', () => {
		expect(scheduleDaysOf(view({ chosen: { from: '2026-11-19', to: '2026-11-10' } }), now)).toEqual({
			errors: { to: 'Choose a To day that is the same as the From day or later.' }
		});
	});
});

describe('scheduleHref', () => {
	const base = '/practices/practice-1/schedule';

	it('is a clean path for the default view, and never writes days=30', () => {
		expect(scheduleHref(base, noView)).toBe(base);
	});

	it('writes only what was chosen', () => {
		expect(scheduleHref(base, view({ preset: 7, isMine: true }))).toBe(`${base}?days=7&mine=true`);
		expect(scheduleHref(base, view({ doula: 'staff-1' }))).toBe(`${base}?doula=staff-1`);
	});

	it('writes chosen days instead of a preset', () => {
		expect(scheduleHref(base, view({ preset: 7, chosen: { from: '2026-11-10', to: '2026-11-19' } }))).toBe(
			`${base}?from=2026-11-10&to=2026-11-19`
		);
	});
});

describe('withDefaultDays and isScheduleNarrowed', () => {
	it('returns to the default days and keeps Mine and the Doula', () => {
		const narrowed = view({ preset: 7, chosen: { from: 'a', to: 'b' }, isMine: true, doula: 'staff-1' });
		expect(withDefaultDays(narrowed)).toEqual({ preset: 30, chosen: undefined, isMine: true, doula: 'staff-1' });
	});

	it('is narrowed by anything beyond the default days', () => {
		expect(isScheduleNarrowed(noView)).toBe(false);
		expect(isScheduleNarrowed(view({ preset: 7 }))).toBe(true);
		expect(isScheduleNarrowed(view({ chosen: { from: '', to: '' } }))).toBe(true);
		expect(isScheduleNarrowed(view({ isMine: true }))).toBe(true);
		expect(isScheduleNarrowed(view({ doula: 'staff-1' }))).toBe(true);
	});
});

describe('scheduleStaffId', () => {
	it('asks for everyone, or for the picked Doula', () => {
		expect(scheduleStaffId(noView, 'me')).toEqual({ staffId: undefined, isEmpty: false });
		expect(scheduleStaffId(view({ doula: 'staff-1' }), 'me')).toEqual({ staffId: 'staff-1', isEmpty: false });
	});

	it('asks for the reader’s own Visits for Mine, alone or beside herself', () => {
		expect(scheduleStaffId(view({ isMine: true }), 'me')).toEqual({ staffId: 'me', isEmpty: false });
		expect(scheduleStaffId(view({ isMine: true, doula: 'me' }), 'me')).toEqual({ staffId: 'me', isEmpty: false });
	});

	it('is empty, and asks for nothing, for Mine beside a different Doula', () => {
		expect(scheduleStaffId(view({ isMine: true, doula: 'staff-1' }), 'me')).toEqual({ isEmpty: true });
	});
});

describe('scheduleDaysTag', () => {
	it('says each day with its weekday, and the year only when it is not this one', () => {
		const now = localDay(2026, 11, 13);
		expect(scheduleDaysTag({ from: '2026-11-10', to: '2026-11-19' }, now)).toBe('Days: Tue Nov 10 to Thu Nov 19');
		expect(scheduleDaysTag({ from: '2026-12-29', to: '2027-01-02' }, now)).toBe(
			'Days: Tue Dec 29 to Sat Jan 2, 2027'
		);
	});
});

describe('scheduleDaysTag, one day', () => {
	it('says a single day once', () => {
		expect(scheduleDaysTag({ from: '2026-11-16', to: '2026-11-16' }, localDay(2026, 11, 13))).toBe('Days: Mon Nov 16');
	});
});

describe('scheduleEmptyNextStep', () => {
	const opening = view({});

	it('offers every Doula\'s Visits, on the opening days, when Mine or a Doula is on', () => {
		expect(scheduleEmptyNextStep(view({ isMine: true }))).toEqual({
			label: "Show every Doula's Visits",
			view: view({ isMine: false, doula: '' })
		});
		expect(scheduleEmptyNextStep(view({ doula: 'staff-1', isMine: true }))?.view).toEqual(view({}));
	});

	it('offers the opening days first, keeping Mine and the Doula, when other days are on', () => {
		const chosen = { from: '2026-11-16', to: '2026-11-17' };
		expect(scheduleEmptyNextStep(view({ doula: 'staff-1', chosen, isMine: true }))).toEqual({
			label: 'Show the next 30 days',
			view: view({ doula: 'staff-1', isMine: true })
		});
	});

	it('offers the next 30 days when only other days are on', () => {
		expect(scheduleEmptyNextStep(view({ preset: SHORT_SCHEDULE_DAYS }))?.label).toBe('Show the next 30 days');
		expect(scheduleEmptyNextStep(view({ chosen: { from: '2027-03-01', to: '2027-03-02' } }))?.view).toEqual(opening);
	});

	it('offers nothing on the opening days with nothing narrowed', () => {
		expect(scheduleEmptyNextStep(opening)).toBeUndefined();
	});
});

// Each day is held whole with non-breaking spaces; read as a person reads it.
const plain = (text: string) => text.replaceAll('\u{A0}', ' ');

describe('scheduleResultSummary', () => {
	const now = localDay(2026, 11, 13);
	const days = { from: '2026-11-13', to: '2026-11-19' };
	const say = (count: number, hasMore: boolean, who = {}) => plain(scheduleResultSummary(count, hasMore, days, who, now));

	it('keeps each day whole on one line, so a line breaks between days and never inside one', () => {
		const text = scheduleResultSummary(3, false, days, {}, now);
		expect(text).toContain('Fri Nov 13');
		expect(text).toContain('Thu Nov 19');
		expect(scheduleResultSummary(3, false, { from: '2027-03-01', to: '2027-03-02' }, {}, now)).toContain(
			'Mon Mar 1, 2027'
		);
	});

	it('says a one-day range as "on" that day, in every wording', () => {
		const oneDay = { from: '2026-11-16', to: '2026-11-16' };
		const sayOn = (count: number, hasMore: boolean, who = {}) =>
			plain(scheduleResultSummary(count, hasMore, oneDay, who, now));
		expect(sayOn(4, false)).toBe('Showing 4 Visits on Mon Nov 16, soonest first.');
		expect(sayOn(0, false)).toBe('No Visits on Mon Nov 16.');
		expect(sayOn(0, false, { isMine: true })).toBe('You have no Visits on Mon Nov 16.');
	});

	it('says one Visit with whose first, never "1 of"', () => {
		expect(say(1, false, { isMine: true })).toBe('Showing your 1 Visit from Fri Nov 13 to Thu Nov 19.');
		expect(say(1, false, { doulaName: 'Priya Raman' })).toBe(
			"Showing Priya Raman's 1 Visit from Fri Nov 13 to Thu Nov 19."
		);
	});

	it('says a single Visit with more to come as the first, with whose', () => {
		expect(say(1, true)).toBe('Showing the first Visit from Fri Nov 13 to Thu Nov 19, soonest first.');
		expect(say(1, true, { isMine: true })).toBe('Showing your first Visit from Fri Nov 13 to Thu Nov 19, soonest first.');
		expect(say(1, true, { doulaName: 'Priya Raman' })).toBe(
			"Showing Priya Raman's first Visit from Fri Nov 13 to Thu Nov 19, soonest first."
		);
	});

	it('says the count, the days and the order, and that more exist when they do', () => {
		expect(say(30, true)).toBe('Showing the first 30 Visits from Fri Nov 13 to Thu Nov 19, soonest first.');
		expect(say(12, false)).toBe('Showing 12 Visits from Fri Nov 13 to Thu Nov 19, soonest first.');
	});

	it('says one Visit with no order', () => {
		expect(say(1, false)).toBe('Showing 1 Visit from Fri Nov 13 to Thu Nov 19.');
	});

	it('says whose when narrowed', () => {
		expect(say(4, false, { doulaName: 'Priya Raman' })).toBe(
			"Showing Priya Raman's 4 Visits from Fri Nov 13 to Thu Nov 19, soonest first."
		);
		expect(say(30, true, { doulaName: 'Priya Raman' })).toBe(
			"Showing the first 30 of Priya Raman's Visits from Fri Nov 13 to Thu Nov 19, soonest first."
		);
		expect(say(5, false, { isMine: true })).toBe('Showing your 5 Visits from Fri Nov 13 to Thu Nov 19, soonest first.');
		expect(say(30, true, { isMine: true })).toBe(
			'Showing the first 30 of your Visits from Fri Nov 13 to Thu Nov 19, soonest first.'
		);
	});

	it('says an empty result in words, for each whose', () => {
		expect(say(0, false)).toBe('No Visits from Fri Nov 13 to Thu Nov 19.');
		expect(say(0, false, { doulaName: 'Priya Raman' })).toBe('No Visits for Priya Raman from Fri Nov 13 to Thu Nov 19.');
		expect(say(0, false, { isMine: true })).toBe('You have no Visits from Fri Nov 13 to Thu Nov 19.');
		expect(say(0, false, { isMine: true, doulaName: 'Priya Raman' })).toBe(
			'No Visits for Priya Raman that are also yours from Fri Nov 13 to Thu Nov 19.'
		);
	});
});

const at = (year: number, month: number, date: number, hour = 14) =>
	new Date(year, month - 1, date, hour).toISOString();

describe('scheduleDayGroup (#2867)', () => {
	const now = localDay(2026, 11, 13);

	it('says Today and Tomorrow with the day, and any other day as a row writes it', () => {
		expect(scheduleDayGroup(at(2026, 11, 13), now)).toEqual({ key: '2026-11-13', label: 'Today, Fri Nov 13' });
		expect(scheduleDayGroup(at(2026, 11, 14), now)).toEqual({ key: '2026-11-14', label: 'Tomorrow, Sat Nov 14' });
		expect(scheduleDayGroup(at(2026, 11, 15), now)).toEqual({ key: '2026-11-15', label: 'Sun Nov 15' });
	});

	it('says the year only when the day is not this one', () => {
		expect(scheduleDayGroup(at(2027, 1, 4), now).label).toBe('Mon Jan 4, 2027');
	});

	it('keys two Visits of one day alike, whatever the hour, and a past day as a plain day', () => {
		expect(scheduleDayGroup(at(2026, 11, 15, 1), now).key).toBe(scheduleDayGroup(at(2026, 11, 15, 23), now).key);
		expect(scheduleDayGroup(at(2026, 11, 12), now).label).toBe('Thu Nov 12');
	});

	it('reads Tomorrow across a month end', () => {
		expect(scheduleDayGroup(at(2026, 12, 1), localDay(2026, 11, 30)).label).toBe('Tomorrow, Tue Dec 1');
	});
});

describe('scheduleResultRuler', () => {
	it('is the longest wording the line can say, with the longest Doula’s name and days with a year', () => {
		const ruler = scheduleResultRuler('Persephone Vandermeulen-Achterberg');
		expect(ruler).toContain("Showing the first 999 of Persephone Vandermeulen-Achterberg's Visits from");
		expect(ruler).toContain('2000');
	});
});

