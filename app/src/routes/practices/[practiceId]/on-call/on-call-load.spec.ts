import { afterEach, describe, expect, it, vi } from 'vitest';
import { jsonResponse } from '#lib/testResponse.js';
import type { OnCallPageData } from './+page.js';
import type { Roster } from '#lib/onCall.js';

/*
 * This spec declares its own body rather than importing the fixture
 * beside it, which is the one departure from #596's rule and has a
 * mechanical reason: `page.fixture.ts` imports `+page.svelte`, and this
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

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

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

		const data = (await load(
			loadEvent('?from=2026-10-01&to=2026-10-31') as Parameters<typeof load>[0]
		)) as OnCallPageData;

		expect(fetchMock.mock.calls[0][0]).toContain(
			'/api/practices/practice-1/on-call?from=2026-10-01&to=2026-10-31'
		);
		expect(data.range).toEqual({ from: '2026-10-01', to: '2026-10-31' });
		expect(data.roster.windows.length).toBeGreaterThan(0);
	});

	it('opens on today when the URL names no days, because the question is tonight', async () => {
		const { load } = await import('./+page.js');
		const { fetchMock } = setup(200);

		const data = (await load(
			loadEvent() as Parameters<typeof load>[0]
		)) as OnCallPageData;

		expect(data.range.from).toBe(data.range.to);
		expect(fetchMock.mock.calls[0][0]).toContain(
			`from=${data.range.from}&to=${data.range.to}`
		);
	});

	it('sends a reader whose session ended to the staff login', async () => {
		const { load } = await import('./+page.js');
		setup(401, 'unauthorized');

		await expect(
			load(loadEvent() as Parameters<typeof load>[0])
		).rejects.toMatchObject({ status: 303 });
	});

	it('raises the refusal a reader who may not read this Practice meets', async () => {
		const { load } = await import('./+page.js');
		setup(403, 'forbidden');

		await expect(
			load(loadEvent() as Parameters<typeof load>[0])
		).rejects.toBeDefined();
	});

	it('raises the BFF’s own sentence for a range it refuses', async () => {
		const { load } = await import('./+page.js');
		setup(400, 'Choose a range of 92 days or fewer');

		await expect(
			load(
				loadEvent('?from=2026-01-01&to=2026-12-31') as Parameters<
					typeof load
				>[0]
			)
		).rejects.toMatchObject({
			status: 400,
		});
	});
});
