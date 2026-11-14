import { describe, expect, it } from 'vitest';
import {
	formatActivityTimestamp,
	formatCalendarDay,
	formatCalendarMonth,
	formatCalendarWeekdayDay,
	formatInstant,
	formatInstantWithClock,
	formatInstantWithSeconds,
	formatPostpartum,
	formatPortalVisit,
	formatScheduledVisit,
	scheduledVisitParts,
	hasClockTime,
	toDatetimeLocalValue
} from './dates.js';
import { instantFromWall } from './zonedTime.js';

const EASTERN = 'America/New_York';

describe('the instant / calendar-day distinction', () => {
	// The one that matters: a due date is a calendar day, so it must read
	// as the day it was typed no matter which zone the reader sits in.
	it('reads a calendar day as its own parts, not as UTC midnight', () => {
		expect(formatCalendarDay('2027-03-01')).toBe('Mar 1, 2027');
	});

	it('reads a calendar month as its own parts', () => {
		expect(formatCalendarMonth('2027-02')).toBe('Feb 2027');
	});

	// Noon UTC is the same calendar day in every zone from UTC-11 to
	// UTC+11, so this fixture's day holds wherever the suite runs.
	it('reads a timestamp as an instant in the reader own zone, month first', () => {
		expect(formatInstant('2026-08-01T12:00:00Z', 'America/New_York')).toBe('Aug 1, 2026');
	});

	it('reads an instant to the minute in the reader own zone', () => {
		const iso = local(2026, 10, 25, 15, 4);
		expect(formatInstantWithClock(iso, 'America/New_York')).toBe('Oct 25, 2026, 3:04pm');
	});
});

// ADR-0022: "An entry displays a relative time under seven days ... and
// an absolute one beyond it" -- the activity ledger's own clock, 12-hour
// with a lowercase am/pm and no periods. Every fixture below is built from
// LOCAL wall-clock components (never a UTC 'Z' literal): the function
// under test reads in the reader's own zone, the same as formatInstant
// above, so a fixture has to be constructed the same way to stay correct
// on every machine's zone, CI's included -- not just the one it was
// written on.
const two = (n: number) => String(n).padStart(2, '0');

function local(year: number, month: number, day: number, hour = 0, minute = 0, second = 0): string {
	return instantFromWall(`${year}-${two(month)}-${two(day)}T${two(hour)}:${two(minute)}:${two(second)}`, EASTERN).toISOString();
}

describe('formatActivityTimestamp (ADR-0022)', () => {
	const now = new Date(local(2026, 8, 14, 20, 12)); // Friday Aug 14, 2026, 8:12pm Eastern

	it.each([
		['30 seconds ago reads as just now', local(2026, 8, 14, 20, 11, 30), 'just now'],
		['1 minute ago is singular', local(2026, 8, 14, 20, 10, 59), '1 minute ago'],
		['12 minutes ago', local(2026, 8, 14, 20, 0, 0), '12 minutes ago'],
		['1 hour ago is singular', local(2026, 8, 14, 19, 11, 0), '1 hour ago'],
		['2 hours ago', local(2026, 8, 14, 18, 12, 0), '2 hours ago'],
		['24-48h ago reads as Yesterday, with the 12-hour clock', local(2026, 8, 13, 9, 31), 'Yesterday, 9:31am'],
		['2-7 days ago reads as the weekday name', local(2026, 8, 11, 16, 40), 'Tuesday, 4:40pm'],
		['midnight is 12, not 0, on the 12-hour clock', local(2026, 8, 11, 0, 0), 'Tuesday, 12:00am'],
		['noon is 12pm, not 0pm', local(2026, 8, 11, 12, 0), 'Tuesday, 12:00pm'],
		['7+ days ago is the absolute date', local(2026, 7, 31, 20, 0), 'Jul 31, 2026, 8:00pm']
	])('%s', (_name, iso, want) => {
		expect(formatActivityTimestamp(iso, 'America/New_York', now)).toBe(want);
	});
});

describe('hasClockTime', () => {
	const now = new Date('2026-08-14T20:12:00Z');

	it.each([
		['just now', '2026-08-14T20:11:30Z', false],
		['23 hours ago', '2026-08-13T21:12:00Z', false],
		['a day ago, which reads "Yesterday, 4:12pm"', '2026-08-13T20:12:00Z', true],
		['a week ago', '2026-08-07T20:12:00Z', true]
	])('%s', (_name, value, want) => {
		expect(hasClockTime(value, now)).toBe(want);
	});
});

// #250, #2867: a Visit's own Date column and per-row summary, in the Practice's zone, never the runner's.
const NOW = new Date('2026-11-13T14:00:00Z');

