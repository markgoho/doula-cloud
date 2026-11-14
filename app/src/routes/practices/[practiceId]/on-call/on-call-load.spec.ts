import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { jsonResponse } from '#lib/testResponse.js';
import { loadedValue, settledRead } from '#lib/testStreamedRead.js';
import { FROM_BEFORE_TODAY, TOO_MANY_DAYS, type Roster } from '#lib/onCall.js';
import { loadEvent } from '#testing/loadEvent.js';
import type { PracticeSession } from '../+layout.js';
import { load, _ON_CALL_READ, type OnCallPageData } from './+page.js';

/*
 * This spec declares its own body rather than importing the fixture
 * beside it, which is the one departure from #596's rule and has a
 * mechanical reason: `page.fixture.ts` imports `OnCallScreen.svelte`, and this
 * spec runs in the `server` project, where a Svelte component is loaded
 * and never rendered. Importing the fixture here would pull every
 * component the screen composes into that project's coverage with none
 * of their render branches taken, and the 100% gate would fail on
 * components this file does not exercise. The schedule route's own load
 * spec declares its body inline for the same reason.
 *
 * What is asserted here is the load's plumbing -- the path it asks for,
 * the days it resolves, the refusals it maps -- none of which reads a
 * row's content, so one thin window is the whole of what it needs.
 */
const roster: Roster = {
	from: '2026-10-02',
	to: '2026-10-31',
	today: '2026-10-02',
	rule: { startRule: 'gestational_week', startWeek: 37, graceDays: 14 },
	windows: [
		{
			engagementId: 'eng-1',
			clientName: 'Anne-Marie Ochieng-Whitfield',
			dueDate: '2026-10-30',
			window: { start: '2026-10-09', end: '2026-11-13' },
			onCall: [],
			gaps: [],
			unstaffedDays: [],
			uncovered: true
		}
	],
	noWindow: [],
	doulas: []
};

function setup(status: number, body: unknown = roster) {
	const fetchMock = vi.fn(async (path: string) => {
		void path;
		return jsonResponse(body, status);
	});
	vi.stubGlobal('fetch', fetchMock);
	return { fetchMock };
}

/** The `load` argument: the Practice in the path, and whatever narrowing the
 * URL carries. */
function eventFor(search = '', session: Partial<PracticeSession> = {}) {
	const depends = vi.fn();
	const event = loadEvent(load, {
		depends,
		params: { practiceId: 'practice-1' },
		url: new URL(`https://example.test/practices/practice-1/on-call${search}`),
		parent: async () => ({
			session: { timezone: 'America/New_York', roles: ['owner'], staffId: 'staff-1', isContractor: false, ...session } as PracticeSession
		})
	});
	return { depends, event };
}

const run = ({ event }: ReturnType<typeof eventFor>) => load(event);

beforeEach(() => {
	vi.useFakeTimers({ toFake: ['Date'] });
	// 9pm on the 2nd in New York, already the 3rd in Tokyo.
	vi.setSystemTime(new Date('2026-10-03T01:00:00Z'));
});

afterEach(() => {
	vi.unstubAllGlobals();
	vi.useRealTimers();
});

