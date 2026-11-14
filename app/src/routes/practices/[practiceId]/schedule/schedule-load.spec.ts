import { describe, expect, it, vi, afterEach, beforeEach } from 'vitest';
import { jsonResponse } from '#lib/testResponse.js';
import { loadedValue, settledRead } from '#lib/testStreamedRead.js';
import { load, type SchedulePageData } from './+page.js';

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
function setup(status: number, body: unknown, rosterStatus = 200) {
	const fetchMock = vi.fn(async (path: string) =>
		path.includes('/staff') ? jsonResponse(roster, rosterStatus) : jsonResponse(body, status)
	);
	vi.stubGlobal('fetch', fetchMock);
	return { fetchMock };
}

const depends = vi.fn<(...keys: string[]) => void>();

/** The `load` argument, with the Membership `practices/[practiceId]/
 * +layout.ts` already resolved and whatever narrowing the URL carries. */
function loadEvent(search = '', roles: string[] = ['owner'], hasOtherDoulas = true, timezone = 'America/New_York') {
	const event = {
		params: { practiceId: 'practice-1' },
		url: new URL(`https://example.test/practices/practice-1/schedule${search}`),
		depends: (key: string) => depends(key),
		parent: async () => ({
			session: {
				practiceId: 'practice-1',
				staffId: 'me',
				practiceName: 'Riverside Doula Collective',
				roles,
				isContractor: false,
				hasOtherDoulas,
				timezone
			}
		})
	};
	return event as Parameters<typeof load>[0];
}

const scheduleCalls = (fetchMock: ReturnType<typeof setup>['fetchMock']) =>
	fetchMock.mock.calls.filter(([path]) => !path.includes('/staff'));

beforeEach(() => {
	vi.useFakeTimers({ toFake: ['Date'], now: new Date('2026-10-02T13:00:00Z') });
});

afterEach(() => {
	depends.mockReset();
	vi.useRealTimers();
	vi.unstubAllGlobals();
});

