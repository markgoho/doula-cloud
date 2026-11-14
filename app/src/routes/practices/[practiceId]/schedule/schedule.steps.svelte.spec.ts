/*
 * #2870: Earlier and Later step the applied days, and the bar folds below
 * 42rem. Its own file because the days it steps from are today's, which the
 * sibling specs fix at the fixture's September 7.
 */
import { page as testPage } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { SvelteURL } from 'svelte/reactivity';
import { render } from 'vitest-browser-svelte';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import '#lib/styles/app.css';
import Page from './ScheduleScreen.svelte';
// jscpd:ignore-start -- the route-spec mock preamble every list's spec repeats
import { practiceSession, toPageState } from '../../../routeFixture.js';
import { data, fixture } from './page.fixture.js';
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

async function setupAt(query = '', { search = '', width = 1440 }: { search?: string; width?: number } = {}) {
	pageState.url = new SvelteURL(`https://example.test${schedulePath}${query}`);
	pageState.state = search ? { scheduleSearch: search } : {};
	await testPage.viewport(width, 900);
	const parameters = new URLSearchParams(query);
	const chosen = parameters.has('from') || parameters.has('to') ? { from: parameters.get('from') ?? '', to: parameters.get('to') ?? '' } : undefined;
	const view = {
		...data.view,
		preset: parameters.get('days') === '7' ? (7 as const) : (30 as const),
		chosen,
		order: parameters.get('order') === 'latest' ? ('latest' as const) : ('' as const)
	};
	if (chosen && (!chosen.from || !chosen.to)) {
		const filters = { from: '', to: '', timeZone: data.filters.timeZone };
		return render(Page, { data: { ...data, view, filters, rangeErrors: { from: 'Enter a From day' } } });
	}
	return render(Page, { data: { ...data, view, filters: { ...data.filters, ...chosen } } });
}

const link = (name: string) => testPage.getByRole('link', { name });

describe('Earlier and Later (#2870)', () => {
	it('step the next 30 days by 30, named for the days they land on', async () => {
		await setupAt();

		const earlier = link('Earlier, Sat Aug 8 to Sun Sep 6');
		await expect.element(earlier).toHaveAttribute('href', `${schedulePath}?from=2026-08-08&to=2026-09-06`);
		await expect.element(earlier).toHaveTextContent('Earlier');
		await expect
			.element(link('Later, Wed Oct 7 to Thu Nov 5'))
			.toHaveAttribute('href', `${schedulePath}?from=2026-10-07&to=2026-11-05`);
	});

	it('step the next 7 days by 7, and write the way back to today as the preset', async () => {
		await setupAt('?days=7');

		await expect
			.element(link('Later, Mon Sep 14 to Sun Sep 20'))
			.toHaveAttribute('href', `${schedulePath}?from=2026-09-14&to=2026-09-20`);
		await expect
			.element(link('Earlier, Mon Aug 31 to Sun Sep 6'))
			.toHaveAttribute('href', `${schedulePath}?from=2026-08-31&to=2026-09-06`);
	});

	it('step chosen days by their own length, and land on today’s days as the bare path or ?days=7', async () => {
		await setupAt('?from=2026-08-08&to=2026-09-06');
		await expect.element(link('Later, Mon Sep 7 to Tue Oct 6')).toHaveAttribute('href', schedulePath);

		await setupAt('?from=2026-08-31&to=2026-09-06');
		await expect.element(link('Later, Mon Sep 7 to Sun Sep 13')).toHaveAttribute('href', `${schedulePath}?days=7`);
	});

	it('read the days stepped to as Choose days, with From and To filled', async () => {
		await setupAt('?from=2026-08-08&to=2026-09-06');

		await expect.element(testPage.getByRole('radio', { name: 'Choose days' })).toBeChecked();
		await expect.element(testPage.getByLabelText('From')).toHaveValue('2026-08-08');
		await expect.element(testPage.getByLabelText('To', { exact: true })).toHaveValue('2026-09-06');
	});

	it('keep Mine, the Doula, the narrowing to Visits waiting and the order, and Earlier still goes earlier under latest first', async () => {
		await setupAt('?order=latest&needsDoula=true');

		await expect
			.element(link('Earlier, Sat Aug 8 to Sun Sep 6'))
			.toHaveAttribute('href', `${schedulePath}?from=2026-08-08&to=2026-09-06&needsDoula=true&order=latest`);
	});

	it('hand the applied Client search on in the history entry’s state, and move with no full load', async () => {
		await setupAt('', { search: 'Camille' });
		apiFetchWithSession.mockResolvedValue(Response.json({ items: [], hasMore: false }));

		await link('Later, Wed Oct 7 to Thu Nov 5').click();

		expect(goto).toHaveBeenCalledWith(`${schedulePath}?from=2026-10-07&to=2026-11-05`, {
			state: { scheduleSearch: 'Camille' }
		});
	});

	it('move with an empty search state when none is applied', async () => {
		await setupAt();

		await link('Earlier, Sat Aug 8 to Sun Sep 6').click();

		expect(goto).toHaveBeenCalledWith(`${schedulePath}?from=2026-08-08&to=2026-09-06`, {
			state: { scheduleSearch: '' }
		});
	});

	it('are not drawn where the address names days that are not a range', async () => {
		await setupAt('?from=&to=2026-09-06');

		expect(testPage.getByRole('link', { name: /^Earlier/ }).elements()).toHaveLength(0);
		expect(testPage.getByRole('link', { name: /^Later/ }).elements()).toHaveLength(0);
	});

	it('are one tag away from today’s default: the Days tag’s × is the default days', async () => {
		await setupAt('?from=2026-08-08&to=2026-09-06');

		await expect
			.element(testPage.getByRole('link', { name: 'Remove filter: Days Sat Aug 8 to Sun Sep 6' }))
			.toHaveAttribute('href', schedulePath);
	});
});