describe('formatScheduledVisit', () => {
	it.each([
		[undefined, 'Not yet scheduled'],
		['', 'Not yet scheduled']
	])('%s reads as "Not yet scheduled"', (value, want) => {
		expect(formatScheduledVisit(value, EASTERN)).toBe(want);
	});

	it.each([
		['leads with the weekday and drops the year inside the current year', '2026-11-15T23:00:00Z', 'Sun Nov 15, 6:00pm'],
		['keeps the year for a Visit in a later year', '2027-03-15T18:30:00Z', 'Mon Mar 15, 2027, 2:30pm'],
		['keeps the year for a Visit in an earlier year', '2025-12-31T05:05:00Z', 'Wed Dec 31, 2025, 12:05am']
	])('%s', (_name, iso, want) => {
		expect(formatScheduledVisit(iso, EASTERN, NOW)).toBe(want);
	});

	it('reads one Visit the same in the Practice zone whatever the reader zone is', () => {
		const iso = '2026-11-14T04:30:00Z';
		expect(formatScheduledVisit(iso, EASTERN, NOW)).toBe('Fri Nov 13, 11:30pm');
		expect(formatScheduledVisit(iso, 'America/Denver', NOW)).toBe('Fri Nov 13, 9:30pm');
		expect(formatScheduledVisit(iso, 'Asia/Tokyo', NOW)).toBe('Sat Nov 14, 1:30pm');
		expect(formatScheduledVisit(iso, 'UTC', NOW)).toBe('Sat Nov 14, 4:30am');
	});
});

describe('scheduledVisitParts', () => {
	it('splits the Visit’s words into its day, comma kept, and its clock', () => {
		expect(scheduledVisitParts('2026-11-15T23:00:00Z', EASTERN, NOW)).toEqual({ day: 'Sun Nov 15,', clock: '6:00pm' });
		expect(scheduledVisitParts('2027-03-15T18:30:00Z', EASTERN, NOW)).toEqual({
			day: 'Mon Mar 15, 2027,',
			clock: '2:30pm'
		});
	});

	it('decides which year is this one in the Practice zone', () => {
		const lateNewYearsEve = new Date('2027-01-01T03:00:00Z');
		expect(scheduledVisitParts('2026-12-31T10:00:00Z', EASTERN, lateNewYearsEve).day).toBe('Thu Dec 31,');
		expect(scheduledVisitParts('2026-12-31T10:00:00Z', 'Asia/Tokyo', lateNewYearsEve).day).toBe('Thu Dec 31, 2026,');
	});
});

describe('formatCalendarWeekdayDay', () => {
	it('reads a calendar day as its own parts with its weekday, the year only when it is not this one', () => {
		expect(formatCalendarWeekdayDay('2026-11-13', EASTERN, NOW)).toBe('Fri Nov 13');
		expect(formatCalendarWeekdayDay('2027-01-02', EASTERN, NOW)).toBe('Sat Jan 2, 2027');
	});

	it('says a day the same in any zone, and the year by the Practice zone', () => {
		const newYear = new Date('2026-12-31T20:00:00Z');
		expect(formatCalendarWeekdayDay('2026-11-13', 'Asia/Tokyo', NOW)).toBe('Fri Nov 13');
		expect(formatCalendarWeekdayDay('2027-01-02', EASTERN, newYear)).toBe('Sat Jan 2, 2027');
		expect(formatCalendarWeekdayDay('2027-01-02', 'Asia/Tokyo', newYear)).toBe('Sat Jan 2');
	});
});

// #478: the two things a Client is shown about a Visit, and the whole of
// what tells "Thursday at 2pm" from "she came on August 18".
describe('formatPortalVisit', () => {
	it('reads a Visit still to come as a weekday and a clock', () => {
		const iso = local(2027, 2, 18, 14, 0);
		expect(formatPortalVisit(iso, false, EASTERN)).toBe('Thu Feb 18, 2:00pm');
	});

	it('reads a Visit that has happened as the calendar day, with no clock', () => {
		const iso = local(2026, 8, 18, 14, 30);
		expect(formatPortalVisit(iso, true, EASTERN)).toBe('Aug 18, 2026');
	});

	// The same instant, both ways: the flag is the server's answer, and
	// nothing about the value itself decides which form is used.
	it('renders one instant differently on each side of the flag', () => {
		const iso = local(2027, 2, 18, 14, 0);
		expect(formatPortalVisit(iso, false, 'America/New_York')).not.toBe(formatPortalVisit(iso, true, 'America/New_York'));
	});
});

describe('toDatetimeLocalValue', () => {
	it.each([[undefined], ['']])('%s round-trips to an empty control value', (value) => {
		expect(toDatetimeLocalValue(value, 'America/New_York')).toBe('');
	});

	it('renders local date/time parts an <input type="datetime-local"> accepts back', () => {
		const iso = local(2027, 3, 5, 9, 5);
		expect(toDatetimeLocalValue(iso, 'America/New_York')).toBe('2027-03-05T09:05');
	});
});

