import { describe, expect, it, vi } from 'vitest';
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
	scheduleZoneNote,
	toInstantRange,
	withDefaultDays,
	type ScheduleView
} from './visitSchedule.js';

const EASTERN = 'America/New_York';
const DENVER = 'America/Denver';
const TOKYO = 'Asia/Tokyo';
const NOW = new Date('2026-11-13T15:00:00Z');

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

	it('counts calendar days across both daylight saving changes, in any runner zone', () => {
		expect(addDays('2026-10-02', 30)).toBe('2026-11-01');
		expect(addDays('2026-11-01', 1)).toBe('2026-11-02');
		expect(addDays('2026-03-07', 1)).toBe('2026-03-08');
		expect(addDays('2026-03-08', 1)).toBe('2026-03-09');
	});
});

describe('toInstantRange', () => {
	it('makes the upper bound the start of the day AFTER the one named, so that day is included', () => {
		expect(toInstantRange('2026-09-12', '2026-09-19', EASTERN)).toEqual({
			from: '2026-09-12T04:00:00.000Z',
			to: '2026-09-20T04:00:00.000Z'
		});
	});

	it('includes the whole of the fall-back day in New York, all 25 hours of it', () => {
		expect(toInstantRange('2026-11-01', '2026-11-01', EASTERN)).toEqual({
			from: '2026-11-01T04:00:00.000Z',
			to: '2026-11-02T05:00:00.000Z'
		});
	});

	it('includes the whole of the spring-forward day in New York, all 23 hours of it', () => {
		expect(toInstantRange('2026-03-08', '2026-03-08', EASTERN)).toEqual({
			from: '2026-03-08T05:00:00.000Z',
			to: '2026-03-09T04:00:00.000Z'
		});
	});

	it('reads the same days as other instants in a zone east of UTC', () => {
		expect(toInstantRange('2026-09-12', '2026-09-12', TOKYO)).toEqual({
			from: '2026-09-11T15:00:00.000Z',
			to: '2026-09-12T15:00:00.000Z'
		});
	});

	it('holds both daylight saving days in a zone east of UTC (Berlin)', () => {
		expect(toInstantRange('2026-03-29', '2026-03-29', 'Europe/Berlin')).toEqual({
			from: '2026-03-28T23:00:00.000Z',
			to: '2026-03-29T22:00:00.000Z'
		});
		expect(toInstantRange('2026-10-25', '2026-10-25', 'Europe/Berlin')).toEqual({
			from: '2026-10-24T22:00:00.000Z',
			to: '2026-10-25T23:00:00.000Z'
		});
	});

	it('finds the start of a day that begins in the previous UTC day for a zone far east (Auckland)', () => {
		expect(toInstantRange('2026-09-27', '2026-09-27', 'Pacific/Auckland')).toEqual({
			from: '2026-09-26T12:00:00.000Z',
			to: '2026-09-27T11:00:00.000Z'
		});
	});
});

describe('practiceSchedulePath', () => {
	it('sends instants and carries the cursor when there is one', () => {
		const path = practiceSchedulePath(
			'practice-1',
			{ from: '2026-09-12', to: '2026-09-19', timeZone: EASTERN, staffId: 'staff-1' },
			'cursor-token'
		);
		const url = new URL(path, 'https://example.test');
		expect(url.pathname).toBe('/api/practices/practice-1/visits');
		expect(url.searchParams.get('staffId')).toBe('staff-1');
		expect(url.searchParams.get('cursor')).toBe('cursor-token');
	});

	it('omits the Doula and the cursor when neither was chosen', () => {
		const url = new URL(
			practiceSchedulePath('practice-1', { from: '2026-09-12', to: '2026-09-19', timeZone: EASTERN }),
			'https://example.test'
		);
		expect(url.searchParams.has('staffId')).toBe(false);
		expect(url.searchParams.has('cursor')).toBe(false);
	});
});

