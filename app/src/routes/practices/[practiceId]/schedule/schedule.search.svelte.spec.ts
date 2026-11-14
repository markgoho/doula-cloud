/*
 * #2870: finding a Client's Visits by her name, and the bar and the list
 * agreeing while a newer search loads.
 *
 * Its own file because the search lives in the history entry's state
 * (`page.state`), a page state the sibling spec never sets.
 */
import { page as testPage } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { SvelteMap, SvelteURL } from 'svelte/reactivity';
import { cleanup, render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import '#lib/styles/app.css';
import Page from './ScheduleScreen.svelte';
// jscpd:ignore-start -- the route-spec mock preamble every list's search spec repeats
import { toPageState } from '../../../routeFixture.js';
import { data, fixture, schedulePage } from './page.fixture.js';
import type { SchedulePageData } from './+page.js';
import { apiFetchWithSession, goto, pageState } from '#testing/routePage.js';

vi.mock('$app/state', () => import('#testing/routePage.js'));
vi.mock('#lib/api.js', () => import('#testing/routePage.js'));
vi.mock('$app/navigation', async () => ({
	...(await import('#testing/routePage.js')),
	afterNavigate: vi.fn(),
	beforeNavigate: vi.fn(),
	snapshot: vi.fn()
}));
// jscpd:ignore-end
if (!customElements.get('center-l')) registerLayoutPrimitives();

const { practiceId } = fixture.params;
const schedulePath = `/practices/${practiceId}/schedule`;
const listPath = `/api/practices/${practiceId}/visits/list`;
const [earlierVisit, laterVisit] = schedulePage.items;
const FIELD = 'Client';

beforeEach(() => {
	vi.useFakeTimers({ toFake: ['Date'] });
	vi.setSystemTime(new Date('2026-09-07T13:00:00Z'));
	apiFetchWithSession.mockReset();
	goto.mockReset();
	goto.mockResolvedValue(undefined);
	Object.assign(pageState, toPageState(fixture));
});

afterEach(() => {
	vi.useRealTimers();
});

interface SetupOptions {
	search?: string;
	query?: string;
	read?: SchedulePageData | undefined;
}

async function setupSearch({ search = '', query = '', read = data }: SetupOptions = {}) {
	pageState.url = new SvelteURL(`https://example.test${schedulePath}${query}`);
	pageState.state = search ? { scheduleSearch: search } : {};
	await testPage.viewport(1440, 900);
	return render(Page, { data: read });
}

// jscpd:ignore-start -- reads the last POST body, as the Invoices list's search spec does
function lastBody(): Record<string, unknown> {
	const call = apiFetchWithSession.mock.calls.at(-1)!;
	return JSON.parse((call[1] as RequestInit).body as string) as Record<string, unknown>;
}
// jscpd:ignore-end

const found = (items = [earlierVisit], extra: object = {}) =>
	jsonResponse({ items, hasMore: false, visitCount: items.length, ...extra });

describe('finding a Client’s Visits by her name (#2870)', () => {
	it('offers one field for a Client, first among the fields, which never reaches the address', async () => {
		await setupSearch();

		const field = testPage.getByRole('searchbox', { name: FIELD });
		await expect.element(field).toBeVisible();
		await expect.element(field).not.toHaveAttribute('name');
		await expect.element(field).toHaveAttribute('maxlength', '100');
	});

	it('applies the typed search with the days, in the history entry and not the address', async () => {
		await setupSearch();

		await testPage.getByRole('radio', { name: 'Next 7 days' }).click();
		await testPage.getByRole('searchbox', { name: FIELD }).fill('  Camille ');
		await testPage.getByRole('button', { name: 'Apply' }).click();

		expect(goto).toHaveBeenLastCalledWith(`${schedulePath}?days=7`, { state: { scheduleSearch: 'Camille' } });
	});

	it('reads a search in a POST body under the days, and says what was looked for', async () => {
		apiFetchWithSession.mockResolvedValue(found());
		await setupSearch({ search: 'Anne' });

		await expect
			.element(testPage.getByRole('status'))
			.toHaveTextContent(/^Showing 1 Visit for "Anne" from Mon\sSep\s7 to Tue\sOct\s6\.$/);
		expect(apiFetchWithSession).toHaveBeenLastCalledWith(listPath, expect.objectContaining({ method: 'POST' }));
		expect(lastBody()).toEqual({
			from: '2026-09-07T04:00:00.000Z',
			to: '2026-10-07T04:00:00.000Z',
			staffId: '',
			order: '',
			search: 'Anne',
			cursor: ''
		});
		await expect.element(testPage.getByRole('searchbox', { name: FIELD })).toHaveValue('Anne');
		const otherClient = new RegExp(`^${laterVisit.clientName.slice(0, 10)}`);
		expect(testPage.getByRole('link', { name: otherClient }).elements()).toHaveLength(0);
	});

	it('names the order and the total once more than one Visit matches', async () => {
		apiFetchWithSession.mockResolvedValue(found([earlierVisit, laterVisit], { hasMore: true, nextCursor: 'c1', visitCount: 41 }));
		await setupSearch({ search: 'Anne' });

		await expect
			.element(testPage.getByRole('status'))
			.toHaveTextContent(/^Showing the first 2 of 41 Visits for "Anne" from Mon\sSep\s7 to Tue\sOct\s6, soonest first\.$/);
	});

	it('shows a removable tag for the search, each tag keeping the other', async () => {
		apiFetchWithSession.mockResolvedValue(found());
		await setupSearch({ search: 'Anne', query: '?days=7' });

		await testPage.getByRole('link', { name: 'Remove filter: Client Anne' }).click();
		expect(goto).toHaveBeenLastCalledWith(`${schedulePath}?days=7`, { state: { scheduleSearch: '' } });

		await testPage.getByRole('link', { name: 'Remove filter: Next 7 days' }).click();
		expect(goto).toHaveBeenLastCalledWith(schedulePath, { state: { scheduleSearch: 'Anne' } });

		await expect
			.element(testPage.getByRole('link', { name: 'Clear filters' }))
			.toHaveAttribute('href', schedulePath);
	});

	it('draws Clear filters beside a search that is the only filter', async () => {
		apiFetchWithSession.mockResolvedValue(found());
		await setupSearch({ search: 'Anne' });

		await expect.element(testPage.getByRole('link', { name: 'Remove filter: Client Anne' })).toBeVisible();
	});

	it('says when nothing matched, and offers the same search over the next 12 months', async () => {
		apiFetchWithSession.mockResolvedValue(found([]));
		await setupSearch({ search: 'Zelda' });

		await expect
			.element(testPage.getByText(/^No Visits for "Zelda" from Mon\sSep\s7 to Tue\sOct\s6\.$/).last())
			.toBeVisible();
		await testPage.getByRole('button', { name: 'Look for "Zelda" in the next 12 months' }).click();

		expect(goto).toHaveBeenLastCalledWith(`${schedulePath}?from=2026-09-07&to=2027-09-06`, {
			state: { scheduleSearch: 'Zelda' }
		});
	});

	it('offers no wider look when the days already reach a year ahead', async () => {
		apiFetchWithSession.mockResolvedValue(found([]));
		await setupSearch({
			search: 'Zelda',
			query: '?from=2026-09-07&to=2027-09-06',
			read: { ...data, view: { ...data.view, chosen: { from: '2026-09-07', to: '2027-09-06' } } }
		});

		await expect.element(testPage.getByText(/^No Visits for "Zelda"/).last()).toBeVisible();
		expect(testPage.getByRole('button', { name: /in the next 12 months/ }).elements()).toHaveLength(0);
	});

	it('keeps Mine in the way on from an empty search, and says whose Visits', async () => {
		apiFetchWithSession.mockResolvedValue(found([]));
		await setupSearch({
			search: 'Zelda',
			query: '?mine=true',
			read: { ...data, view: { ...data.view, isMine: true } }
		});

		await expect.element(testPage.getByText(/^You have no Visits for "Zelda"/).last()).toBeVisible();
	});

	it('keeps the search on every later page', async () => {
		apiFetchWithSession
			.mockResolvedValueOnce(found([earlierVisit], { hasMore: true, nextCursor: 'cursor-1', visitCount: 2 }))
			.mockResolvedValueOnce(found([laterVisit], { visitCount: undefined }));
		await setupSearch({ search: 'Anne' });

		await expect.element(testPage.getByRole('button', { name: 'Load more' })).toBeVisible();
		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect.poll(() => apiFetchWithSession.mock.calls.length).toBe(2);
		expect(lastBody()).toMatchObject({ search: 'Anne', cursor: 'cursor-1' });
	});

	it('says a search that did not run, leaves the list unmarked, and Try again runs it again', async () => {
		apiFetchWithSession.mockResolvedValueOnce(new Response('no', { status: 500 }));
		await setupSearch({ search: 'Anne' });

		const said = testPage.getByText('Your search did not run. Nothing you saved is lost.');
		await expect.element(said).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Apply' })).toBeEnabled();
		await expect.element(testPage.getByRole('searchbox', { name: FIELD })).toHaveValue('Anne');

		apiFetchWithSession.mockResolvedValueOnce(found());
		await testPage.getByRole('button', { name: 'Try again' }).click();

		await expect.element(testPage.getByText('Your search did not run. Nothing you saved is lost.')).not.toBeInTheDocument();
		expect(lastBody()).toMatchObject({ search: 'Anne', cursor: '' });
	});

	it('leaves a failed page read to the search while one is applied', async () => {
		apiFetchWithSession.mockResolvedValue(found());
		pageState.state = { scheduleSearch: 'Anne' };
		pageState.url = new SvelteURL(`https://example.test${schedulePath}`);
		await testPage.viewport(1440, 900);
		await render(Page, { hasReadFailed: true });

		await expect.element(testPage.getByRole('link', { name: new RegExp(`^${earlierVisit.clientName}`) })).toBeVisible();
		await expect.element(testPage.getByText('Your schedule did not load. Nothing you saved is lost.')).not.toBeInTheDocument();
	});

	it('answers with no read when the days are not a range', async () => {
		await setupSearch({
			search: 'Anne',
			query: '?from=2026-09-10',
			read: { ...data, view: { ...data.view, chosen: { from: '2026-09-10', to: '' } }, rangeErrors: { to: 'Enter a To day' } }
		});

		await expect.element(testPage.getByText('Enter a To day')).toBeVisible();
		expect(apiFetchWithSession).not.toHaveBeenCalled();
	});

	it('lists the whole read again when the search is removed and the address is unchanged', async () => {
		apiFetchWithSession.mockResolvedValue(found());
		// The history entry's state as SvelteKit's is: reactive, so a move that changes only it is seen.
		const entry = new SvelteMap([['scheduleSearch', 'Anne']]);
		await setupSearch({ read: data });
		cleanup();
		pageState.state = {
			get scheduleSearch() {
				return entry.get('scheduleSearch');
			}
		};
		await render(Page, { data });
		await expect.element(testPage.getByRole('status')).toHaveTextContent('for "Anne"');

		entry.set('scheduleSearch', '');

		await expect.element(testPage.getByRole('status')).toHaveTextContent(/^Showing 3 Visits from/);
	});
});
