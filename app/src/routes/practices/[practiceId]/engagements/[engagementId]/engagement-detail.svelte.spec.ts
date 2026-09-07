import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import { jsonResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';
// Rendering `+page.svelte` directly bypasses `+layout.svelte`, the only
// place the real app calls this -- without it a layout primitive like
// `center-l`'s own `max` override sits unregistered and inert (matching
// clients-list.svelte.spec.ts's own reason for the same line). #486's own
// DataTable assertions below need the real table-view/record-view switch.
import '#lib/styles/app.css';
import { toApiResponder, toPageState } from '../../../../routeFixture.js';
import { detail as fixtureDetail, fixture } from './page.fixture.js';
if (!customElements.get('center-l')) registerLayoutPrimitives();

/*
 * The `page` this route reads comes from its own fixture (#596), so the
 * params this spec installs and the params the continuum sweep installs
 * are one description. `vi.mock` is hoisted above every import, so the
 * object is declared empty here and filled from the fixture once the
 * imports have run.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetchWithSession,
	apiErrorMessage: vi.fn(async (response: Response) => await response.text())
}));

// A real service-worker push subscription would fail in headless Chromium
// -- the summary this spec covers doesn't depend on it, so it's stood in
// with a no-op unsubscribe.
vi.mock('#lib/pushRefresh.js', () => ({ subscribeToThreadPushMessages: vi.fn(() => vi.fn()) }));

interface Detail {
	engagementId: string;
	clientId: string;
	clientName: string;
	status: string;
	createdAt: string;
	dueDate?: string;
}

// The Engagement is handed in as `data` rather than stubbed out of a
// fetch: it comes from +page.ts's load now (#695), and that load has its
// own spec. What remains mocked is the seven sections that still fill in
// after mount, every one answered with a refusal by default -- each
// section's own loader already treats that as "nothing to show" rather
// than throwing. `activityResponse` lets #486's own tests below answer
// just that one path differently, without a blanket mock stopping being
// useful to every other test in this file.
async function setup(detail: Detail, activityResponse?: Response) {
	await testPage.viewport(1440, 900);
	if (activityResponse) {
		apiFetchWithSession.mockImplementation((path: string) =>
			Promise.resolve(path.includes('/activity') ? activityResponse : jsonResponse('not available', 403))
		);
	} else {
		apiFetchWithSession.mockResolvedValue(jsonResponse('not available', 403));
	}
	// No `params` prop: the page reads `page.params`, which the $app/state
	// `params` rides along because PageProps requires it; the page itself
	// reads `page.params`, which the $app/state mock above supplies. Both
	// come from the fixture, so the two cannot disagree about which
	// Engagement this is (#596).
	//
	// `session` merges in from practices/[practiceId]/+layout.ts (#835) --
	// this route never reads it, but the generated `data` prop type
	// requires it, since SvelteKit really does merge ancestor layout data
	// into it at runtime.
	await render(Page, {
		data: {
			...detail,
			session: {
				practiceId: fixture.params.practiceId,
				practiceName: 'Riverside Doula Collective',
				roles: [],
				isContractor: false
			}
		},
		params: fixture.params
	});
}

// Renders directly rather than through setup(): that helper's own
// activityResponse-less branch always overwrites apiFetchWithSession
// with a blanket 403 (its "everything refuses by default" default),
// which would erase the fixture responder a caller sets up itself -- the
// #841 test below takes the same direct-render path for the same reason.
async function renderWithFixtureResponder(
	respondOverride?: (path: string, init?: RequestInit) => Promise<Response> | undefined
) {
	await testPage.viewport(1440, 900);
	const respond = toApiResponder(fixture);
	apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
		const overridden = respondOverride?.(path, init);
		if (overridden) return overridden;
		return respond(path);
	});
	await render(Page, {
		data: {
			...fixtureDetail,
			session: {
				practiceId: fixture.params.practiceId,
				practiceName: 'Riverside Doula Collective',
				roles: [],
				isContractor: false
			}
		},
		params: fixture.params
	});
}

describe('Staff Engagement detail summary', () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	it('shows the due date under its own label, alongside Created (#538)', async () => {
		// The fixture's own Engagement (#596) already carries a due date.
		await setup(fixtureDetail);

		await expect.element(testPage.getByText('Due date')).toBeVisible();
		await expect.element(testPage.getByText('Mar 1, 2027')).toBeVisible();
		await expect.element(testPage.getByText('Created')).toBeVisible();
	});

	// ADR-0017: a postpartum-only Engagement has no due date. #538 asks for
	// nothing to show, not a blank row and not a placeholder -- the row is
	// left out of the DescriptionList's own items entirely. The fixture's
	// Engagement always carries a due date, so a Detail with none is this
	// test's own -- the fixture has no way to hold the absence (#596).
	it('shows nothing for a null due date -- no blank label, no placeholder', async () => {
		await setup({ ...fixtureDetail, dueDate: undefined });

		await expect.element(testPage.getByText('active')).toBeVisible();
		await expect.element(testPage.getByText(/due date/i)).not.toBeInTheDocument();
	});
});

// #486 AC4: the same ledger treatment reused on the staff Engagement page,
// through the record-scoped read engagement.ListActivityHandler already
// exposes at /activity -- last in the page's own sections.
describe('the Activity ledger section (#486)', () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	it('renders the ledger under an Activity heading, low on the page', async () => {
		await setup(
			fixtureDetail,
			jsonResponse({
				items: [
					{
						subjectKind: 'engagement',
						subjectId: fixtureDetail.engagementId,
						action: 'visit_logged',
						actorKind: 'staff',
						actorName: 'Maya Torres',
						createdAt: new Date().toISOString()
					}
				],
				hasMore: false
			})
		);

		await expect.element(testPage.getByRole('heading', { name: 'Activity' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Visit logged' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Maya Torres' })).toBeVisible();
	});

	it('shows the empty-ledger message when the Engagement has no activity yet', async () => {
		await setup(fixtureDetail, jsonResponse({ items: [], hasMore: false }));

		await expect.element(testPage.getByRole('cell', { name: 'Nothing has happened yet.' })).toBeVisible();
	});

	it('says so when the ledger cannot be read', async () => {
		await setup(fixtureDetail, jsonResponse('nope', 403));

		await expect.element(testPage.getByText('nope')).toBeVisible();
	});
});

// #250: a Visit's own scheduled date, distinct from createdAt.
describe('the Visits section Date column and schedule control (#250)', () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	it('shows a formatted date for a scheduled Visit and "Not yet scheduled" for one without', async () => {
		await renderWithFixtureResponder();

		// The fixture's own two rows (page.fixture.ts): visit-1 carries
		// scheduledAt, visit-2 carries none. The expected cell text is built
		// from the same Date methods formatScheduledVisit itself uses
		// (dates.spec.ts's own reason) rather than a literal UTC string, so
		// this passes regardless of the runner's own time zone.
		const scheduled = new Date('2027-03-15T14:30:00Z');
		const hour12 = scheduled.getHours() % 12 === 0 ? 12 : scheduled.getHours() % 12;
		const minute = scheduled.getMinutes().toString().padStart(2, '0');
		const suffix = scheduled.getHours() < 12 ? 'am' : 'pm';
		const expected = `${scheduled.getDate()} ${scheduled.toLocaleDateString('en-US', { month: 'short' })} ${scheduled.getFullYear()}, ${hour12}:${minute}${suffix}`;
		await expect.element(testPage.getByRole('cell', { name: expected, exact: true })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Not yet scheduled', exact: true })).toBeVisible();
	});

	it('sets an unscheduled Visit schedule from its own row control', async () => {
		const requests: { path: string; body: unknown }[] = [];
		await renderWithFixtureResponder((path, init) => {
			if (!init || init.method !== 'PATCH' || !path.endsWith('/schedule')) return;
			requests.push({ path, body: init.body ? JSON.parse(init.body as string) : undefined });
			return Promise.resolve(jsonResponse({ visitId: 'visit-2', scheduledAt: '2027-04-01T09:00:00Z' }));
		});

		// Jordan Reyes is visit-2, the fixture's unscheduled row (#250) --
		// its own field is scoped by that row's unique label id. `exact`
		// excludes the "Add a Visit" form's own field just above the
		// table, whose label ("...(optional)") contains this one's as a
		// substring.
		const field = testPage.getByLabelText('Scheduled date and time', { exact: true }).nth(1);
		await field.fill('2027-04-01T09:00');
		await testPage.getByRole('button', { name: 'Update schedule' }).nth(1).click();

		await expect.poll(() => requests).toHaveLength(1);
		expect(requests[0]!.path).toContain('/visits/visit-2/schedule');
		expect(requests[0]!.body).toEqual({ scheduledAt: new Date('2027-04-01T09:00').toISOString() });
	});

	it('creates a Visit already scheduled from the Add a Visit form', async () => {
		const requests: { path: string; body: unknown }[] = [];
		await renderWithFixtureResponder((path, init) => {
			if (!init || init.method !== 'POST' || !path.endsWith('/visits')) return;
			requests.push({ path, body: init.body ? JSON.parse(init.body as string) : undefined });
			return Promise.resolve(jsonResponse({ visitId: 'visit-3', staffId: 'staff-1' }, 201));
		});

		await testPage.getByLabelText('Scheduled date and time (optional)').fill('2027-05-20T10:15');
		await testPage.getByRole('button', { name: 'Add a Visit' }).click();

		await expect.poll(() => requests).toHaveLength(1);
		expect(requests[0]!.path).toContain('/visits');
		expect(requests[0]!.body).toEqual({ scheduledAt: new Date('2027-05-20T10:15').toISOString() });
	});
});

// #841: each SectionState is its own instance, so one section's failure
// must not touch another's -- proved here rather than by the default
// "everything answers 403" mock every other test in this file uses, since
// that shows every section failing at once, not one failing independently.
describe('a section fails on its own, independent of the others (#841)', () => {
	beforeEach(() => {
		apiFetchWithSession.mockReset();
	});

	it("shows the Contract section's own failure while Visits still renders its fixture row", async () => {
		await testPage.viewport(1440, 900);
		const respond = toApiResponder(fixture);
		apiFetchWithSession.mockImplementation((path: string) =>
			path.endsWith('/contract') ? Promise.resolve(jsonResponse('contract is down', 500)) : respond(path)
		);

		await render(Page, {
			data: {
				...fixtureDetail,
				session: {
					practiceId: fixture.params.practiceId,
					practiceName: 'Riverside Doula Collective',
					roles: [],
					isContractor: false
				}
			},
			params: fixture.params
		});

		await expect.element(testPage.getByText('contract is down')).toBeVisible();
		// Scoped to the Visits section by its own label: the fixture's
		// Visit's staffName also appears in the reassign row's
		// visually-hidden name and, once Activity's own fixture entry
		// loads, in that section too -- an unscoped query is ambiguous.
		await expect
			.element(
				testPage.getByLabelText('Visits').getByRole('cell', { name: 'Anne-Marie Ochieng-Whitfield', exact: true })
			)
			.toBeVisible();
	});
});