describe('on-call/+page.ts load', () => {
	it('is read again when the screen asks to try again', async () => {
		setup(200);
		const call = eventFor();

		await run(call);

		expect(call.depends).toHaveBeenCalledWith(_ON_CALL_READ);
	});

	it('asks for the days the URL names', async () => {
		const { fetchMock } = setup(200);

		const data = await loadedValue<OnCallPageData>(await run(eventFor('?from=2026-10-05&to=2026-10-31')));

		expect(fetchMock.mock.calls[0]![0]).toContain('/api/practices/practice-1/on-call?from=2026-10-05&to=2026-10-31');
		expect(data.filters).toMatchObject({ from: '2026-10-05', to: '2026-10-31' });
		expect(data.roster?.windows.length).toBeGreaterThan(0);
	});

	it('opens on the next 30 days when the URL names none, counting today as the first (#2026)', async () => {
		const { fetchMock } = setup(200);

		const data = await loadedValue<OnCallPageData>(await run(eventFor()));

		expect(data.filters).toMatchObject({ from: '2026-10-02', to: '2026-10-31' });
		expect(fetchMock.mock.calls[0]![0]).toContain('from=2026-10-02&to=2026-10-31');
	});

	it('opens on today in the Practice’s own zone (ADR-0036)', async () => {
		const { fetchMock } = setup(200);

		const data = await loadedValue<OnCallPageData>(await run(eventFor('?days=tonight', { timezone: 'Asia/Tokyo' })));

		expect(data.filters).toMatchObject({ from: '2026-10-03', to: '2026-10-03' });
		expect(fetchMock.mock.calls[0]![0]).toContain('from=2026-10-03&to=2026-10-03');
	});

	it('asks for one Doula, the births that need cover, and the order', async () => {
		const { fetchMock } = setup(200);

		await run(eventFor('?doula=staff-2&cover=needed&order=cover'));

		expect(fetchMock.mock.calls[0]![0]).toContain('&staffId=staff-2&cover=needed&order=cover');
	});

	it('reads Mine as the reader’s own id', async () => {
		const { fetchMock } = setup(200);

		await run(eventFor('?mine=true', { roles: ['doula'], hasOtherDoulas: true }));

		expect(fetchMock.mock.calls[0]![0]).toContain('&staffId=staff-1');
	});

	it('shows no births for Mine beside a different Doula', async () => {
		setup(200);

		const data = await loadedValue<OnCallPageData>(await run(eventFor('?mine=true&doula=staff-2', { roles: ['owner', 'doula'], hasOtherDoulas: true })));

		expect(data.roster?.windows).toEqual([]);
		expect(data.roster?.noWindow).toEqual([]);
	});

	it('answers days that are not a range in place, with no read', async () => {
		const { fetchMock } = setup(200);

		const data = await loadedValue<OnCallPageData>(await run(eventFor('?from=2026-10-01&to=2026-10-20')));

		expect(fetchMock).not.toHaveBeenCalled();
		expect(data.roster).toBeUndefined();
		expect(data.rangeErrors).toEqual({ from: FROM_BEFORE_TODAY });
	});

	it('answers a range over 92 days in place', async () => {
		const { fetchMock } = setup(200);

		const data = await loadedValue<OnCallPageData>(await run(eventFor('?from=2026-10-03&to=2027-03-01')));

		expect(fetchMock).not.toHaveBeenCalled();
		expect(data.rangeErrors).toEqual({ to: TOO_MANY_DAYS });
	});

	it('maps the BFF’s refusal of the days onto the same fields, never the error page', async () => {
		setup(400, { code: 'INVALID_ARGUMENT', message: 'The days could not be shown.', details: { to: TOO_MANY_DAYS } });

		const data = await loadedValue<OnCallPageData>(await run(eventFor('?from=2026-10-03&to=2026-12-01')));

		expect(data.rangeErrors).toEqual({ to: TOO_MANY_DAYS });
		expect(data.roster).toBeUndefined();
	});

	// #2249: the screen has changed by the time a refusal lands, so the
	// screen follows it; the read carries the response through unread.
	it('hands any other refusal to the screen rather than throwing mid-load', async () => {
		setup(500, 'broken');

		const data = await run(eventFor());

		await expect(settledRead(data)).resolves.toMatchObject({ isLoaded: false, response: { status: 500 } });
	});

	it('hands a 400 that is not about the days to the screen too', async () => {
		setup(400, { code: 'INVALID_ARGUMENT', message: 'Who is on call could not be shown.', details: { order: 'Choose an order this list offers.' } });

		const data = await run(eventFor());

		await expect(settledRead(data)).resolves.toMatchObject({ isLoaded: false, response: { status: 400 } });
	});
});
