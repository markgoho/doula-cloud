import { describe, expect, it } from 'vitest';
import {
	formatActivityTimestamp,
	formatCalendarDay,
	formatCalendarMonth,
	formatInstant,
	formatInstantWithClock,
	formatInstantWithSeconds,
	formatPostpartum,
	formatPortalVisit,
	formatScheduledVisit,
	toDatetimeLocalValue
} from './dates.js';

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
		expect(formatInstant('2026-08-01T12:00:00Z')).toBe('Aug 1, 2026');
	});

	it('reads an instant to the minute in the reader own zone', () => {
		const iso = local(2026, 10, 25, 15, 4);
		expect(formatInstantWithClock(iso)).toBe('Oct 25, 2026, 3:04pm');
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
function local(year: number, month: number, day: number, hour = 0, minute = 0, second = 0): string {
	return new Date(year, month - 1, day, hour, minute, second).toISOString();
}

describe('formatActivityTimestamp (ADR-0022)', () => {
	const now = new Date(2026, 7, 14, 20, 12, 0); // Friday Aug 14, 2026, 8:12pm local

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
		expect(formatActivityTimestamp(iso, now)).toBe(want);
	});
});

// #250: a Visit's own Date column and per-row summary.
describe('formatScheduledVisit', () => {
	it.each([
		[undefined, 'Not yet scheduled'],
		['', 'Not yet scheduled']
	])('%s reads as "Not yet scheduled"', (value, want) => {
		expect(formatScheduledVisit(value)).toBe(want);
	});

	it.each([
		['leads with the weekday and drops the year inside the current year', local(2026, 11, 15, 18, 0), 'Sun Nov 15, 6:00pm'],
		['keeps the year for a Visit in a later year', local(2027, 3, 15, 14, 30), 'Mon Mar 15, 2027, 2:30pm'],
		['keeps the year for a Visit in an earlier year', local(2025, 12, 31, 0, 5), 'Wed Dec 31, 2025, 12:05am']
	])('%s', (_name, iso, want) => {
		const now = new Date(2026, 10, 13, 9, 0);
		expect(formatScheduledVisit(iso, now)).toBe(want);
	});
});

// #478: the two things a Client is shown about a Visit, and the whole of
// what tells "Thursday at 2pm" from "she came on August 18".
describe('formatPortalVisit', () => {
	it('reads a Visit still to come as a weekday and a clock', () => {
		const iso = local(2027, 2, 18, 14, 0);
		const expected = new Date(iso);
		expect(formatPortalVisit(iso, false)).toBe(
			`${expected.toLocaleDateString('en-US', { weekday: 'short' })} ${expected.toLocaleDateString('en-US', { month: 'short' })} ${expected.getDate()}, 2:00pm`
		);
	});

	it('reads a Visit that has happened as the calendar day, with no clock', () => {
		const iso = local(2026, 8, 18, 14, 30);
		const expected = new Date(iso);
		expect(formatPortalVisit(iso, true)).toBe(
			`${expected.toLocaleDateString('en-US', { month: 'short' })} ${expected.getDate()}, ${expected.getFullYear()}`
		);
	});

	// The same instant, both ways: the flag is the server's answer, and
	// nothing about the value itself decides which form is used.
	it('renders one instant differently on each side of the flag', () => {
		const iso = local(2027, 2, 18, 14, 0);
		expect(formatPortalVisit(iso, false)).not.toBe(formatPortalVisit(iso, true));
	});
});

describe('toDatetimeLocalValue', () => {
	it.each([[undefined], ['']])('%s round-trips to an empty control value', (value) => {
		expect(toDatetimeLocalValue(value)).toBe('');
	});

	it('renders local date/time parts an <input type="datetime-local"> accepts back', () => {
		const iso = local(2027, 3, 5, 9, 5);
		expect(toDatetimeLocalValue(iso)).toBe('2027-03-05T09:05');
	});
});

describe('formatInstantWithSeconds (#2666)', () => {
	it.each([
		['afternoon', local(2026, 11, 2, 15, 4, 9), 'Nov 2, 2026, 3:04:09pm'],
		['noon', local(2026, 11, 2, 12, 0, 0), 'Nov 2, 2026, 12:00:00pm'],
		['morning', local(2026, 11, 2, 10, 46, 43), 'Nov 2, 2026, 10:46:43am']
	])('reads %s to the second, in the reader own zone', (_name, iso, want) => {
		expect(formatInstantWithSeconds(iso)).toBe(want);
	});
});

describe('formatPostpartum', () => {
	const now = new Date(2026, 10, 8, 15, 30);

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
		expect(formatPostpartum(birthDay, now)).toBe(want);
	});

	it('counts calendar days across a clock change, not hours', () => {
		expect(formatPostpartum('2026-10-18', new Date(2026, 10, 1, 0, 5))).toBe('2 weeks postpartum');
	});
});
