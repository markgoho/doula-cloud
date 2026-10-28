import { describe, expect, it, vi, afterEach } from 'vitest';
import { jsonResponse } from '#lib/testResponse.js';
import { loadedValue, settledRead } from '#lib/testStreamedRead.js';
import type { SchedulePageData } from './+page.js';

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const emptyPage = { items: [], hasMore: false };

const roster = {
	members: [
		{
			staffId: 'staff-1',
			name: 'Persephone Vandermeulen-Achterberg, CD(DONA)',
			email: 'p@example.test',
			roles: ['doula'],
			employmentType: 'employee',
			workState: 'available',
			workStateReportedAt: '2026-09-01T00:00:00Z'
		},
		{
			staffId: 'staff-2',
			name: 'Renata Alvarez',
			email: 'r@example.test',
			roles: ['owner', 'admin'],
			employmentType: 'employee',
			workState: 'available',
			workStateReportedAt: '2026-09-01T00:00:00Z'
		}
	],
	invitations: emptyPage
};

/**
 * One fetch mock answering both reads this `load` makes: the schedule
 * itself and, for an Owner or Admin, the Staff roster the Doula filter is
 * built from.
 */
function setup(status: number, body: unknown) {
	const fetchMock = vi.fn(async (path: string) =>
		path.includes('/staff') ? jsonResponse(roster, 200) : jsonResponse(body, status)
	);
	vi.stubGlobal('fetch', fetchMock);
	return { fetchMock };
}

/** The `load` argument, with the Membership `practices/[practiceId]/
 * +layout.ts` already resolved and whatever narrowing the URL carries. */
function loadEvent(search = '', roles: string[] = ['owner']) {
	return {
		params: { practiceId: 'practice-1' },
		url: new URL(`https://example.test/practices/practice-1/schedule${search}`),
		parent: async () => ({
			session: {
				practiceId: 'practice-1',
				practiceName: 'Riverside Doula Collective',
				roles,
				isContractor: roles.includes('doula') && !roles.includes('owner')
			}
		})
	};
}

afterEach(() => {
	vi.unstubAllGlobals();
	vi.useRealTimers();
});

describe('schedule/+page.ts load', () => {
	it('turns the URL’s calendar days into the half-open instant range the BFF takes', async () => {
		const { load } = await import('./+page.js');
		const { fetchMock } = setup(200, emptyPage);

		const result = await loadedValue<SchedulePageData>(await load(loadEvent('?from=2026-09-12&to=2026-09-19&staffId=staff-1') as Parameters<typeof load>[0]));

		const [requestedPath] = fetchMock.mock.calls[0];
		const url = new URL(requestedPath, 'https://example.test');
		expect(url.pathname).toBe('/api/practices/practice-1/visits');
		// The reader named two days; `to` reaches the START of the 20th, so
		// the whole of the 19th is inside the window.
		expect(new Date(url.searchParams.get('from')!)).toEqual(new Date(2026, 8, 12));
		expect(new Date(url.searchParams.get('to')!)).toEqual(new Date(2026, 8, 20));
		expect(url.searchParams.get('staffId')).toBe('staff-1');
		expect(result.filters).toEqual({ from: '2026-09-12', to: '2026-09-19', staffId: 'staff-1' });
		expect(result.page).toEqual(emptyPage);
	});

	it('falls back to the default window when the URL names no range', async () => {
		// A fixed instant, not the wall clock (#1605): 9am local on October
		// 2, so the window reaches across the night daylight saving ends in
		// a zone that has one. Only `Date` is faked; the load's own promises
		// still settle.
		vi.useFakeTimers({ toFake: ['Date'] });
		vi.setSystemTime(new Date(2026, 9, 2, 9, 0));
		const { load } = await import('./+page.js');
		const { fetchMock } = setup(200, emptyPage);

		const result = await loadedValue<SchedulePageData>(await load(loadEvent() as Parameters<typeof load>[0]));

		const url = new URL(fetchMock.mock.calls[0][0], 'https://example.test');
		expect(result.filters).toEqual({ from: '2026-10-02', to: '2026-11-01', staffId: undefined });
		// Thirty days, then the start of the day after the last one, which
		// the exclusive upper bound needs so November 1 is all inside.
		expect(new Date(url.searchParams.get('from')!)).toEqual(new Date(2026, 9, 2));
		expect(new Date(url.searchParams.get('to')!)).toEqual(new Date(2026, 10, 2));
	});

	it('offers an Owner the Doulas on the roster, and never a Staff member a Visit cannot be assigned to', async () => {
		const { load } = await import('./+page.js');
		setup(200, emptyPage);

		const result = await loadedValue<SchedulePageData>(await load(loadEvent() as Parameters<typeof load>[0]));

		expect(result.doulas).toEqual([
			{ value: 'staff-1', label: 'Persephone Vandermeulen-Achterberg, CD(DONA)' }
		]);
	});

	it('offers a Doula no Doula filter, because the roster read she would need is Owner/Admin only', async () => {
		const { load } = await import('./+page.js');
		const { fetchMock } = setup(200, emptyPage);

		const result = await loadedValue<SchedulePageData>(await load(loadEvent('', ['doula']) as Parameters<typeof load>[0]));

		expect(result.doulas).toEqual([]);
		expect(fetchMock.mock.calls.every(([path]) => !path.includes('/staff'))).toBe(true);
	});

	it('starts the schedule and the roster together', async () => {
		const { load } = await import('./+page.js');
		const { fetchMock } = setup(200, emptyPage);

		await load(loadEvent() as Parameters<typeof load>[0]);

		expect(fetchMock.mock.calls.map(([path]) => new URL(path, 'https://example.test').pathname)).toEqual([
			'/api/practices/practice-1/visits',
			'/api/practices/practice-1/staff'
		]);
	});

	// #2249: a refusal is the screen's to draw, after the screen has
	// changed, so the read carries the response through unread.
	it('hands a refused schedule read to the screen', async () => {
		const { load } = await import('./+page.js');
		setup(403, { code: 'FORBIDDEN', message: 'not permitted to read this' });

		const data = await load(loadEvent() as Parameters<typeof load>[0]);

		await expect(settledRead(data)).resolves.toMatchObject({
			isLoaded: false,
			response: { status: 403 }
		});
	});

	it('hands a refused roster read to the screen', async () => {
		const { load } = await import('./+page.js');
		vi.stubGlobal(
			'fetch',
			vi.fn(async (path: string) =>
				path.includes('/staff') ? jsonResponse('no', 403) : jsonResponse(emptyPage, 200)
			)
		);

		const data = await load(loadEvent() as Parameters<typeof load>[0]);

		await expect(settledRead(data)).resolves.toMatchObject({
			isLoaded: false,
			response: { status: 403 }
		});
	});
});