describe('loadPracticeSchedule', () => {
	const filters = { from: '2026-09-12', to: '2026-09-19', timeZone: EASTERN };

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
	const now = NOW;

	it('counts a preset of N days from today through today plus N - 1, the last day included', () => {
		expect(scheduleDaysOf(noView, EASTERN, now)).toEqual({ days: { from: '2026-11-13', to: '2026-12-12' } });
		expect(scheduleDaysOf(view({ preset: 7 }), EASTERN, now)).toEqual({ days: { from: '2026-11-13', to: '2026-11-19' } });
	});

	it('reads chosen days that are a range', () => {
		const chosen = { from: '2026-11-10', to: '2026-11-19' };
		expect(scheduleDaysOf(view({ chosen }), EASTERN, now)).toEqual({ days: chosen });
	});

	it('carries the errors of chosen days that are not a range, and no days', () => {
		expect(scheduleDaysOf(view({ chosen: { from: '2026-11-19', to: '2026-11-10' } }), EASTERN, now)).toEqual({
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
		const now = NOW;
		expect(scheduleDaysTag({ from: '2026-11-10', to: '2026-11-19' }, EASTERN, now)).toBe('Days: Tue Nov 10 to Thu Nov 19');
		expect(scheduleDaysTag({ from: '2026-12-29', to: '2027-01-02' }, EASTERN, now)).toBe(
			'Days: Tue Dec 29 to Sat Jan 2, 2027'
		);
	});
});

describe('scheduleDaysTag, one day', () => {
	it('says a single day once', () => {
		expect(scheduleDaysTag({ from: '2026-11-16', to: '2026-11-16' }, EASTERN, NOW)).toBe('Days: Mon Nov 16');
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
	const now = NOW;
	const days = { from: '2026-11-13', to: '2026-11-19' };
	const say = (count: number, hasMore: boolean, who = {}) => plain(scheduleResultSummary(count, hasMore, days, EASTERN, who, now));

	const sayOf = (who = {}, total = 42) => plain(scheduleResultSummary(30, true, days, EASTERN, who, now, total));

	it('keeps each day whole on one line, so a line breaks between days and never inside one', () => {
		const text = scheduleResultSummary(3, false, days, EASTERN, {}, now);
		expect(text).toContain('Fri Nov 13');
		expect(text).toContain('Thu Nov 19');
		expect(scheduleResultSummary(3, false, { from: '2027-03-01', to: '2027-03-02' }, EASTERN, {}, now)).toContain(
			'Mon Mar 1, 2027'
		);
	});

	it('says a one-day range as "on" that day, in every wording', () => {
		const oneDay = { from: '2026-11-16', to: '2026-11-16' };
		const sayOn = (count: number, hasMore: boolean, who = {}) =>
			plain(scheduleResultSummary(count, hasMore, oneDay, EASTERN, who, now));
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

	it('says how many there are in all while more exist, for each whose (#2867)', () => {
		expect(sayOf()).toBe('Showing the first 30 of 42 Visits from Fri Nov 13 to Thu Nov 19, soonest first.');
		expect(sayOf({ doulaName: 'Priya Raman' })).toBe(
			"Showing the first 30 of Priya Raman's 42 Visits from Fri Nov 13 to Thu Nov 19, soonest first."
		);
		expect(sayOf({ isMine: true })).toBe('Showing the first 30 of your 42 Visits from Fri Nov 13 to Thu Nov 19, soonest first.');
	});

	it('says the plain line once every Visit is shown, no total smaller than the rows, and none when the page had none', () => {
		expect(plain(scheduleResultSummary(42, false, days, EASTERN, { isMine: true }, now, 42))).toBe(
			'Showing your 42 Visits from Fri Nov 13 to Thu Nov 19, soonest first.'
		);
		expect(plain(scheduleResultSummary(31, true, days, EASTERN, {}, now, 30))).toBe(
			'Showing the first 31 of 31 Visits from Fri Nov 13 to Thu Nov 19, soonest first.'
		);
		expect(plain(scheduleResultSummary(30, true, days, EASTERN, {}, now, undefined))).toBe(
			'Showing the first 30 Visits from Fri Nov 13 to Thu Nov 19, soonest first.'
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

// An instant at a wall-clock hour of an Eastern day (EST, UTC-5, from Nov 1), as the BFF would send it.
const at = (year: number, month: number, date: number, hour = 14) =>
	new Date(Date.UTC(year, month - 1, date, hour + 5)).toISOString();

describe('scheduleDayGroup (#2867)', () => {
	const now = NOW;

	it('says Today and Tomorrow with the day, and any other day as a row writes it', () => {
		expect(scheduleDayGroup(at(2026, 11, 13), EASTERN, now)).toEqual({ key: '2026-11-13', label: 'Today, Fri Nov 13' });
		expect(scheduleDayGroup(at(2026, 11, 14), EASTERN, now)).toEqual({ key: '2026-11-14', label: 'Tomorrow, Sat Nov 14' });
		expect(scheduleDayGroup(at(2026, 11, 15), EASTERN, now)).toEqual({ key: '2026-11-15', label: 'Sun Nov 15' });
	});

	it('says the year only when the day is not this one', () => {
		expect(scheduleDayGroup(at(2027, 1, 4), EASTERN, now).label).toBe('Mon Jan 4, 2027');
	});

	it('keys two Visits of one day alike, whatever the hour, and a past day as a plain day', () => {
		expect(scheduleDayGroup(at(2026, 11, 15, 1), EASTERN, now).key).toBe(scheduleDayGroup(at(2026, 11, 15, 23), EASTERN, now).key);
		expect(scheduleDayGroup(at(2026, 11, 12), EASTERN, now).label).toBe('Thu Nov 12');
	});

	it('reads Tomorrow across a month end', () => {
		expect(scheduleDayGroup(at(2026, 12, 1), EASTERN, new Date('2026-11-30T15:00:00Z')).label).toBe('Tomorrow, Tue Dec 1');
	});

	it('puts an 11:30pm Practice-time Visit under its Practice day, though it is the next day in UTC', () => {
		const lateVisit = '2026-11-14T04:30:00Z';
		expect(scheduleDayGroup(lateVisit, EASTERN, now)).toEqual({ key: '2026-11-13', label: 'Today, Fri Nov 13' });
		expect(scheduleDayGroup(lateVisit, 'UTC', now).key).toBe('2026-11-14');
	});

	it('reads Today in the zone it is given, so the same instant is a day later for a Practice in Tokyo than in New York', () => {
		const earlyInRochester = new Date('2026-11-13T06:30:00Z');
		expect(scheduleDayGroup('2026-11-13T15:00:00Z', EASTERN, earlyInRochester).label).toBe('Today, Fri Nov 13');
		expect(scheduleDayGroup('2026-11-13T15:00:00Z', TOKYO, earlyInRochester).label).toBe('Tomorrow, Sat Nov 14');
	});
});

describe('the Practice’s today (#2867)', () => {
	it('opens the next-7-days window on the Practice’s day, not the reader’s', () => {
		const lateEvening = new Date('2026-11-14T03:30:00Z');
		expect(scheduleDaysOf(view({ preset: 7 }), EASTERN, lateEvening).days).toEqual({ from: '2026-11-13', to: '2026-11-19' });
		expect(scheduleDaysOf(view({ preset: 7 }), TOKYO, lateEvening).days).toEqual({ from: '2026-11-14', to: '2026-11-20' });
	});

	it('sends the instants of the Practice’s days for the default window', () => {
		const path = practiceSchedulePath('practice-1', { from: '2026-11-13', to: '2026-12-12', timeZone: EASTERN });
		const parameters = new URL(path, 'https://example.test').searchParams;
		expect(parameters.get('from')).toBe('2026-11-13T05:00:00.000Z');
		expect(parameters.get('to')).toBe('2026-12-13T05:00:00.000Z');
	});
});

describe('scheduleZoneNote (#2867)', () => {
	it('says nothing to a reader in the Practice’s zone', () => {
		expect(scheduleZoneNote(EASTERN, EASTERN, NOW)).toBeUndefined();
	});

	it('says nothing when another name for the same clock is the reader’s', () => {
		expect(scheduleZoneNote(EASTERN, 'America/Toronto', NOW)).toBeUndefined();
	});

	it('names the Practice’s zone to a reader in another, in words', () => {
		const note = "Times are in this Practice's time zone, Eastern Time.";
		expect(scheduleZoneNote(EASTERN, DENVER, NOW)).toBe(note);
		expect(scheduleZoneNote(EASTERN, TOKYO, NOW)).toBe(note);
		expect(scheduleZoneNote(EASTERN, 'UTC', NOW)).toBe(note);
	});
});

describe('scheduleResultRuler', () => {
	it('is the longest wording the line can say, with the longest Doula’s name and days with a year', () => {
		const ruler = scheduleResultRuler('Persephone Vandermeulen-Achterberg');
		expect(ruler).toContain("Showing the first 999 of Persephone Vandermeulen-Achterberg's 9999 Visits from");
		expect(ruler).toContain('2000');
	});
});

