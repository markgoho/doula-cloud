import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import { jsonResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';
// The Skeleton reserves a line of body copy with `var(--text-body-size)`,
// so without the tokens it draws at zero height and reserves nothing --
// the very thing ADR-0020 asks it to do. The real app loads these in the
// root layout.
import '#lib/styles/app.css';
import { toApiResponder, toPageState } from '../../routeFixture.js';
import { fixture, offers, practiceName } from './page.fixture.js';

// Rendering `+page.svelte` directly bypasses `+layout.svelte`, which is the
// only place that calls this in the real app -- without it every layout
// primitive (`center-l`'s own `max="none"`, included) sits unregistered and
// inert, matching clients-list.svelte.spec.ts's own reason for the same line.
if (!customElements.get('center-l')) registerLayoutPrimitives();

/*
 * The `page` this route reads comes from its own fixture (#596), so what
 * this spec renders and what the continuum sweep measures are one
 * description. `vi.mock` is hoisted above every import, so `pageState` is
 * declared empty and filled from the fixture once the imports have run --
 * the route reads `page.params.practiceId` inside its own functions
 * rather than at module scope, so the later write is seen. Same
 * installation, through the same `toPageState`, as
 * `route-continuum.svelte.spec.ts`. The fixture's own `respond` only
 * answers what a Doula role ever fetches (session/offers/clients/
 * awaiting-reply/activity/push-subscriptions) -- most of this file's own
 * tests default to `roles: ['owner']`, which reaches `staff`/`billing`/
 * `payments/connect`/`engagement-requests` the fixture holds no content
 * for, so `setup()`'s own answers stay this spec's for those cases (#596).
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const apiFetch = vi.hoisted(() => vi.fn());
const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetch,
	apiFetchWithSession,
	// payments.ts reads a failure through this; the mock has to carry
	// every export the module tree imports, not only the ones used here.
	apiErrorMessage: (response: Response) => response.text()
}));

const registerPushSubscription = vi.hoisted(() => vi.fn());
vi.mock('#lib/pushRegistration.js', () => ({
	registerPushSubscription,
	practicePushSubscriptionsPath: (practiceId: string) =>
		`/api/practices/${practiceId}/push-subscriptions`
}));

function refusal(body: string): Response {
	return jsonResponse(body, 403);
}

const { practiceId } = fixture.params;

interface SetupOptions {
	roles?: string[];
	clients?: unknown[];
	overrides?: Record<string, Response>;
}

async function setup({ roles = ['owner'], clients = [{ clientId: 'c1' }], overrides = {} }: SetupOptions = {}) {
	const answers: Record<string, Response> = {
		session: jsonResponse({ practiceName, roles }),
		// The declined Offer has no counterpart in the fixture, which
		// answers `/offers` with only the still-open one -- so it is
		// written as a spread onto that same Offer rather than a second
		// object restating its fields (#596).
		offers: jsonResponse({ items: [...offers, { ...offers[0], offerId: 'offer-2', state: 'declined' }] }),
		clients: jsonResponse({ items: clients, hasMore: false }),
		staff: jsonResponse({
			members: [{ staffId: 's1' }, { staffId: 's2' }],
			invitations: { items: [{ expired: true }], hasMore: false }
		}),
		billing: jsonResponse({ balance: 12, ledger: { items: [], hasMore: false } }),
		connect: jsonResponse({
			status: 'onboarding_incomplete',
			cardPaymentsStatus: 'restricted',
			payoutsStatus: 'restricted',
			requirementsDue: ['individual.dob']
		}),
		'engagement-requests': jsonResponse({
			items: [{ requestId: 'request-1' }, { requestId: 'request-2' }],
			hasMore: false
		}),
		'awaiting-reply': jsonResponse({ items: [], hasMore: false }),
		activity: jsonResponse({ items: [], hasMore: false }),
		...overrides
	};

	apiFetchWithSession.mockImplementation((path: string) => {
		const key = Object.keys(answers).find((name) =>
			name === 'connect' ? path.includes('/payments/connect') : path.includes(`/${name}`)
		);
		return Promise.resolve(answers[key!]);
	});

	await render(Page, {});
}

beforeEach(() => {
	apiFetch.mockReset();
	apiFetchWithSession.mockReset();
	registerPushSubscription.mockReset();
	registerPushSubscription.mockResolvedValue(undefined);
});

describe('the Practice landing page', () => {
	it('leads with the Offers still awaiting an answer, and drops the decided ones', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('heading', { name: 'Offers awaiting your answer' }))
			.toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Accept' })).toBeVisible();
		expect(testPage.getByText('Declined').elements()).toHaveLength(0);
	});

	// #455: the roll-up of Engagements whose thread's latest Message came
	// from the Client, in the primary column alongside Offers -- "who is
	// waiting on me" is the same question, for any Staff role.
	it('shows the Engagements waiting on a reply, linking through to each one', async () => {
		await setup({
			overrides: {
				'awaiting-reply': jsonResponse({
					items: [{ engagementId: 'engagement-9', clientName: 'Priya', lastMessageAt: new Date().toISOString() }],
					hasMore: false
				})
			}
		});

		await expect
			.element(testPage.getByRole('heading', { name: 'Clients waiting on a reply' }))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Priya' }))
			.toHaveAttribute('href', `/practices/${practiceId}/engagements/engagement-9`);
	});

	it('tells a doula nobody is waiting on a reply, rather than drawing an empty block', async () => {
		// DataTable's table view needs a frame wider than its content floor
		// (48.75rem) to render, same reason the activity-feed empty-state
		// test below sets this -- otherwise the narrow record view's own
		// <p>{emptyMessage}</p> and the table's <td> both match a bare text
		// query.
		await testPage.viewport(1440, 900);
		await setup();

		await expect
			.element(testPage.getByRole('cell', { name: 'Nobody is waiting on a reply.' }))
			.toBeVisible();
	});

	it('shows the roster, the credit balance and the Stripe state to an Owner', async () => {
		await setup();

		await expect.element(testPage.getByRole('heading', { name: 'Your people' })).toBeVisible();
		await expect.element(testPage.getByText('1 invitation expired')).toBeVisible();
		await expect.element(testPage.getByRole('heading', { name: 'Credits' })).toBeVisible();
		await expect.element(testPage.getByText('12')).toBeVisible();
		await expect.element(testPage.getByText('Onboarding incomplete')).toBeVisible();
		await expect
			.element(testPage.getByText('Stripe is waiting on 1 more detail.'))
			.toBeVisible();
	});

	// #503: a pending Request stops a Doula from working, so the hub whose
	// question is "what needs me today" counts them and hands off to the
	// inbox, where the decision is actually made.
	it('counts the Requests waiting on a decision and points at the inbox', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('heading', { name: 'Requests awaiting approval' }))
			.toBeVisible();
		await expect.element(testPage.getByText('2 waiting')).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Review requests' }))
			.toHaveAttribute('href', `/practices/${practiceId}/engagement-requests`);
	});

	it('reads a full page of waiting Requests as a floor, not an exact count', async () => {
		await setup({
			overrides: {
				'engagement-requests': jsonResponse({ items: [{ requestId: 'request-1' }], hasMore: true })
			}
		});

		await expect.element(testPage.getByText('1+ waiting')).toBeVisible();
	});

	it('tells an approver nobody is waiting rather than drawing an empty block', async () => {
		await setup({ overrides: { 'engagement-requests': jsonResponse({ items: [], hasMore: false }) } });

		await expect.element(testPage.getByText('Nobody is waiting on you.')).toBeVisible();
	});

	it('says so when the pending Requests cannot be read', async () => {
		await setup({ overrides: { 'engagement-requests': refusal('nope') } });

		await expect.element(testPage.getByText('Could not load pending requests just now.')).toBeVisible();
	});

	it('shows an Admin no Stripe block, because that endpoint would refuse her', async () => {
		await setup({ roles: ['admin'] });

		await expect.element(testPage.getByRole('heading', { name: 'Your people' })).toBeVisible();
		expect(testPage.getByRole('heading', { name: 'Getting paid' }).elements()).toHaveLength(0);
	});

	it('shows a Doula her Offers and no rail at all', async () => {
		// A Doula is exactly the fixture's own happy path (see the comment
		// above `pageState`), so this is the one test in the file that reads
		// its content straight from the fixture instead of `setup()`'s own.
		apiFetchWithSession.mockImplementation(toApiResponder(fixture));
		await render(Page, {});

		await expect.element(testPage.getByRole('button', { name: 'Accept' })).toBeVisible();
		expect(testPage.getByRole('heading', { name: 'Your people' }).elements()).toHaveLength(0);
		expect(testPage.getByRole('heading', { name: 'Credits' }).elements()).toHaveLength(0);
	});

	it('says so when a rail block fails, rather than letting it vanish', async () => {
		await setup({ overrides: { billing: refusal('nope') } });

		await expect.element(testPage.getByText('Could not load your credit balance just now.')).toBeVisible();
	});

	it('names doula work and offers one action on a Practice with no Clients', async () => {
		await setup({ clients: [] });

		await expect
			.element(testPage.getByText(/the Client's birth plan, your visits to the Client/))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Add your first Client' }))
			.toBeVisible();
		// The abandon point was a menu of administration. One action, not eight.
		expect(testPage.getByRole('link').elements()).toHaveLength(1);
	});

	it('reserves the page while it loads, rather than flashing empty', async () => {
		apiFetchWithSession.mockReturnValue(new Promise(() => {}));
		await render(Page, {});

		await expect
			.element(testPage.getByRole('status', { name: 'Loading your Practice' }))
			.toBeVisible();
	});

	it('surfaces a failure of the critical path in place', async () => {
		await setup({ overrides: { session: refusal('your session has expired') } });

		await expect.element(testPage.getByText('your session has expired')).toBeVisible();
	});

	// #486 AC1/AC3: the practice-wide feed, low on the page, rendered
	// through OverviewHub's own `feed` slot with the ledger's three
	// columns -- when, what, who.
	it('renders the practice-wide activity feed low on the page', async () => {
		// DataTable's own table view needs a frame wider than its content
		// floor (48.75rem, DataTable.svelte) or it renders the narrow
		// record view instead, whose cells carry no `cell` role -- see
		// DataTable.svelte.spec.ts's own WIDE viewport for the same reason.
		await testPage.viewport(1440, 900);
		await setup({
			overrides: {
				activity: jsonResponse({
					items: [
						{
							subjectKind: 'engagement',
							subjectId: 'e1',
							action: 'invoice_raised',
							actorKind: 'staff',
							actorName: 'Mark Goho',
							createdAt: new Date().toISOString()
						}
					],
					hasMore: false
				})
			}
		});

		await expect.element(testPage.getByRole('heading', { name: 'Recent activity' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Invoice raised' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Mark Goho' })).toBeVisible();
	});

	it('says so when the activity feed cannot be read', async () => {
		await setup({ overrides: { activity: refusal('nope') } });

		await expect.element(testPage.getByText('nope')).toBeVisible();
	});

	it('shows the empty-feed message when there is no activity yet', async () => {
		await testPage.viewport(1440, 900);
		await setup();

		await expect.element(testPage.getByRole('cell', { name: 'Nothing has happened yet.' })).toBeVisible();
	});

	it('still registers this device for push once it has landed', async () => {
		await setup();

		await expect.element(testPage.getByRole('heading', { name: 'Credits' })).toBeVisible();
		expect(registerPushSubscription).toHaveBeenCalledWith(
			`/api/practices/${practiceId}/push-subscriptions`,
			apiFetch
		);
	});
});
