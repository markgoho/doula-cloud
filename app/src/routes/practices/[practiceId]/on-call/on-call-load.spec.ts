import { afterEach, describe, expect, it, vi } from 'vitest';
import { jsonResponse } from '#lib/testResponse.js';
import { loadedValue, settledRead } from '#lib/testStreamedRead.js';
import type { OnCallPageData } from './+page.js';
import { defaultRange, type Roster } from '#lib/onCall.js';

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
 * the range it resolves, the refusals it maps -- none of which reads a
 * row's content, so one thin window is the whole of what it needs.
 */
const roster: Roster = {
	from: '2026-10-01',
	to: '2026-10-31',
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

/** The `load` argument: the Practice in the path, and whatever range the
 * URL carries. */
function loadEvent(search = '') {
	return {
		params: { practiceId: 'practice-1' },
		url: new URL(`https://example.test/practices/practice-1/on-call${search}`),
	};
}

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('on-call/+page.ts load', () => {
	it('asks for the days the URL names', async () => {
		const { load } = await import('./+page.js');
		const { fetchMock } = setup(200);

		const data = await loadedValue<OnCallPageData>(await load(loadEvent('?from=2026-10-01&to=2026-10-31') as Parameters<typeof load>[0]));

		expect(fetchMock.mock.calls[0][0]).toContain(
			'/api/practices/practice-1/on-call?from=2026-10-01&to=2026-10-31'
		);
		expect(data.range).toEqual({ from: '2026-10-01', to: '2026-10-31' });
		expect(data.roster.windows.length).toBeGreaterThan(0);
	});

	it('opens on the next 30 days when the URL names none, so a first visit is not empty (#2026)', async () => {
		const { load } = await import('./+page.js');
		const { fetchMock } = setup(200);

		const data = await loadedValue<OnCallPageData>(await load(loadEvent() as Parameters<typeof load>[0]));

		expect(data.range).toEqual(defaultRange());
		expect(fetchMock.mock.calls[0][0]).toContain(
			`from=${data.range.from}&to=${data.range.to}`
		);
	});

	// #2249: the screen has changed by the time a refusal lands, so the
	// screen follows it; the read carries the response through unread.
	it('hands a refusal to the screen rather than throwing mid-load', async () => {
		const { load } = await import('./+page.js');
		setup(400, 'Choose a range of 92 days or fewer');

		const data = await load(loadEvent('?from=2026-01-01&to=2026-12-31') as Parameters<typeof load>[0]);

		await expect(settledRead(data)).resolves.toMatchObject({ isLoaded: false, response: { status: 400 } });
	});
});
