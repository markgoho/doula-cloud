import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { formatCalendarDay, formatInstant, formatUtcCalendarDay } from './dates.js';

/*
 * #1958's proof, with the zone set behind UTC by the spec itself rather
 * than by whoever runs it: CI runs in UTC, where a calendar day read as
 * UTC midnight never moves, so a spec that trusted the runner's zone would
 * pass there and say nothing. A Payment recorded for 2026-10-25 comes back
 * from the BFF as `paidAt: 2026-10-25T00:00:00Z` (`manual_payment.go`
 * writes the date-only `paidOn` into a timestamptz), and in
 * America/New_York that instant is 8pm on Oct 24.
 *
 * Node reads `process.env.TZ` again on every assignment, and Vitest runs
 * each spec file in its own fork, so the zone set here reaches no other
 * file; it is put back after the block anyway.
 */

const runnerZone = process.env.TZ;

beforeAll(() => {
	process.env.TZ = 'America/New_York';
});

afterAll(() => {
	process.env.TZ = runnerZone;
});

describe('a date in a zone behind UTC (America/New_York)', () => {
	// Without this the assertions below would pass vacuously in UTC.
	it('is in a zone where UTC midnight is still the day before', () => {
		expect(new Date('2026-10-25T00:00:00Z').getDate()).toBe(24);
	});

	it('shows a Payment recorded for 2026-10-25 as Oct 25, 2026', () => {
		expect(formatUtcCalendarDay('2026-10-25T00:00:00Z')).toBe('Oct 25, 2026');
	});

	it('shows the review step date for 2026-10-25 as Oct 25, 2026', () => {
		expect(formatCalendarDay('2026-10-25')).toBe('Oct 25, 2026');
	});

	it('still reads a real instant in the reader zone', () => {
		expect(formatInstant('2026-10-25T00:00:00Z')).toBe('Oct 24, 2026');
	});
});
