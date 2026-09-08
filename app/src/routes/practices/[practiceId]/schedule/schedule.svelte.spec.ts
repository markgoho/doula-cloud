import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { addDays, defaultScheduleRange } from '#lib/visitSchedule.js';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
// DataTable's frame needs stack-l's display:block default (primitives.css)
// to work as a container-query context -- see DataTable.svelte.spec.ts. This
// route's ListPage (#491) also needs the primitives registered, not just
// their CSS: <center-l max="none"> only lifts the default var(--measure) cap
// via the custom element's own attribute handling, and an unregistered
// center-l never runs it, leaving every DataTable narrower than its floor.
import '#lib/styles/app.css';
import Page from './+page.svelte';
import { toPageState } from '../../../routeFixture.js';
import { data, fixture, schedulePage } from './page.fixture.js';
import type { SchedulePageData } from './+page.js';

/*
 * The screen's content and the `page` it reads both come from the route's
 * own fixture (#596), the same installation contracts.svelte.spec.ts uses.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

if (!customElements.get('center-l')) registerLayoutPrimitives();

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const [earlierVisit, laterVisit] = schedulePage.items;
const { practiceId } = fixture.params;
const schedulePath = `/practices/${practiceId}/schedule`;

beforeEach(() => {
	apiFetchWithSession.mockReset();
	goto.mockReset();
});

// `session` merges in from practices/[practiceId]/+layout.ts (#835) --
// this route reads it only through its own `load`, but the generated
// `data` prop type requires it, since SvelteKit really does merge
// ancestor layout data into it at runtime.
const sessionStub = {
	practiceId,
	staffId: 'staff-1',
	practiceName: 'Riverside Doula Collective',
	roles: ['owner'],
	isContractor: false
};

async function setup(pageData: SchedulePageData = data) {
	// Wide enough for DataTable's <table> rather than the <dl> record view
	// its content floor stacks into below 46rem (#508) -- the same call the
	// Contract list's own spec makes for the same reason.
	await testPage.viewport(1440, 900);
	return render(Page, {
		params: fixture.params,
		data: { ...pageData, session: sessionStub }
	});
}

describe('the Practice-wide schedule (#263)', () => {
	it('lists every scheduled Visit at the Practice, with when, whose and who is covering', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('heading', { name: fixture.readyText }))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: earlierVisit.clientName }))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: laterVisit.clientName }))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('cell', { name: earlierVisit.staffName, exact: true }))
			.toBeVisible();

		// The instant is carried underneath the words a person reads
		// (ADR-0022), so a screen reader, a hover and a copy-paste all get
		// the real one rather than the rendered one. The rendered form is
		// the reader's own zone, so the assertion is on the machine-readable
		// value; `querySelector` because an attribute has no accessible
		// query of its own.
		const whenCell = testPage.getByRole('cell', { name: /Sep 2026/ }).first();
		await expect.element(whenCell).toBeVisible();
		expect(whenCell.element().querySelector('time')).toHaveAttribute(
			'datetime',
			earlierVisit.scheduledAt
		);
	});

	it('names each row as the way in to the Visit’s Engagement', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('link', { name: earlierVisit.clientName }))
			.toHaveAttribute('href', `/practices/${practiceId}/engagements/${earlierVisit.engagementId}`);
	});

	it('says an empty result in words rather than rendering a bare table', async () => {
		await setup({ ...data, page: { items: [], hasMore: false } });

		await expect
			.element(
				testPage.getByRole('cell', {
					name: 'No visits are scheduled in this range. Widen the dates, or choose every doula, to see more.'
				})
			)
			.toBeVisible();
		await expect
			.element(testPage.getByText('No scheduled visits in this range.'))
			.toBeVisible();
	});

	it('announces how many Visits the narrowed view is showing', async () => {
		await setup();

		const summary = testPage.getByText('Showing 2 scheduled visits.');
		await expect.element(summary).toBeVisible();
		await expect.element(summary).toHaveAttribute('aria-live', 'polite');
	});

	it('offers the narrowing as named form controls, each with its own label', async () => {
		await setup();

		await expect.element(testPage.getByLabelText('From')).toHaveValue(data.filters.from);
		await expect.element(testPage.getByLabelText('To')).toHaveValue(data.filters.to);
		await expect.element(testPage.getByLabelText('Doula')).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Apply' })).toBeVisible();
	});

	it('puts the narrowing in the URL, so it survives a reload and can be shared', async () => {
		await setup();

		// Both dates are said relative to today rather than written out.
		// `scheduleHref` omits a date that still equals the default range,
		// so a literal pair asserts what the URL holds on one calendar day
		// and nothing on any other -- which is how this spec came to fail
		// at midnight UTC. `To` is filled with the default itself, to
		// assert that the unchosen half stays out of the URL.
		const { from: today, to: defaultTo } = defaultScheduleRange();
		const chosenFrom = addDays(today, 5);

		await testPage.getByLabelText('From').fill(chosenFrom);
		await testPage.getByLabelText('To').fill(defaultTo);
		await testPage.getByLabelText('Doula').selectOptions(earlierVisit.staffName);
		await testPage.getByRole('button', { name: 'Apply' }).click();

		expect(goto).toHaveBeenCalledWith(
			`${schedulePath}?from=${chosenFrom}&staffId=${earlierVisit.staffId}`
		);
	});

	it('offers no Doula picker to a Staff member whose role cannot read the roster it is built from', async () => {
		await setup({ ...data, doulas: [] });

		expect(testPage.getByLabelText('Doula').elements()).toHaveLength(0);
		// The date range is still hers: only the picker's data source is
		// Owner/Admin, not the schedule itself.
		await expect.element(testPage.getByLabelText('From')).toBeVisible();
	});

	it('replaces the rows when the narrowing changes, rather than keeping the previous one’s', async () => {
		const { rerender } = await setup();

		await rerender({
			params: fixture.params,
			data: {
				...data,
				filters: { from: '2026-09-18', to: '2026-09-20' },
				page: { items: [laterVisit], hasMore: false },
				session: sessionStub
			}
		});

		await expect.element(testPage.getByRole('link', { name: laterVisit.clientName })).toBeVisible();
		expect(testPage.getByRole('link', { name: earlierVisit.clientName }).elements()).toHaveLength(0);
		// The controls follow the URL too, so what is shown and what was
		// asked for cannot drift apart.
		await expect.element(testPage.getByLabelText('From')).toHaveValue('2026-09-18');
	});

	it('appends the next page rather than replacing the one already read, continuing the same narrowing', async () => {
		apiFetchWithSession.mockResolvedValue(
			jsonResponse({
				items: [
					{
						visitId: 'visit-3',
						engagementId: 'eng-3',
						clientName: 'Cleo',
						staffId: 'staff-2',
						staffName: 'Bo Ng',
						scheduledAt: '2026-09-25T09:00:00Z'
					}
				],
				hasMore: false
			})
		);

		await setup({ ...data, page: { ...schedulePage, hasMore: true, nextCursor: 'cursor-1' } });
		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect.element(testPage.getByRole('link', { name: 'Cleo' })).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: earlierVisit.clientName }))
			.toBeVisible();
		const [requestedPath] = apiFetchWithSession.mock.calls[0];
		const url = new URL(requestedPath, 'https://example.test');
		expect(url.pathname).toBe(`/api/practices/${practiceId}/visits`);
		expect(url.searchParams.get('cursor')).toBe('cursor-1');
	});

	it('reports a failed next page in place rather than losing the list', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse('invalid cursor', 400));

		await setup({ ...data, page: { ...schedulePage, hasMore: true, nextCursor: 'cursor-1' } });
		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect.element(testPage.getByRole('alert')).toHaveTextContent('invalid cursor');
		await expect
			.element(testPage.getByRole('link', { name: earlierVisit.clientName }))
			.toBeVisible();
	});
});
