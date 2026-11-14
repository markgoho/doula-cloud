import { describe, expect, it, vi } from 'vitest';
import { jsonResponse } from './testResponse.js';
import {
	isScheduleNarrowed,
	loadPracticeSchedule,
	practiceSchedulePath,
	resolveScheduleRead,
	scheduleEmptyNextStep,
	scheduleHref,
	scheduleResultRuler,
	scheduleResultSummary,
	scheduleViewFromParameters,
	withDefaultDays,
	type ScheduleView
} from './visitSchedule.js';

const EASTERN = 'America/New_York';
const NOW = new Date('2026-11-13T15:00:00Z');
const DAYS = { from: '2026-11-13', to: '2026-12-12' };
const noView: ScheduleView = { preset: 30, isMine: false, doula: '', needsDoula: false, type: '', order: '' };
const view = (change: Partial<ScheduleView>): ScheduleView => ({ ...noView, ...change });
const say = (
	count: number,
	hasMore: boolean,
	who: Parameters<typeof scheduleResultSummary>[4] = {},
	total?: number
) => scheduleResultSummary(count, hasMore, DAYS, EASTERN, who, NOW, total).replaceAll('\u{A0}', ' ');

describe('Doula no longer on this work (#2863, #2888): the address', () => {
	it('reads needsDoula=true for a reader offered the Doula picker, and for no one else', () => {
		const parameters = new URLSearchParams('needsDoula=true');
		expect(scheduleViewFromParameters(parameters, true).needsDoula).toBe(true);
		expect(scheduleViewFromParameters(parameters, true, { staffId: 'staff-1' }).needsDoula).toBe(false);
		expect(scheduleViewFromParameters(new URLSearchParams('needsDoula=yes'), true).needsDoula).toBe(false);
	});

	it('writes needsDoula=true beside the other narrowings and counts it as one', () => {
		expect(scheduleHref('/s', view({ needsDoula: true }))).toBe('/s?needsDoula=true');
		expect(scheduleHref('/s', view({ preset: 7, isMine: true, needsDoula: true, order: 'latest' }))).toBe(
			'/s?days=7&mine=true&needsDoula=true&order=latest'
		);
		expect(isScheduleNarrowed(view({ needsDoula: true }))).toBe(true);
	});

	it('keeps it when the days go back to the default', () => {
		expect(withDefaultDays(view({ preset: 7, needsDoula: true })).needsDoula).toBe(true);
	});

	it('asks the BFF for offEngagement, in the address and in a POST body', async () => {
		const filters = { from: '2026-11-13', to: '2026-12-12', timeZone: EASTERN, needsDoula: true };
		expect(new URL(practiceSchedulePath('p', filters), 'https://example.test').searchParams.get('offEngagement')).toBe('true');
		expect(new URL(practiceSchedulePath('p', { ...filters, needsDoula: false }), 'https://example.test').searchParams.has('offEngagement')).toBe(
			false
		);

		const fetcher = vi.fn<(path: string, init?: RequestInit) => Promise<Response>>(async () =>
			jsonResponse({ items: [], hasMore: false }, 200)
		);
		await loadPracticeSchedule(fetcher, 'p', { ...filters, search: 'Camille' });
		expect(JSON.parse(fetcher.mock.calls[0][1]!.body as string).offEngagement).toBe(true);
	});

	it('resolves it into the filters the read is made under', () => {
		expect(resolveScheduleRead(view({ needsDoula: true }), EASTERN, 'me', NOW).filters.needsDoula).toBe(true);
		expect(resolveScheduleRead(noView, EASTERN, 'me', NOW).filters.needsDoula).toBeUndefined();
		expect(resolveScheduleRead(view({ needsDoula: true, chosen: { from: '', to: '' } }), EASTERN, 'me', NOW).filters.needsDoula).toBe(true);
	});
});

