import { describe, expect, it } from 'vitest';
import { THIS_WEEK } from './dayRange.js';
import {
	isScheduleNarrowed,
	scheduleChoiceOfPreset,
	scheduleDaysOf,
	scheduleDaysWords,
	scheduleEmptyNextStep,
	scheduleHref,
	schedulePresetOfChoice,
	scheduleStep,
	scheduleViewFromParameters,
	type ScheduleView
} from './visitSchedule.js';

const EASTERN = 'America/New_York';
// Friday Nov 13 in New York: the week is Mon Nov 9 to Sun Nov 15.
const NOW = new Date('2026-11-13T15:00:00Z');
const week: ScheduleView = { preset: THIS_WEEK, isMine: false, doula: '', needsDoula: false, type: '', order: '' };

describe('This week on the Schedule (#2888)', () => {
	it('is written and read as ?days=week, beside the other narrowings', () => {
		expect(scheduleHref('/s', week)).toBe('/s?days=week');
		expect(scheduleHref('/s', { ...week, isMine: true })).toBe('/s?days=week&mine=true');
		expect(scheduleViewFromParameters(new URLSearchParams('days=week'), true).preset).toBe(THIS_WEEK);
		expect(scheduleViewFromParameters(new URLSearchParams('days=fortnight'), true).preset).toBe(30);
		expect(isScheduleNarrowed(week)).toBe(true);
	});

	it('maps its radio to the preset and back, and the others as before', () => {
		expect(schedulePresetOfChoice('week')).toBe(THIS_WEEK);
		expect(schedulePresetOfChoice('7')).toBe(7);
		expect(schedulePresetOfChoice('30')).toBe(30);
		expect(schedulePresetOfChoice('choose')).toBe(30);
		expect(scheduleChoiceOfPreset(THIS_WEEK)).toBe('week');
		expect(scheduleChoiceOfPreset(7)).toBe('7');
		expect(scheduleChoiceOfPreset(30)).toBe('30');
	});

	it('reads the Monday through Sunday that holds today in the Practice zone', () => {
		expect(scheduleDaysOf(week, EASTERN, NOW).days).toEqual({ from: '2026-11-09', to: '2026-11-15' });
	});

	it('steps by the week, with the link words in the form the other steps use', () => {
		const earlier = scheduleStep(week, -1, EASTERN, NOW)!;
		const later = scheduleStep(week, 1, EASTERN, NOW)!;

		expect(`Earlier, ${scheduleDaysWords(earlier.days, EASTERN, NOW)}`).toBe('Earlier, Mon Nov 2 to Sun Nov 8');
		expect(`Later, ${scheduleDaysWords(later.days, EASTERN, NOW)}`).toBe('Later, Mon Nov 16 to Sun Nov 22');
		expect(scheduleHref('/s', later.view)).toBe('/s?from=2026-11-16&to=2026-11-22');
	});

	it('steps back to this week as ?days=week', () => {
		const next = scheduleStep(week, 1, EASTERN, NOW)!;

		expect(scheduleHref('/s', scheduleStep(next.view, -1, EASTERN, NOW)!.view)).toBe('/s?days=week');
	});

	it('offers the way back to the next 30 days from an empty result', () => {
		expect(scheduleEmptyNextStep(week)?.label).toBe('Show the next 30 days');
	});
});
