import { describe, expect, it, vi } from 'vitest';
import { jsonResponse } from './testResponse.js';
import {
	DEFAULT_SCHEDULE_DAYS,
	addDays,
	defaultScheduleRange,
	loadPracticeSchedule,
	practiceSchedulePath,
	scheduleFiltersFromParameters,
	scheduleHref,
	scheduleResultSummary,
	toDateInputValue,
	toInstantRange
} from './visitSchedule.js';

// A day in the reader's own zone, built from its parts -- never
// `new Date('2026-09-12')`, which is UTC midnight and is the exact
// mistake these helpers exist to prevent.
const localDay = (year: number, month: number, date: number) => new Date(year, month - 1, date);

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
});

describe('defaultScheduleRange', () => {
	it('runs from today to the end of the default window', () => {
		const range = defaultScheduleRange(localDay(2026, 9, 7));
		expect(range.from).toBe('2026-09-07');
		expect(range.to).toBe(addDays('2026-09-07', DEFAULT_SCHEDULE_DAYS));
	});
});

describe('toInstantRange', () => {
	it('makes the upper bound the start of the day AFTER the one named, so that day is included', () => {
		const range = toInstantRange('2026-09-12', '2026-09-19');
		expect(new Date(range.from)).toEqual(localDay(2026, 9, 12));
		expect(new Date(range.to)).toEqual(localDay(2026, 9, 20));
	});
});

describe('scheduleFiltersFromParameters', () => {
	it('reads the narrowing a shared link carries', () => {
		const parameters = new URLSearchParams('from=2026-09-12&to=2026-09-19&staffId=staff-1');
		expect(scheduleFiltersFromParameters(parameters, localDay(2026, 9, 7))).toEqual({
			from: '2026-09-12',
			to: '2026-09-19',
			staffId: 'staff-1'
		});
	});

	it('falls back to the default window for a day the URL does not name', () => {
		expect(scheduleFiltersFromParameters(new URLSearchParams(), localDay(2026, 9, 7))).toEqual({
			from: '2026-09-07',
			to: '2026-10-07',
			staffId: undefined
		});
	});
});

describe('scheduleHref', () => {
	const base = '/practices/practice-1/schedule';
	const now = localDay(2026, 9, 7);

	it('leaves the default window out, so an unnarrowed schedule stays a clean path', () => {
		expect(scheduleHref(base, { from: '2026-09-07', to: '2026-10-07' }, now)).toBe(base);
	});

	it('writes only the parts a reader actually chose', () => {
		expect(scheduleHref(base, { from: '2026-09-12', to: '2026-10-07' }, now)).toBe(
			`${base}?from=2026-09-12`
		);
		expect(scheduleHref(base, { to: '2026-09-19' }, now)).toBe(`${base}?to=2026-09-19`);
		expect(scheduleHref(base, { staffId: 'staff-1' }, now)).toBe(`${base}?staffId=staff-1`);
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

describe('scheduleResultSummary', () => {
	it('says an empty result in words, not as a bare zero', () => {
		expect(scheduleResultSummary(0, false)).toBe('No scheduled visits in this range.');
	});

	it('counts one visit in the singular', () => {
		expect(scheduleResultSummary(1, false)).toBe('Showing 1 scheduled visit.');
	});

	it('never claims a total the endpoint did not give it', () => {
		expect(scheduleResultSummary(30, true)).toBe('Showing 30 scheduled visits so far. More remain.');
		expect(scheduleResultSummary(31, false)).toBe('Showing 31 scheduled visits.');
	});
});
