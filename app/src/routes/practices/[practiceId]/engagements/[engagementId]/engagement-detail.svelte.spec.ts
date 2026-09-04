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
import { toPageState } from '../../../../routeFixture.js';
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
	await render(Page, { data: detail, params: fixture.params });
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
