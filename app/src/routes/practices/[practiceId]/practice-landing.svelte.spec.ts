import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import { jsonResponse } from '#lib/testResponse.js';
import { CONNECT_STATUS_BADGES } from '#lib/payments.js';
import Page from './+page.svelte';
// The Skeleton reserves a line of body copy with `var(--text-body-size)`,
// so without the tokens it draws at zero height and reserves nothing --
// the very thing ADR-0020 asks it to do. The real app loads these in the
// root layout.
import '#lib/styles/app.css';
import { toApiResponder, toPageState } from '../../routeFixture.js';
import { asOwner, asOwnerWithNoClient, fixture, offers, practiceName } from './page.fixture.js';

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
 * `route-continuum.svelte.spec.ts`. `pageData.session` carries the
 * Membership `practices/[practiceId]/+layout.ts` resolves (#835); the
 * fixture's own `respond` answers the Owner's four `secondary` blocks as
 * well as what a Doula fetches, since #928 gave this route an Owner
 * variant and `block()` swallows an unanswered path into a "Could not
 * load" rail. The variant for an Owner with no Client (#1621) has a
 * `respond` of its own, which answers `/clients` with no items and hands
 * each other path to that same function, so those answers are still
 * written once. `setup()` below still installs its own answers rather than
 * the fixture's, because most of these tests are about a *particular*
 * one of those blocks failing or arriving empty -- content that is not
 * the happy path, which svelte-tests.md leaves to the spec (#596).
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
	apiFetchWithSession
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
	// The Membership (roles, practiceName) comes off page.data.session
	// (#835), not a fetch this mock has to answer.
	pageState.data = { session: { practiceId, practiceName, roles, isContractor: false } };
	const answers: Record<string, Response> = {
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
		// Before `billing`: its path holds `/billing` (#1589).
		'billing-mode': jsonResponse({}),
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
	// setup() overwrites pageState.data per test; reset it back to the
	// fixture's own Doula session (#835) for the one test that bypasses
	// setup() and reads straight off the fixture.
	Object.assign(pageState, toPageState(fixture));
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

	// #267 gave Stripe Connect state to the Admin as well as the Owner --
	// ADR-0008's read table puts it beside the Invoice history it is the
	// payment rail for -- so her Overview carries the block the endpoint
	// now answers for her.
	it('shows an Admin the Stripe block, the same as the Owner', async () => {
		await setup({ roles: ['admin'] });

		await expect.element(testPage.getByRole('heading', { name: 'Your people' })).toBeVisible();
		await expect.element(testPage.getByRole('heading', { name: 'Getting paid' })).toBeVisible();
	});

	/*
	 * #1589: the Getting paid card is the first moment the product asks an
	 * Owner to connect Stripe (#1495). The words are the decided ones.
	 */
	describe('the Getting paid card', () => {
		const notConnected = jsonResponse({
			status: 'not_connected',
			cardPaymentsStatus: 'inactive',
			payoutsStatus: 'inactive',
			requirementsDue: []
		});
		const ownerWords =
			'Clients cannot pay you by card yet. To take card payments, connect Stripe. It takes about fifteen minutes, and then Stripe reviews your details. If you collect payment yourself, you do not need Stripe.';
		const ownerWordsWithStripe =
			'Clients cannot pay you by card yet. To take card payments, connect Stripe. It takes about fifteen minutes, and then Stripe reviews your details.';
		const adminWords = 'Clients cannot pay this Practice by card yet. A Practice Owner has to connect Stripe.';

		function setupCard(overrides: Record<string, Response>, roles = ['owner']) {
			return setup({ roles, overrides: { connect: notConnected, ...overrides } });
		}

		it('asks an Owner with no Stripe account and no billing mode to set up card payments', async () => {
			await setupCard({});

			await expect.element(testPage.getByText(ownerWords, { exact: true })).toBeVisible();
			await expect
				.element(testPage.getByRole('link', { name: 'Set up card payments' }))
				.toHaveAttribute('href', `/practices/${practiceId}/settings/payments`);
			expect(testPage.getByText('Not connected').elements()).toHaveLength(0);
		});

		it('leaves out the sentence about collecting payment herself once the mode is Stripe', async () => {
			await setupCard({ 'billing-mode': jsonResponse({ billingMode: 'stripe' }) });

			await expect.element(testPage.getByText(ownerWordsWithStripe, { exact: true })).toBeVisible();
			expect(testPage.getByText(/collect payment yourself/).elements()).toHaveLength(0);
		});

		it('tells an Admin, who cannot connect Stripe, who can', async () => {
			await setupCard({}, ['admin']);

			await expect.element(testPage.getByText(adminWords, { exact: true })).toBeVisible();
			await expect
				.element(testPage.getByRole('link', { name: 'Getting paid' }))
				.toHaveAttribute('href', `/practices/${practiceId}/settings/payments`);
			expect(testPage.getByText('Not connected').elements()).toHaveLength(0);
		});

		it('is shown when the billing mode cannot be loaded', async () => {
			await setupCard({ 'billing-mode': refusal('nope') });

			await expect.element(testPage.getByText(ownerWords, { exact: true })).toBeVisible();
		});

		it.each(['not_connected', 'onboarding_incomplete', 'pending', 'payouts_restricted', 'active'])(
			'is not drawn for a Practice that bills by hand, whatever Stripe says (%s)',
			async (status) => {
				await setup({
					overrides: {
						'billing-mode': jsonResponse({ billingMode: 'by_hand' }),
						connect: jsonResponse({
							status,
							cardPaymentsStatus: 'inactive',
							payoutsStatus: 'inactive',
							requirementsDue: []
						})
					}
				});

				// The rail has drawn: its other blocks are the proof the page is
				// loaded, so the absence below is not just a slow render.
				await expect.element(testPage.getByRole('heading', { name: 'Credits' })).toBeVisible();
				expect(testPage.getByRole('heading', { name: 'Getting paid' }).elements()).toHaveLength(0);
			}
		);

		it.each(['onboarding_incomplete', 'pending', 'payouts_restricted', 'active'] as const)(
			'keeps its Badge for %s, in the words the Getting paid screen shares',
			async (status) => {
				await setup({
					overrides: {
						connect: jsonResponse({
							status,
							cardPaymentsStatus: 'inactive',
							payoutsStatus: 'inactive',
							requirementsDue: []
						})
					}
				});

				await expect
					.element(testPage.getByText(CONNECT_STATUS_BADGES[status].label, { exact: true }))
					.toBeVisible();
			}
		);

		it('draws no Getting paid card on the empty Practice', async () => {
			await setup({ clients: [], overrides: { connect: notConnected } });

			await expect.element(testPage.getByRole('link', { name: 'Add your first Client' })).toBeVisible();
			expect(testPage.getByRole('heading', { name: 'Getting paid' }).elements()).toHaveLength(0);
		});
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

	/*
	 * The other half of the same pair, and the one guard the continuum
	 * sweep cannot be (#928). The sweep mounts `asOwner` and measures it,
	 * but `block()` in `practiceLanding.ts` swallows a thrown `respond`
	 * into "Could not load ..." -- so a fixture that stopped answering one
	 * of the Owner's four reads would draw a smaller rail and stay green
	 * there. This asserts the fixture's own answers actually fill it.
	 */
	it('fills the whole rail for an Owner from the fixture that sweeps her', async () => {
		Object.assign(pageState, toPageState({ ...fixture, ...asOwner }));
		apiFetchWithSession.mockImplementation(toApiResponder(fixture));
		await render(Page, {});

		await expect.element(testPage.getByRole('heading', { name: 'Your people' })).toBeVisible();
		await expect.element(testPage.getByText('4 invitations expired')).toBeVisible();
		await expect.element(testPage.getByRole('heading', { name: 'Credits' })).toBeVisible();
		await expect.element(testPage.getByText('1284')).toBeVisible();
		await expect.element(testPage.getByText('30+ waiting')).toBeVisible();
		await expect.element(testPage.getByText('Onboarding incomplete')).toBeVisible();
		await expect
			.element(testPage.getByText('Stripe is waiting on 3 more details.'))
			.toBeVisible();
	});

	/*
	 * The third subject, and the same guard for it (#1621). The sweep mounts
	 * `asOwnerWithNoClient` and measures it, but it cannot tell an empty
	 * Practice from a populated one: the two fit, so a variant that drew
	 * the populated hub under the empty Practice's name would stay green
	 * there. This asserts which tree the fixture's own answers draw.
	 *
	 * The realized fixture goes to `toApiResponder` as well as to
	 * `toPageState`. The Owner test above hands `fixture` to
	 * `toApiResponder`, which is correct there because `asOwner` inherits
	 * `respond`. This variant restates it, so the base's `respond` here
	 * would answer `/clients` with a Client and mount the populated hub.
	 */
	it('draws the empty Practice for an Owner with no Client from the fixture that sweeps her', async () => {
		const realized = { ...fixture, ...asOwnerWithNoClient };
		Object.assign(pageState, toPageState(realized));
		apiFetchWithSession.mockImplementation(toApiResponder(realized));
		await render(Page, {});

		await expect
			.element(testPage.getByRole('heading', { level: 1, name: `Welcome to ${practiceName}` }))
			.toBeVisible();
		await expect
			.element(
				testPage.getByText(
					"Nothing is here yet, because no Client is. Add one and this becomes the Client's birth plan, the visits with the Client, and the contract and invoices between the Client and your Practice.",
					{ exact: true }
				)
			)
			.toBeVisible();
		// #1612: the fixture's Practice holds the signup bonus alone, so the
		// sentence is in its "Welcome credits" form, the longer of the two.
		await expect
			.element(
				testPage.getByText(
					'Adding a Client is free. Starting work with a Client uses 1 Credit, and this Practice has 3 Welcome credits.',
					{ exact: true }
				)
			)
			.toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Add your first Client' }))
			.toBeVisible();
		expect(testPage.getByRole('heading', { name: 'Offers awaiting your answer' }).elements()).toHaveLength(0);
		expect(testPage.getByRole('heading', { name: 'Your people' }).elements()).toHaveLength(0);
	});

	it('says so when a rail block fails, rather than letting it vanish', async () => {
		await setup({ overrides: { billing: refusal('nope') } });

		await expect.element(testPage.getByText('Could not load your credit balance just now.')).toBeVisible();
	});

	it('names doula work and offers one action on a Practice with no Clients', async () => {
		await setup({ clients: [] });

		// #1599: the whole sentence, not a fragment of it. Signup gives a
		// solo Owner and an agency Owner the same three roles and nothing
		// asks her which she is before this screen, so the words must be
		// true for the two of them -- "your visits" was not, for an Owner
		// who attends no births.
		await expect
			.element(
				testPage.getByText(
					"Nothing is here yet, because no Client is. Add one and this becomes the Client's birth plan, the visits with the Client, and the contract and invoices between the Client and your Practice.",
					{ exact: true }
				)
			)
			.toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Add your first Client' }))
			.toBeVisible();
		// #1609: a Practice with no Client record has nobody to search for,
		// so the one action opens the name question, and its Back comes here.
		await expect
			.element(testPage.getByRole('link', { name: 'Add your first Client' }))
			.toHaveAttribute('href', `/practices/${practiceId}/clients/new?from=overview`);
		// The abandon point was a menu of administration. One action, not
		// eight (ADR-0048: the empty Practice asks for one act and nothing
		// else) -- and an action is a link or a button, so both are counted.
		expect(testPage.getByRole('link').elements()).toHaveLength(1);
		expect(testPage.getByRole('button').elements()).toHaveLength(0);
	});

	/*
	 * #1612: what uses a Credit, and how many the Practice has, said at the
	 * start point -- its own paragraph below the words and before the one
	 * link. The balance is the one on the ledger at that moment.
	 */
	const costSentence = 'Adding a Client is free. Starting work with a Client uses 1 Credit, and this Practice has';
	const noNumberSentence = "Adding a Client is free. Starting work with a Client uses 1 of the Practice's Credits.";

	it('tells an Owner on an empty Practice what uses a Credit, and how many the Practice has', async () => {
		// `setup()`'s balance has no signup bonus on its ledger, so it reads
		// "N Credits", the form for a founding grant or a purchase.
		await setup({ clients: [] });

		const sentence = testPage.getByText(`${costSentence} 12 Credits.`, { exact: true });
		await expect.element(sentence).toBeVisible();
		// Its own paragraph, after the words and before the link, and not
		// a live region: it is a fact on the page, not an announcement.
		// The DOM is read here because the order of siblings and the absence
		// of a live region have no accessible query of their own.
		const paragraph = sentence.element();
		expect(paragraph.tagName).toBe('P');
		expect(paragraph.previousElementSibling?.textContent).toContain('Nothing is here yet');
		expect(paragraph.nextElementSibling?.textContent?.trim()).toBe('Add your first Client');
		expect(paragraph.closest('[aria-live], [role="status"], [role="alert"]')).toBeNull();
		// No price: the Credits screen is the one place that says one (#285).
		expect(testPage.getByText(/costs|\$/).elements()).toHaveLength(0);
	});

	it('gives an Admin the same sentence', async () => {
		await setup({ roles: ['admin'], clients: [] });

		await expect.element(testPage.getByText(`${costSentence} 12 Credits.`, { exact: true })).toBeVisible();
	});

	it('says one Credit in the singular', async () => {
		await setup({
			clients: [],
			overrides: { billing: jsonResponse({ balance: 1, ledger: { items: [], hasMore: false } }) }
		});

		await expect.element(testPage.getByText(`${costSentence} 1 Credit.`, { exact: true })).toBeVisible();
	});

	it('gives a Doula, who cannot read the balance, the sentence with no number', async () => {
		await setup({ roles: ['doula'], clients: [] });

		await expect.element(testPage.getByText(noNumberSentence, { exact: true })).toBeVisible();
		expect(testPage.getByText(costSentence).elements()).toHaveLength(0);
	});

	it('leaves the balance out, and the link working, where the balance cannot be loaded', async () => {
		await setup({ clients: [], overrides: { billing: refusal('nope') } });

		await expect.element(testPage.getByText(noNumberSentence, { exact: true })).toBeVisible();
		expect(testPage.getByText(costSentence).elements()).toHaveLength(0);
		// The empty Practice keeps one action, and says nothing of a failed
		// read: the rail that would say so is not drawn here.
		expect(testPage.getByText('Could not load your credit balance just now.').elements()).toHaveLength(0);
		await expect
			.element(testPage.getByRole('link', { name: 'Add your first Client' }))
			.toHaveAttribute('href', `/practices/${practiceId}/clients/new?from=overview`);
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
		await setup({ overrides: { offers: refusal('your session has expired') } });

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