describe('the bar below 42rem (#2870)', () => {
	it('folds Show only and the Doula with Order under one summary, and keeps Days, Earlier, Later, Client and Apply in view', async () => {
		await setupAt('', { width: 375 });

		const summary = testPage.getByText('More filters and order');
		await expect.element(summary).toBeVisible();
		for (const label of ['Mine', 'Needing a new Doula', 'Doula', 'Order']) {
			expect(testPage.getByLabelText(label, { exact: true }).element().checkVisibility(), `${label} is folded away`).toBe(false);
		}
		await expect.element(testPage.getByRole('radio', { name: 'Next 30 days' })).toBeVisible();
		await expect.element(link('Earlier, Sat Aug 8 to Sun Sep 6')).toBeVisible();
		await expect.element(testPage.getByLabelText('Client')).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Apply' })).toBeVisible();

		await summary.click();

		await expect.element(testPage.getByRole('combobox', { name: 'Doula' })).toBeVisible();
		await expect.element(testPage.getByRole('combobox', { name: 'Order' })).toBeVisible();
	});

	it('is just "Order" for a reader who has nothing else to fold', async () => {
		pageState.data = { session: { ...practiceSession(['doula']).session, isContractor: true } };
		await setupAt('', { width: 375 });

		await expect.element(testPage.getByText('Order', { exact: true }).first()).toBeVisible();
		expect(testPage.getByText('Mine').elements()).toHaveLength(0);
		expect(testPage.getByText('More filters and order').elements()).toHaveLength(0);
	});

	it('draws everything in the row at 42rem and wider, with no summary', async () => {
		await setupAt('', { width: 1440 });

		expect(testPage.getByText('More filters and order').element().checkVisibility()).toBe(false);
		await expect.element(testPage.getByRole('checkbox', { name: 'Mine' })).toBeVisible();
		await expect.element(testPage.getByRole('combobox', { name: 'Order' })).toBeVisible();
	});

	it('still sends the order from a closed fold', async () => {
		await setupAt('?order=latest', { width: 375 });

		const form = testPage.getByRole('button', { name: 'Apply' }).element().closest('form')!;
		expect(new FormData(form).get('order')).toBe('latest');
	});
});