describe('Doula no longer on this work (#2863, #2888): the result line', () => {
	it('says how many wait after the line, when the rows are not narrowed to them', () => {
		expect(say(42, false, { waitingCount: 2 }, 42)).toBe('Showing 42 Visits from Fri Nov 13 to Sat Dec 12, soonest first. 2 have a Doula no longer on this work.');
		expect(say(42, false, { waitingCount: 1 }, 42)).toBe('Showing 42 Visits from Fri Nov 13 to Sat Dec 12, soonest first. 1 has a Doula no longer on this work.');
	});

	it('says nothing of them when none wait, when there are no rows, or when the read is not the first page', () => {
		expect(say(42, false, { waitingCount: 0 }, 42)).not.toContain('Doula');
		expect(say(42, false, {}, 42)).not.toContain('Doula');
		expect(say(0, false, { waitingCount: 3 })).toBe('No Visits from Fri Nov 13 to Sat Dec 12.');
	});

	it('names the narrowed rows in place of the count', () => {
		expect(say(2, false, { needsDoula: true, waitingCount: 2 })).toBe(
			'Showing 2 Visits whose Doula is no longer on this work, from Fri Nov 13 to Sat Dec 12, soonest first.'
		);
		expect(say(1, false, { needsDoula: true })).toBe('Showing 1 Visit whose Doula is no longer on this work, from Fri Nov 13 to Sat Dec 12, soonest first.');
		expect(say(30, true, { needsDoula: true }, 45)).toBe(
			'Showing the first 30 of 45 Visits whose Doula is no longer on this work, from Fri Nov 13 to Sat Dec 12, soonest first.'
		);
	});

	it('composes with whose, the search and the order', () => {
		expect(say(2, false, { needsDoula: true, isMine: true, order: 'latest' })).toBe(
			'Showing your 2 Visits whose Doula is no longer on this work, from Fri Nov 13 to Sat Dec 12, latest first.'
		);
		expect(say(2, false, { needsDoula: true, search: 'Camille', doulaName: 'Priya Raman' })).toBe(
			`Showing Priya Raman's 2 Visits for "Camille" whose Doula is no longer on this work, from Fri Nov 13 to Sat Dec 12, soonest first.`
		);
	});

	it('says that no Visit is waiting when the narrowing finds none', () => {
		expect(say(0, false, { needsDoula: true })).toBe('No Visit has a Doula no longer on this work from Fri Nov 13 to Sat Dec 12.');
		expect(say(0, false, { needsDoula: true, isMine: true })).toBe('None of your Visits has a Doula no longer on this work from Fri Nov 13 to Sat Dec 12.');
		expect(say(0, false, { needsDoula: true, doulaName: 'Priya Raman', search: 'Camille' })).toBe(
			'No Visit for "Camille" for Priya Raman has a Doula no longer on this work from Fri Nov 13 to Sat Dec 12.'
		);
	});

	it('offers the next 30 days to an empty narrowing on other days, and nothing on the default days', () => {
		expect(scheduleEmptyNextStep(view({ needsDoula: true, preset: 7 }))).toEqual({
			label: 'Show the next 30 days',
			view: view({ needsDoula: true })
		});
		expect(scheduleEmptyNextStep(view({ needsDoula: true }))).toBeUndefined();
	});

	it('is held in the ruler, so the line is as tall in every state', () => {
		const ruler = scheduleResultRuler('Persephone Vandermeulen-Achterberg');
		expect(ruler).toContain('9999 have a Doula no longer on this work.');
		const longest = ruler.length;
		expect(say(999, true, { needsDoula: true, doulaName: 'Persephone Vandermeulen-Achterberg', order: 'latest' }, 9999).length).toBeLessThanOrEqual(longest);
		expect(say(0, false, { needsDoula: true, doulaName: 'Persephone Vandermeulen-Achterberg', search: 'Camille Ortiz' }).length).toBeLessThanOrEqual(longest);
	});
});
