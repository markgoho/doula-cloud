import { describe, expect, it } from 'vitest';
import {
	addDays,
	daysIn,
	daysOf,
	daysSpan,
	daysTag,
	daysWords,
	ENTER_FROM_DAY,
	ENTER_TO_DAY,
	isCalendarDay,
	rangeErrors,
	stepDays,
	TO_BEFORE_FROM,
	type DaysView
} from './dayRange.js';

const EASTERN = 'America/New_York';
const TOKYO = 'Asia/Tokyo';
// 10:00am Friday Nov 13 in New York, already 12:00am Saturday Nov 14 in Tokyo.
const NOW = new Date('2026-11-13T15:00:00Z');
const PRESETS = [1, 7, 30] as const;
const UPCOMING = { earliest: { day: '2026-11-13', message: 'Choose today or a later day.' }, longest: { days: 92, message: 'Choose a range of 92 days or fewer.' } };

function viewOf(preset: number, chosen?: DaysView['chosen']): DaysView {
	return { preset, chosen };
}

describe('day arithmetic', () => {
	it('adds days to a day across a month and a year, whatever the zone', () => {
		expect(addDays('2026-11-13', 30)).toBe('2026-12-13');
		expect(addDays('2026-12-31', 1)).toBe('2027-01-01');
		expect(addDays('2026-11-13', -13)).toBe('2026-10-31');
	});

	it('counts a day across daylight saving as a day', () => {
		expect(addDays('2026-11-01', 1)).toBe('2026-11-02');
		expect(daysIn({ from: '2026-10-31', to: '2026-11-02' })).toBe(3);
	});

	it('counts both ends of a range', () => {
		expect(daysIn({ from: '2026-11-13', to: '2026-11-13' })).toBe(1);
		expect(daysIn({ from: '2026-11-13', to: '2026-12-13' })).toBe(31);
	});

	it('knows a real calendar day from a string that only looks like one', () => {
		expect(isCalendarDay('2026-11-13')).toBe(true);
		expect(isCalendarDay('2026-02-30')).toBe(false);
		expect(isCalendarDay('11/13/2026')).toBe(false);
		expect(isCalendarDay('')).toBe(false);
	});
});

describe('why two typed days are not a range', () => {
	it('names an empty or impossible day on its own field', () => {
		expect(rangeErrors('', '2026-11-20')).toEqual({ from: ENTER_FROM_DAY });
		expect(rangeErrors('2026-11-13', '2026-02-30')).toEqual({ to: ENTER_TO_DAY });
		expect(rangeErrors('', '')).toEqual({ from: ENTER_FROM_DAY, to: ENTER_TO_DAY });
	});

	it('says a To before the From, and nothing about a pair in order', () => {
		expect(rangeErrors('2026-11-20', '2026-11-13')).toEqual({ to: TO_BEFORE_FROM });
		expect(rangeErrors('2026-11-13', '2026-11-13')).toEqual({});
	});

	it('refuses a From before the earliest day, in the words it is given, on From', () => {
		expect(rangeErrors('2026-11-12', '2026-11-20', UPCOMING)).toEqual({ from: UPCOMING.earliest.message });
		expect(rangeErrors('2026-11-13', '2026-11-20', UPCOMING)).toEqual({});
	});

	it('does not also fault a To when From is already at fault', () => {
		expect(rangeErrors('2026-11-12', '2026-11-01', UPCOMING)).toEqual({ from: UPCOMING.earliest.message });
	});

	it('refuses a range longer than the longest, on To, and takes the longest itself', () => {
		expect(rangeErrors('2026-11-13', '2027-02-12', UPCOMING)).toEqual({});
		expect(rangeErrors('2026-11-13', '2027-02-13', UPCOMING)).toEqual({ to: UPCOMING.longest.message });
	});

	it('limits nothing a screen has not named', () => {
		expect(rangeErrors('2020-01-01', '2030-01-01')).toEqual({});
	});
});

describe('the days a view reads', () => {
	it('counts a preset from the Practice’s today, the last day included', () => {
		expect(daysOf(viewOf(1), EASTERN, NOW)).toEqual({ days: { from: '2026-11-13', to: '2026-11-13' } });
		expect(daysOf(viewOf(7), EASTERN, NOW)).toEqual({ days: { from: '2026-11-13', to: '2026-11-19' } });
		expect(daysOf(viewOf(30), EASTERN, NOW)).toEqual({ days: { from: '2026-11-13', to: '2026-12-12' } });
	});

	it('reads today in the Practice’s zone, not the reader’s', () => {
		expect(daysOf(viewOf(1), TOKYO, NOW).days).toEqual({ from: '2026-11-14', to: '2026-11-14' });
	});

	it('reads the days a reader chose when they are a range', () => {
		const chosen = { from: '2026-11-20', to: '2026-12-05' };

		expect(daysOf(viewOf(30, chosen), EASTERN, NOW)).toEqual({ days: chosen });
	});

	it('carries the errors of chosen days that are not a range, under the limits it is given', () => {
		expect(daysOf(viewOf(30, { from: '', to: '2026-11-20' }), EASTERN, NOW)).toEqual({ errors: { from: ENTER_FROM_DAY } });
		expect(daysOf(viewOf(30, { from: '2026-11-12', to: '2026-11-20' }), EASTERN, NOW, UPCOMING)).toEqual({
			errors: { from: UPCOMING.earliest.message }
		});
	});
});

