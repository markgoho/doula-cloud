import { describe, expect, it } from 'vitest';
import { formatGapTimes, gapTimes } from './coverageGap.js';

const EASTERN = 'America/New_York';
// Noon Friday Nov 13 2026 in New York.
const NOW = new Date('2026-11-13T17:00:00Z');

describe('when a coverage gap runs', () => {
	it('says both ends with the day, in the Practice’s zone', () => {
		expect(formatGapTimes('2026-11-14T23:00:00Z', '2026-11-15T14:00:00Z', EASTERN, NOW)).toBe(
			'Sat Nov 14, 6:00pm to Sun Nov 15, 9:00am'
		);
	});

	it('says the clock alone for an end on the day the gap starts', () => {
		expect(formatGapTimes('2026-11-14T23:00:00Z', '2026-11-15T02:00:00Z', EASTERN, NOW)).toBe(
			'Sat Nov 14, 6:00pm to 9:00pm'
		);
	});

	it('reads the day in the Practice’s zone, so a gap that crosses UTC midnight is one day', () => {
		const { end } = gapTimes('2026-11-14T23:00:00Z', '2026-11-15T04:30:00Z', EASTERN, NOW);

		expect(end).toBe('11:30pm');
	});

	it('adds the year to a day only when it is not this one', () => {
		expect(formatGapTimes('2026-12-31T23:00:00Z', '2027-01-01T14:00:00Z', EASTERN, NOW)).toBe(
			'Thu Dec 31, 6:00pm to Fri Jan 1, 2027, 9:00am'
		);
		expect(formatGapTimes('2027-01-02T23:00:00Z', '2027-01-03T02:00:00Z', EASTERN, NOW)).toBe(
			'Sat Jan 2, 2027, 6:00pm to 9:00pm'
		);
	});

	it('gives each end apart, for the element that holds its instant', () => {
		expect(gapTimes('2026-11-14T23:00:00Z', '2026-11-15T14:00:00Z', EASTERN, NOW)).toEqual({
			start: 'Sat Nov 14, 6:00pm',
			end: 'Sun Nov 15, 9:00am'
		});
	});
});
