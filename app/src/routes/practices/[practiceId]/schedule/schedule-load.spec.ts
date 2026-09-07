import { describe, expect, it, vi, afterEach } from 'vitest';
import { jsonResponse } from '#lib/testResponse.js';
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
});

describe('schedule/+page.ts load', () => {
	it('turns the URL’s calendar days into the half-open instant range the BFF takes', async () => {
		const { load } = await import('./+page.js');
		const { fetchMock } = setup(200, emptyPage);

		const result = (await load(
			loadEvent('?from=2026-09-12&to=2026-09-19&staffId=staff-1') as Parameters<typeof load>[0]
		)) as SchedulePageData;

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
		const { load } = await import('./+page.js');
		const { fetchMock } = setup(200, emptyPage);

		const result = (await load(loadEvent() as Parameters<typeof load>[0])) as SchedulePageData;

		const url = new URL(fetchMock.mock.calls[0][0], 'https://example.test');
		const from = new Date(url.searchParams.get('from')!);
		const to = new Date(url.searchParams.get('to')!);
		// Thirty days plus the one the exclusive upper bound adds.
		expect(Math.round((to.getTime() - from.getTime()) / 86_400_000)).toBe(31);
		expect(result.filters.staffId).toBeUndefined();
	});

	it('offers an Owner the Doulas on the roster, and never a Staff member a Visit cannot be assigned to', async () => {
		const { load } = await import('./+page.js');
		setup(200, emptyPage);

		const result = (await load(loadEvent() as Parameters<typeof load>[0])) as SchedulePageData;

		expect(result.doulas).toEqual([
			{ value: 'staff-1', label: 'Persephone Vandermeulen-Achterberg, CD(DONA)' }
		]);
	});

	it('offers a Doula no Doula filter, because the roster read she would need is Owner/Admin only', async () => {
		const { load } = await import('./+page.js');
		const { fetchMock } = setup(200, emptyPage);

		const result = (await load(
			loadEvent('', ['doula']) as Parameters<typeof load>[0]
		)) as SchedulePageData;

		expect(result.doulas).toEqual([]);
		expect(fetchMock.mock.calls.every(([path]) => !path.includes('/staff'))).toBe(true);
	});

	it('redirects to login on a 401, rather than reaching for goto mid-load', async () => {
		const { load } = await import('./+page.js');
		setup(401, 'no session');

		await expect(load(loadEvent() as Parameters<typeof load>[0])).rejects.toMatchObject({
			status: 303,
			location: '/login?sessionEnded=true'
		});
	});

	it('throws a 403 SvelteKit error on a role refusal, for practices/+error.svelte to render', async () => {
		const { load } = await import('./+page.js');
		setup(403, 'not permitted to read this');

		await expect(load(loadEvent() as Parameters<typeof load>[0])).rejects.toMatchObject({
			status: 403
		});
	});

	it('throws with the response status on any other failure', async () => {
		const { load } = await import('./+page.js');
		setup(500, 'boom');

		await expect(load(loadEvent() as Parameters<typeof load>[0])).rejects.toMatchObject({
			status: 500
		});
	});
});
