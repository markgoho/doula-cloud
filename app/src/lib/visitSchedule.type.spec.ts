// jscpd:ignore-start -- the sibling visitSchedule specs' import list and view fixtures
import { describe, expect, it, vi } from 'vitest';
import { jsonResponse } from './testResponse.js';
import {
	SCHEDULE_TYPE_OPTIONS,
	isScheduleNarrowed,
	loadPracticeSchedule,
	practiceSchedulePath,
	resolveScheduleRead,
	scheduleEmptyNextStep,
	scheduleHref,
	scheduleResultRuler,
	scheduleResultSummary,
	scheduleSearchNextStep,
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
// jscpd:ignore-end

describe('the Visit type (#2888): the address and the read', () => {
	it('offers every type and the words the column draws', () => {
		expect(SCHEDULE_TYPE_OPTIONS).toEqual([
			{ value: '', label: 'Any type' },
			{ value: 'prenatal', label: 'Prenatal' },
			{ value: 'birth', label: 'Birth' },
			{ value: 'postpartum', label: 'Postpartum' }
		]);
	});

	it('reads one of the three types, and any other value as every type', () => {
		for (const type of ['prenatal', 'birth', 'postpartum']) {
			expect(scheduleViewFromParameters(new URLSearchParams(`type=${type}`), true).type).toBe(type);
		}
		expect(scheduleViewFromParameters(new URLSearchParams('type=labor'), true).type).toBe('');
		expect(scheduleViewFromParameters(new URLSearchParams(''), true).type).toBe('');
	});

	it('writes ?type= beside the other narrowings, leaves it out when empty, and counts it as one', () => {
		expect(scheduleHref('/s', view({ type: 'birth' }))).toBe('/s?type=birth');
		expect(scheduleHref('/s', view({ preset: 7, isMine: true, type: 'postpartum', order: 'latest' }))).toBe(
			'/s?days=7&mine=true&type=postpartum&order=latest'
		);
		expect(scheduleHref('/s', noView)).toBe('/s');
		expect(isScheduleNarrowed(view({ type: 'prenatal' }))).toBe(true);
	});

	it('keeps it when the days go back to the default', () => {
		expect(withDefaultDays(view({ preset: 7, type: 'birth' })).type).toBe('birth');
	});

	it('resolves it into the filters the read is made under', () => {
		expect(resolveScheduleRead(view({ type: 'birth' }), EASTERN, 'me', NOW).filters.type).toBe('birth');
		expect(resolveScheduleRead(noView, EASTERN, 'me', NOW).filters.type).toBeUndefined();
	});

	it('asks the BFF for the type, in the address and in a POST body', async () => {
		const filters = { from: '2026-11-13', to: '2026-12-12', timeZone: EASTERN, type: 'postpartum' as const };
		expect(new URL(practiceSchedulePath('p', filters), 'https://example.test').searchParams.get('type')).toBe('postpartum');
		expect(new URL(practiceSchedulePath('p', { ...filters, type: undefined }), 'https://example.test').searchParams.has('type')).toBe(false);

		const fetcher = vi.fn<(path: string, init?: RequestInit) => Promise<Response>>(async () =>
			jsonResponse({ items: [], hasMore: false }, 200)
		);
		await loadPracticeSchedule(fetcher, 'p', { ...filters, search: 'Camille' });
		expect(JSON.parse(fetcher.mock.calls[0][1]!.body as string).type).toBe('postpartum');
	});
});

describe('the Visit type (#2888): the result line', () => {
	it('puts the type before the noun, in lowercase', () => {
		expect(say(3, false, { type: 'postpartum' })).toBe('Showing 3 postpartum Visits from Fri Nov 13 to Sat Dec 12, soonest first.');
		expect(say(1, false, { type: 'birth' })).toBe('Showing 1 birth Visit from Fri Nov 13 to Sat Dec 12, soonest first.');
		expect(say(30, true, { type: 'prenatal' }, 45)).toBe('Showing the first 30 of 45 prenatal Visits from Fri Nov 13 to Sat Dec 12, soonest first.');
		expect(say(1, true, { type: 'prenatal' })).toBe('Showing the first prenatal Visit from Fri Nov 13 to Sat Dec 12, soonest first.');
	});

	it('composes with whose and the search', () => {
		expect(say(2, false, { type: 'birth', doulaName: 'Priya Raman' })).toBe(
			"Showing Priya Raman's 2 birth Visits from Fri Nov 13 to Sat Dec 12, soonest first."
		);
		expect(say(2, false, { type: 'birth', isMine: true, search: 'Camille', order: 'latest' })).toBe(
			'Showing your 2 birth Visits for "Camille" from Fri Nov 13 to Sat Dec 12, latest first.'
		);
	});

	it('says what was looked for when nothing matched', () => {
		expect(say(0, false, { type: 'postpartum' })).toBe('No postpartum Visits from Fri Nov 13 to Sat Dec 12.');
		expect(say(0, false, { type: 'birth', isMine: true })).toBe('You have no birth Visits from Fri Nov 13 to Sat Dec 12.');
		expect(say(0, false, { type: 'birth', needsDoula: true })).toBe(
			'No birth Visit has a Doula no longer on this work from Fri Nov 13 to Sat Dec 12.'
		);
	});

	it('keeps the type beside the Visits whose Doula is no longer on the work', () => {
		expect(say(2, false, { type: 'birth', needsDoula: true })).toBe(
			'Showing 2 birth Visits whose Doula is no longer on this work, from Fri Nov 13 to Sat Dec 12, soonest first.'
		);
	});

	it('holds the longest typed wording in the ruler', () => {
		const ruler = scheduleResultRuler('Persephone Vandermeulen-Achterberg');
		const widest = say(999, true, { type: 'postpartum', doulaName: 'Persephone Vandermeulen-Achterberg', search: 'Camille Ortiz', order: 'latest' }, 9999);
		expect(ruler.length).toBeGreaterThanOrEqual(widest.length);
		expect(
			say(0, false, { type: 'postpartum', needsDoula: true, doulaName: 'Persephone Vandermeulen-Achterberg', search: 'Camille Ortiz' }).length
		).toBeLessThanOrEqual(ruler.length);
	});
});

describe('the Visit type (#2888): the way on from an empty result', () => {
	it('offers the opening days first, keeping the type', () => {
		expect(scheduleEmptyNextStep(view({ type: 'birth', preset: 7 }))).toEqual({
			label: 'Show the next 30 days',
			view: view({ type: 'birth' })
		});
	});

	it('offers every type of Visit on the opening days', () => {
		expect(scheduleEmptyNextStep(view({ type: 'birth', isMine: true }))).toEqual({
			label: 'Show every type of Visit',
			view: view({ isMine: true })
		});
	});

	it('keeps the type through a search that looks a year back and a year ahead', () => {
		const step = scheduleSearchNextStep(view({ type: 'postpartum' }), 'Camille', EASTERN, NOW);
		expect(step?.view).toMatchObject({ type: 'postpartum', chosen: { from: '2025-11-13', to: '2027-11-12' } });
	});
});