describe('stepping the days', () => {
	it('moves a preset by its own length and lands on chosen days', () => {
		const step = stepDays(viewOf(7), 1, EASTERN, NOW, PRESETS);

		expect(step?.days).toEqual({ from: '2026-11-20', to: '2026-11-26' });
		expect(step?.view.chosen).toEqual(step?.days);
	});

	it('steps back onto a preset counted from today, and says which', () => {
		const step = stepDays(viewOf(7, { from: '2026-11-20', to: '2026-11-26' }), -1, EASTERN, NOW, PRESETS);

		expect(step?.days).toEqual({ from: '2026-11-13', to: '2026-11-19' });
		expect(step?.view).toEqual({ preset: 7, chosen: undefined });
	});

	it('picks the preset the offered list names, so a screen without one lands on chosen days', () => {
		const step = stepDays(viewOf(7, { from: '2026-11-20', to: '2026-11-26' }), -1, EASTERN, NOW, [30]);

		expect(step?.view.chosen).toEqual({ from: '2026-11-13', to: '2026-11-19' });
	});

	it('steps one day at a time from tonight', () => {
		const step = stepDays(viewOf(1), 1, EASTERN, NOW, PRESETS);

		expect(step?.days).toEqual({ from: '2026-11-14', to: '2026-11-14' });
	});

	it('steps a month without moving on a daylight-saving day', () => {
		const step = stepDays(viewOf(3, { from: '2026-10-31', to: '2026-11-02' }), 1, EASTERN, NOW, PRESETS);

		expect(step?.days).toEqual({ from: '2026-11-03', to: '2026-11-05' });
	});

	it('floors an earlier step at the floor, with the same length', () => {
		const step = stepDays(viewOf(7, { from: '2026-11-16', to: '2026-11-22' }), -1, EASTERN, NOW, PRESETS, '2026-11-13');

		expect(step?.days).toEqual({ from: '2026-11-13', to: '2026-11-19' });
		expect(step?.view.preset).toBe(7);
	});

	it('steps an earlier step that stays after the floor by its length', () => {
		const step = stepDays(viewOf(3, { from: '2026-11-20', to: '2026-11-22' }), -1, EASTERN, NOW, PRESETS, '2026-11-13');

		expect(step?.days).toEqual({ from: '2026-11-17', to: '2026-11-19' });
	});

	it('has no earlier step from the floor itself, and a later one all the same', () => {
		expect(stepDays(viewOf(7), -1, EASTERN, NOW, PRESETS, '2026-11-13')).toBeUndefined();
		expect(stepDays(viewOf(7), 1, EASTERN, NOW, PRESETS, '2026-11-13')?.days.from).toBe('2026-11-20');
	});

	it('has no step where the days are not a range', () => {
		expect(stepDays(viewOf(7, { from: '', to: '' }), 1, EASTERN, NOW, PRESETS)).toBeUndefined();
	});
});

describe('the days in words', () => {
	const days = { from: '2026-11-13', to: '2026-12-12' };

	it('says a range and one day, the year only when it is not this one', () => {
		expect(daysWords(days, EASTERN, NOW)).toBe('Fri Nov 13 to Sat Dec 12');
		expect(daysWords({ from: '2026-11-14', to: '2026-11-14' }, EASTERN, NOW)).toBe('Sat Nov 14');
		expect(daysWords({ from: '2026-12-28', to: '2027-01-03' }, EASTERN, NOW)).toBe('Mon Dec 28 to Sun Jan 3, 2027');
	});

	it('tags chosen days', () => {
		expect(daysTag(days, EASTERN, NOW)).toBe('Days: Fri Nov 13 to Sat Dec 12');
	});

	it('says where a sentence puts the days, never breaking inside a day', () => {
		expect(daysSpan(days, EASTERN, NOW)).toBe('from Fri\u{A0}Nov\u{A0}13 to Sat\u{A0}Dec\u{A0}12');
		expect(daysSpan({ from: '2026-11-16', to: '2026-11-16' }, EASTERN, NOW)).toBe('on Mon\u{A0}Nov\u{A0}16');
	});
});
