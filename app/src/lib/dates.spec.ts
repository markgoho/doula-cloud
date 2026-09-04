import { describe, expect, it } from 'vitest';
import { formatActivityTimestamp, formatCalendarDay, formatInstant } from './dates.js';

describe('the instant / calendar-day distinction', () => {
	// The one that matters: a due date is a calendar day, so it must read
	// as the day it was typed no matter which zone the reader sits in.
	it('reads a calendar day as its own parts, not as UTC midnight', () => {
		expect(formatCalendarDay('2027-03-01')).toBe(
			new Date(2027, 2, 1).toLocaleDateString(undefined, {
				year: 'numeric',
				month: 'short',
				day: 'numeric'
			})
		);
	});

	it('reads a timestamp as an instant in the reader own zone', () => {
		expect(formatInstant('2026-08-01T10:00:00Z')).toBe(
			new Date('2026-08-01T10:00:00Z').toLocaleDateString(undefined, {
				year: 'numeric',
				month: 'short',
				day: 'numeric'
			})
		);
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
	const now = new Date(2026, 7, 14, 20, 12, 0); // Friday 14 Aug 2026, 8:12pm local

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
		['7+ days ago is the absolute date', local(2026, 7, 31, 20, 0), '31 Jul 2026, 8:00pm']
	])('%s', (_name, iso, want) => {
		expect(formatActivityTimestamp(iso, now)).toBe(want);
	});
});
