import { describe, expect, it, vi } from 'vitest';
import { jsonResponse } from './testResponse.js';
import {
	loadPracticeSchedule,
	resolveScheduleRead,
	scheduleResultRuler,
	scheduleResultSummary,
	scheduleSearchNextStep,
	type ScheduleView
} from './visitSchedule.js';

const EASTERN = 'America/New_York';
const NOW = new Date('2026-11-13T15:00:00Z');
const noView: ScheduleView = { preset: 30, isMine: false, doula: '', order: '' };
const view = (change: Partial<ScheduleView>): ScheduleView => ({ ...noView, ...change });
const answer = () => jsonResponse({ items: [], hasMore: false }, 200);

describe('the Client search (#2870)', () => {
	const filters = { from: '2026-09-12', to: '2026-09-19', timeZone: EASTERN, staffId: 'staff-1', order: 'latest' as const };

	it('reads a search in a POST body and never in the address', async () => {
		const fetcher = vi.fn<(path: string, init?: RequestInit) => Promise<Response>>(async () => answer());

		await loadPracticeSchedule(fetcher, 'practice-1', { ...filters, search: 'Camille' }, 'c1');

		const [path, init] = fetcher.mock.calls[0];
		expect(path).toBe('/api/practices/practice-1/visits/list');
		expect(init?.method).toBe('POST');
		expect(JSON.parse(init!.body as string)).toEqual({
			from: '2026-09-12T04:00:00.000Z',
			to: '2026-09-20T04:00:00.000Z',
			staffId: 'staff-1',
			order: 'latest',
			search: 'Camille',
			cursor: 'c1'
		});
	});

	it('sends no Doula and the default order as empty strings', async () => {
		const fetcher = vi.fn<(path: string, init?: RequestInit) => Promise<Response>>(async () => answer());

		await loadPracticeSchedule(fetcher, 'p', { from: '2026-09-12', to: '2026-09-19', timeZone: EASTERN, search: 'Ana' });

		expect(JSON.parse(fetcher.mock.calls[0][1]!.body as string)).toMatchObject({ staffId: '', order: '', cursor: '' });
	});

	it('resolves a view to its filters, carrying the search only when there is one', () => {
		const plain = resolveScheduleRead(noView, EASTERN, 'me', NOW);
		expect(plain).toMatchObject({ isReadable: true, filters: { from: '2026-11-13', to: '2026-12-12' } });
		expect(plain.filters).not.toHaveProperty('search');
		expect(resolveScheduleRead(noView, EASTERN, 'me', NOW, 'Ana').filters.search).toBe('Ana');
	});

	it('is not readable for days that are no range, or Mine beside another Doula', () => {
		expect(resolveScheduleRead(view({ chosen: { from: '2026-11-13', to: '' } }), EASTERN, 'me', NOW)).toMatchObject({
			isReadable: false,
			errors: { to: expect.any(String) }
		});
		expect(resolveScheduleRead(view({ isMine: true, doula: 'other' }), EASTERN, 'me', NOW).isReadable).toBe(false);
	});

	it('says what was looked for after the Visits it narrows', () => {
		const days = { from: '2026-11-13', to: '2026-12-12' };
		const say = (count: number, who = {}, hasMore = false) =>
			scheduleResultSummary(count, hasMore, days, EASTERN, { search: 'Camille', ...who }, NOW, 41).replaceAll('\u{A0}', ' ');
		expect(say(2)).toBe('Showing 2 Visits for "Camille" from Fri Nov 13 to Sat Dec 12, soonest first.');
		expect(say(1)).toBe('Showing 1 Visit for "Camille" from Fri Nov 13 to Sat Dec 12.');
		expect(say(30, {}, true)).toBe('Showing the first 30 of 41 Visits for "Camille" from Fri Nov 13 to Sat Dec 12, soonest first.');
		expect(say(0)).toBe('No Visits for "Camille" from Fri Nov 13 to Sat Dec 12.');
		expect(say(0, { isMine: true })).toBe('You have no Visits for "Camille" from Fri Nov 13 to Sat Dec 12.');
		expect(say(0, { doulaName: 'Bo Ng' })).toBe('No Visits for "Camille" for Bo Ng from Fri Nov 13 to Sat Dec 12.');
	});

	it('looks a year ahead, today through today plus 364 days, keeping Mine and the order', () => {
		const step = scheduleSearchNextStep(view({ isMine: true, order: 'latest' }), 'Camille', EASTERN, NOW);

		expect(step?.label).toBe('Look for "Camille" in the next 12 months');
		expect(step?.view).toMatchObject({ isMine: true, order: 'latest', chosen: { from: '2026-11-13', to: '2027-11-12' } });
	});

	it('offers nothing where the days already reach a year ahead', () => {
		const wide = view({ chosen: { from: '2026-11-13', to: '2027-11-12' } });
		expect(scheduleSearchNextStep(wide, 'Camille', EASTERN, NOW)).toBeUndefined();
	});

	it('holds room in the ruler for a searched line', () => {
		expect(scheduleResultRuler('')).toContain('for "Camille Ortiz"');
	});
});