describe('schedule/+page.ts load', () => {
	it('turns the URL’s calendar days into the half-open instant range the BFF takes', async () => {
		const { fetchMock } = setup(200, emptyPage);

		const result = await loadedValue<SchedulePageData>(
			await load(loadEvent('?from=2026-09-12&to=2026-09-19&doula=staff-1'))
		);

		const [requestedPath] = fetchMock.mock.calls[0];
		const url = new URL(requestedPath, 'https://example.test');
		expect(url.pathname).toBe('/api/practices/practice-1/visits');
		// The reader named two days; `to` reaches the START of the 20th, so
		// the whole of the 19th is inside the window.
		expect(url.searchParams.get('from')).toBe('2026-09-12T04:00:00.000Z');
		expect(url.searchParams.get('to')).toBe('2026-09-20T04:00:00.000Z');
		expect(url.searchParams.get('staffId')).toBe('staff-1');
		expect(result.filters).toEqual({ from: '2026-09-12', to: '2026-09-19', timeZone: 'America/New_York', staffId: 'staff-1', order: '' });
		expect(result.now).toBeInstanceOf(Date);
		expect(result.page).toEqual(emptyPage);
	});

	it('asks the BFF for the order the address names (#2869)', async () => {
		const { fetchMock } = setup(200, emptyPage);

		const result = await loadedValue<SchedulePageData>(await load(loadEvent('?order=latest')));

		expect(new URL(fetchMock.mock.calls[0][0], 'https://example.test').searchParams.get('order')).toBe('latest');
		expect(result.filters.order).toBe('latest');
	});

	it('opens on today and the 29 days after it, the 30th day included', async () => {
		// beforeEach's fixed instant, not the wall clock (#1605, #2368).
		const { fetchMock } = setup(200, emptyPage);

		const result = await loadedValue<SchedulePageData>(await load(loadEvent()));

		const url = new URL(fetchMock.mock.calls[0][0], 'https://example.test');
		expect(result.filters).toEqual({ from: '2026-10-02', to: '2026-10-31', timeZone: 'America/New_York', staffId: undefined, order: '' });
		expect(url.searchParams.get('from')).toBe('2026-10-02T04:00:00.000Z');
		expect(url.searchParams.get('to')).toBe('2026-11-01T04:00:00.000Z');
	});

	it('opens on the Practice’s today, not the runner’s, and sends the instants of the Practice’s days (#2867)', async () => {
		vi.setSystemTime(new Date('2026-10-02T20:00:00Z'));
		const { fetchMock } = setup(200, emptyPage);

		const eastern = await loadedValue<SchedulePageData>(await load(loadEvent('?days=7')));
		const tokyo = await loadedValue<SchedulePageData>(await load(loadEvent('?days=7', ['owner'], true, 'Asia/Tokyo')));

		expect(eastern.filters).toMatchObject({ from: '2026-10-02', to: '2026-10-08' });
		expect(tokyo.filters).toMatchObject({ from: '2026-10-03', to: '2026-10-09', timeZone: 'Asia/Tokyo' });
		const sent = (call: number) => new URL(fetchMock.mock.calls[call][0], 'https://example.test').searchParams;
		expect(sent(0).get('from')).toBe('2026-10-02T04:00:00.000Z');
		expect(sent(2).get('from')).toBe('2026-10-02T15:00:00.000Z');
		expect(sent(2).get('to')).toBe('2026-10-09T15:00:00.000Z');
	});

	it('reads the next seven days from days=7', async () => {
		setup(200, emptyPage);

		const result = await loadedValue<SchedulePageData>(await load(loadEvent('?days=7')));

		expect(result.filters).toMatchObject({ from: '2026-10-02', to: '2026-10-08' });
		expect(result.view.preset).toBe(7);
	});

	it('asks the BFF for the reader’s own Visits for Mine', async () => {
		const { fetchMock } = setup(200, emptyPage);

		await loadedValue<SchedulePageData>(await load(loadEvent('?mine=true', ['doula'])));

		const url = new URL(fetchMock.mock.calls[0][0], 'https://example.test');
		expect(url.searchParams.get('staffId')).toBe('me');
	});

	it('gives Mine no effect to a reader who is not offered it', async () => {
		const { fetchMock } = setup(200, emptyPage);

		await loadedValue<SchedulePageData>(await load(loadEvent('?mine=true', ['doula'], false)));

		const url = new URL(fetchMock.mock.calls[0][0], 'https://example.test');
		expect(url.searchParams.has('staffId')).toBe(false);
	});

	it('answers Mine beside a different Doula with an empty list and no schedule read', async () => {
		const { fetchMock } = setup(200, emptyPage);

		const result = await loadedValue<SchedulePageData>(await load(loadEvent('?mine=true&doula=staff-1', ['owner', 'doula'])));

		expect(result.page).toEqual(emptyPage);
		expect(scheduleCalls(fetchMock)).toHaveLength(0);
	});

	it('answers a To day before the From day in place, and never sends the range', async () => {
		const { fetchMock } = setup(200, emptyPage);

		const result = await loadedValue<SchedulePageData>(await load(loadEvent('?from=2026-09-19&to=2026-09-12')));

		expect(result.rangeErrors).toEqual({ to: 'Choose a To day that is the same as the From day or later.' });
		expect(result.page).toEqual(emptyPage);
		expect(result.filters).toEqual({ from: '', to: '', timeZone: 'America/New_York' });
		expect(scheduleCalls(fetchMock)).toHaveLength(0);
	});

	it('answers an empty day in place, never an error page', async () => {
		const { fetchMock } = setup(200, emptyPage);

		const result = await loadedValue<SchedulePageData>(await load(loadEvent('?from=&to=2026-09-12')));

		expect(result.rangeErrors).toEqual({ from: 'Enter a From day' });
		expect(scheduleCalls(fetchMock)).toHaveLength(0);
	});

	it('offers an Owner the Doulas on the roster, and never a Staff member a Visit cannot be assigned to', async () => {
		setup(200, emptyPage);

		const result = await loadedValue<SchedulePageData>(await load(loadEvent()));

		expect(result.doulas).toEqual([{ value: 'staff-1', label: 'Persephone Vandermeulen-Achterberg, CD(DONA)' }]);
		expect(result.isRosterUnavailable).toBe(false);
	});

	it('offers a Doula no Doula filter, because the roster read she would need is Owner/Admin only', async () => {
		const { fetchMock } = setup(200, emptyPage);

		const result = await loadedValue<SchedulePageData>(await load(loadEvent('', ['doula'])));

		expect(result.doulas).toEqual([]);
		expect(fetchMock.mock.calls.every(([path]) => !path.includes('/staff'))).toBe(true);
	});

	it('starts the schedule and the roster together', async () => {
		const { fetchMock } = setup(200, emptyPage);

		await load(loadEvent());

		expect(fetchMock.mock.calls.map(([path]) => new URL(path, 'https://example.test').pathname)).toEqual([
			'/api/practices/practice-1/visits',
			'/api/practices/practice-1/staff'
		]);
	});

	it('names the read, so Try again can run it again', async () => {
		setup(200, emptyPage);

		await load(loadEvent());

		expect(depends).toHaveBeenCalledWith('app:schedule');
	});

	// #2249: a refusal is the screen's to draw, after the screen has
	// changed, so the read carries the response through unread.
	it('hands a refused schedule read to the screen', async () => {
		setup(403, { code: 'FORBIDDEN', message: 'not permitted to read this' });

		const data = await load(loadEvent());

		await expect(settledRead(data)).resolves.toMatchObject({
			isLoaded: false,
			response: { status: 403 }
		});
	});

	it('draws the list when the roster read fails, with the Doula filter left waiting', async () => {
		setup(200, emptyPage, 403);

		const result = await loadedValue<SchedulePageData>(await load(loadEvent()));

		expect(result.page).toEqual(emptyPage);
		expect(result.doulas).toEqual([]);
		expect(result.isRosterUnavailable).toBe(true);
	});
});