describe('formatInstantWithSeconds (#2666)', () => {
	it.each([
		['afternoon', local(2026, 11, 2, 15, 4, 9), 'Nov 2, 2026, 3:04:09pm'],
		['noon', local(2026, 11, 2, 12, 0, 0), 'Nov 2, 2026, 12:00:00pm'],
		['morning', local(2026, 11, 2, 10, 46, 43), 'Nov 2, 2026, 10:46:43am']
	])('reads %s to the second, in the reader own zone', (_name, iso, want) => {
		expect(formatInstantWithSeconds(iso, 'America/New_York')).toBe(want);
	});
});

describe('formatPostpartum', () => {
	const now = new Date(local(2026, 11, 8, 15, 30));

	it.each([
		['2026-11-08', '0 days postpartum'],
		['2026-11-07', '1 day postpartum'],
		['2026-11-03', '5 days postpartum'],
		['2026-10-26', '13 days postpartum'],
		['2026-10-25', '2 weeks postpartum'],
		['2026-10-18', '3 weeks postpartum'],
		['2026-10-12', '3 weeks postpartum'],
		['2026-08-01', '14 weeks postpartum'],
		['2026-11-20', '0 days postpartum']
	])('reads a birth on %s as %s', (birthDay, want) => {
		expect(formatPostpartum(birthDay, 'America/New_York', now)).toBe(want);
	});

	it('counts calendar days across a clock change, not hours', () => {
		const justAfterTheClocksChange = new Date(local(2026, 11, 1, 0, 5));
		expect(formatPostpartum('2026-10-18', EASTERN, justAfterTheClocksChange)).toBe('2 weeks postpartum');
	});
});

describe('every instant formatter reads the zone it is given, never the runner\'s', () => {
	// 02:30Z on Nov 14 is 9:30pm on Nov 13 in New York and 11:30am on Nov 14 in Tokyo.
	const instant = '2026-11-14T02:30:00Z';

	it.each([
		['America/New_York', 'Nov 13, 2026'],
		['Asia/Tokyo', 'Nov 14, 2026']
	])('formatInstant in %s', (zone, want) => {
		expect(formatInstant(instant, zone)).toBe(want);
	});

	it.each([
		['America/New_York', 'Nov 13, 2026, 9:30pm'],
		['Asia/Tokyo', 'Nov 14, 2026, 11:30am']
	])('formatInstantWithClock in %s', (zone, want) => {
		expect(formatInstantWithClock(instant, zone)).toBe(want);
	});

	it.each([
		['America/New_York', 'Nov 13, 2026, 9:30:15pm'],
		['Asia/Tokyo', 'Nov 14, 2026, 11:30:15am']
	])('formatInstantWithSeconds in %s', (zone, want) => {
		expect(formatInstantWithSeconds('2026-11-14T02:30:15Z', zone)).toBe(want);
	});

	it.each([
		['America/New_York', 'Friday, 9:30pm'],
		['Asia/Tokyo', 'Saturday, 11:30am']
	])('formatActivityTimestamp names the weekday in %s', (zone, want) => {
		expect(formatActivityTimestamp(instant, zone, new Date('2026-11-17T12:00:00Z'))).toBe(want);
	});

	it.each([
		['America/New_York', 'Fri Nov 13, 9:30pm'],
		['Asia/Tokyo', 'Sat Nov 14, 11:30am']
	])('formatPortalVisit in %s', (zone, want) => {
		expect(formatPortalVisit(instant, false, zone)).toBe(want);
	});

	it('formatPostpartum counts from the Practice\'s today', () => {
		// 02:30Z on Nov 20 is still Nov 19 in New York and already Nov 20 in Tokyo.
		const early = new Date('2026-11-20T02:30:00Z');
		expect(formatPostpartum('2026-11-19', EASTERN, early)).toBe('0 days postpartum');
		expect(formatPostpartum('2026-11-19', 'Asia/Tokyo', early)).toBe('1 day postpartum');
	});

	it('toDatetimeLocalValue is the Practice\'s wall clock, and instantFromWall reads it back', () => {
		expect(toDatetimeLocalValue(instant, EASTERN)).toBe('2026-11-13T21:30');
		expect(toDatetimeLocalValue(instant, 'Asia/Tokyo')).toBe('2026-11-14T11:30');
		expect(instantFromWall(toDatetimeLocalValue(instant, 'Asia/Tokyo'), 'Asia/Tokyo').toISOString()).toBe(
			'2026-11-14T02:30:00.000Z'
		);
	});
});
