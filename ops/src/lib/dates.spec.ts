import { describe, expect, it } from 'vitest';
import { longDate, newYorkDay } from './dates.ts';

describe('longDate', () => {
	it('writes a calendar day the way a person reads it, moved by no zone', () => {
		expect(longDate('2026-10-30')).toBe('October 30, 2026');
	});
});

describe('newYorkDay', () => {
	it('writes the day an instant falls on in New York', () => {
		expect(newYorkDay('2027-01-02T01:00:00Z')).toBe('January 1, 2027');
		expect(newYorkDay('2026-09-14T03:30:00Z')).toBe('September 13, 2026');
	});
});
