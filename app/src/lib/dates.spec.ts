import { describe, expect, it } from 'vitest';
import { formatCalendarDay, formatInstant } from './dates.js';

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
