import { afterEach, describe, expect, it, vi } from 'vitest';
import { dayIn, partsIn, readerTimeZone, startOfDayIn, zoneName, areZonesApart } from './zonedTime.js';

const EASTERN = 'America/New_York';

afterEach(() => {
	vi.restoreAllMocks();
});

describe('readerTimeZone', () => {
	it('is the zone the runtime reports', () => {
		expect(readerTimeZone()).toBe(new Intl.DateTimeFormat().resolvedOptions().timeZone);
	});
});

describe('partsIn', () => {
	it('reads the wall clock of an instant in the named zone', () => {
		const instant = new Date('2026-11-14T04:30:00Z');
		expect(partsIn(instant, EASTERN)).toEqual({ weekday: 'Fri', month: 'Nov', day: 13, year: 2026, hour: 23, minute: 30 });
		expect(partsIn(instant, 'Asia/Tokyo')).toEqual({ weekday: 'Sat', month: 'Nov', day: 14, year: 2026, hour: 13, minute: 30 });
	});

	it('reads midnight as hour 0, never 24', () => {
		expect(partsIn(new Date('2026-11-13T05:00:00Z'), EASTERN).hour).toBe(0);
	});
});

describe('dayIn', () => {
	it('names the calendar day of an instant in the zone, zero-padded', () => {
		expect(dayIn(new Date('2026-09-07T03:30:00Z'), EASTERN)).toBe('2026-09-06');
		expect(dayIn(new Date('2026-09-07T03:30:00Z'), 'Asia/Tokyo')).toBe('2026-09-07');
	});
});

describe('startOfDayIn', () => {
	it.each([
		['a plain day in New York', '2026-09-12', EASTERN, '2026-09-12T04:00:00.000Z'],
		['the 25-hour fall-back day starts in EDT', '2026-11-01', EASTERN, '2026-11-01T04:00:00.000Z'],
		['the day after it starts in EST', '2026-11-02', EASTERN, '2026-11-02T05:00:00.000Z'],
		['the 23-hour spring-forward day starts in EST', '2026-03-08', EASTERN, '2026-03-08T05:00:00.000Z'],
		['the day after it starts in EDT', '2026-03-09', EASTERN, '2026-03-09T04:00:00.000Z'],
		['a zone east of UTC with no daylight saving', '2026-09-12', 'Asia/Tokyo', '2026-09-11T15:00:00.000Z'],
		['Berlin the day clocks go forward', '2026-03-29', 'Europe/Berlin', '2026-03-28T23:00:00.000Z'],
		['Berlin the day clocks go back', '2026-10-25', 'Europe/Berlin', '2026-10-24T22:00:00.000Z'],
		['Auckland the day clocks go forward', '2026-09-27', 'Pacific/Auckland', '2026-09-26T12:00:00.000Z']
	])('finds %s', (_name, day, zone, want) => {
		expect(startOfDayIn(day, zone).toISOString()).toBe(want);
	});

	it('is the first instant of its day and the last of the day before', () => {
		const start = startOfDayIn('2026-11-01', EASTERN);
		expect(dayIn(start, EASTERN)).toBe('2026-11-01');
		const before = new Date(start.getTime() - 1);
		expect(dayIn(before, EASTERN)).toBe('2026-10-31');
	});
});

describe('zoneName and areZonesApart', () => {
	const winter = new Date('2026-11-13T15:00:00Z');
	const summer = new Date('2026-07-13T15:00:00Z');

	it('says the zone as a person does', () => {
		expect(zoneName(EASTERN, winter)).toBe('Eastern Time');
		expect(zoneName(EASTERN, summer)).toBe('Eastern Time');
	});

	it('falls back to the IANA name when the runtime has none', () => {
		vi.spyOn(Intl.DateTimeFormat.prototype, 'formatToParts').mockReturnValue([{ type: 'literal', value: ' ' }]);
		expect(zoneName('Asia/Tokyo', winter)).toBe('Asia/Tokyo');
	});

	it('differs by offset or by name, and not between two names for one clock', () => {
		expect(areZonesApart(EASTERN, EASTERN, winter)).toBe(false);
		expect(areZonesApart(EASTERN, 'America/Toronto', winter)).toBe(false);
		expect(areZonesApart(EASTERN, 'America/Denver', winter)).toBe(true);
		expect(areZonesApart(EASTERN, 'Asia/Tokyo', summer)).toBe(true);
	});

	it('differs by name when the offsets agree', () => {
		expect(areZonesApart('Europe/London', 'Africa/Abidjan', winter)).toBe(true);
	});
});
