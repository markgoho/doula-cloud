import { createRawSnippet } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { SignOutOutcome } from '#lib/signOut.js';
import Layout from './+layout.svelte';

// The nav marks the current section off the pathname, so the URL moves per
// test the way the route parameters do. `data` stands in for
// `engagements/[engagementId]/+layout.ts`'s load result (#487): the layout
// now reads the Practice's identity from `page.data` rather than fetching
// it itself, so a refusal there is the load's own responsibility, not
// this component's -- see `still draws the bar when the Practice's
// identity is not yet known` below for what this component still owns.
const pageState = vi.hoisted(() => ({
	params: { engagementId: 'engagement-1' },
	url: new URL('http://localhost/portal/engagements/engagement-1'),
	data: {} as {
		practiceName?: string;
		clientName?: string;
		createdAt?: string;
		offersBirthPlan?: boolean;
	}
}));
vi.mock('$app/state', () => ({ page: pageState }));

const goto = vi.hoisted(() => vi.fn());
const invalidateAll = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto, invalidateAll }));

const signOutOfSession = vi.hoisted(() => vi.fn<() => Promise<SignOutOutcome>>());
vi.mock('#lib/signOut.js', () => ({ signOutOfSession }));

const apiFetch = vi.hoisted(() => vi.fn());
const apiBaseURL = vi.hoisted(() => vi.fn(() => ''));
vi.mock('#lib/api.js', () => ({ apiFetch, apiBaseURL }));

// Push registration moved up here from the hub page, so a Client who lands
// straight on her Contract is registered too. Mocked rather than exercised:
// it is fire-and-forget by design (#61), and #303 gates it on the stored
// preference through registerPushSubscriptionIfEnabled.
const registerPushSubscriptionIfEnabled = vi.hoisted(() => vi.fn());
vi.mock('#lib/pushRegistration.js', () => ({
	registerPushSubscriptionIfEnabled,
	unregisterPushSubscription: vi.fn(),
	portalPushSubscriptionsPath: (engagementId: string) =>
		`/api/portal/engagements/${engagementId}/push-subscriptions`,
	portalNotificationPreferencePath: (engagementId: string) =>
		`/api/portal/engagements/${engagementId}/notification-preference`
}));

interface SetupOptions {
	outcome?: SignOutOutcome;
	pathname?: string;
	/**
	 * The Practice's identity not yet known -- either the load hasn't
	 * resolved yet, or (before #487) the read had refused. Either way this
	 * component's own job is unchanged: draw the bar regardless.
	 */
	identityUnknown?: boolean;
	/** #311: whether the Engagement calls for a Birth Plan -- true by
	 * default so every existing test here keeps seeing the same five nav
	 * items it always has. */
	offersBirthPlan?: boolean;
}

async function setup({
	outcome = { ok: true },
	pathname = '/portal/engagements/engagement-1',
	identityUnknown = false,
	offersBirthPlan = true
}: SetupOptions = {}) {
	pageState.url = new URL(`http://localhost${pathname}`);
	pageState.data = identityUnknown
		? {}
		: {
				practiceName: 'Riverside Doula Collective',
				clientName: 'Tasha Bell',
				createdAt: '2026-03-12T20:00:00Z',
				offersBirthPlan
			};
	goto.mockReset();
	invalidateAll.mockReset();
	registerPushSubscriptionIfEnabled.mockReset();
	signOutOfSession.mockReset();
	signOutOfSession.mockResolvedValue(outcome);
	await render(Layout, {
		children: createRawSnippet(() => ({ render: () => '<p>portal child content</p>' }))
	});
}

async function openAvatarMenu() {
	await page.getByRole('button', { name: 'Your account, Tasha Bell' }).first().click();
	return page.getByRole('button', { name: 'Sign out' }).first();
}

describe('Client portal authenticated layout', () => {
	it('renders its children', async () => {
		await setup();

		await expect.element(page.getByText('portal child content')).toBeVisible();
	});

	it('gives the page a main landmark and a way to skip to it', async () => {
		await setup();

		await expect.element(page.getByRole('main')).toBeVisible();
		await expect
			.element(page.getByRole('link', { name: 'Skip to main content' }))
			.toHaveAttribute('href', '#main');
	});

	it('names the Practice, which is the portal identity', async () => {
		await setup();

		await expect.element(page.getByText('Riverside Doula Collective')).toBeVisible();
	});

	/*
	 * #310: reachable from every authenticated screen, not only the hub --
	 * this spec's own `setup` renders the layout at
	 * /portal/engagements/engagement-1/contract by default, and the link
	 * is still there. Its accessible name is engagementLabel's own words
	 * (built from the same `createdAt` `+layout.ts` now loads), so it
	 * reads the same as the root list's and the choosers' link for the
	 * same Engagement.
	 */
	it('offers a persistent way to the portal root, named the way the root list names it', async () => {
		await setup();

		const link = page.getByRole('link', { name: 'Riverside Doula Collective, started Mar 12, 2026' });
		await expect.element(link).toBeVisible();
		expect(link.element()).toHaveAttribute('href', '/');
	});

	/*
	 * Messages is its own destination now (#452). Before this it rendered
	 * inside the hub, and a nav item pointing at a section of another page
	 * is a nav item that lies about where it goes.
	 */
	it.each(['Your care', 'Messages', 'Birth plan', 'Contract', 'Notifications'])('offers %s', async (label) => {
		await setup();

		await expect.element(page.getByRole('link', { name: label }).first()).toBeVisible();
	});

	// #311: a postpartum-only Engagement offers no Birth Plan anywhere in
	// the portal -- this persistent nav item included, with no gap left in
	// its place.
	it('offers no Birth plan nav item when the Engagement does not call for one', async () => {
		await setup({ offersBirthPlan: false });

		await expect.element(page.getByRole('link', { name: 'Your care' }).first()).toBeVisible();
		await expect.element(page.getByRole('link', { name: 'Contract' }).first()).toBeVisible();
		await expect.element(page.getByRole('link', { name: 'Birth plan' })).not.toBeInTheDocument();
	});

	/*
	 * #619's screen is reached from the account menu, not the nav row. The
	 * nav names the care she is receiving; where she signs in from is not
	 * one of those, and the bar's measured content floor is what four
	 * destinations beside the Practice's name cost. This asserts it is
	 * reachable at all, which is the half a route with no affordance fails.
	 */
	it('offers her own account behind the menu, not as a sixth destination', async () => {
		await setup();

		await page.getByRole('button', { name: 'Your account, Tasha Bell' }).first().click();
		const account = page.getByRole('link', { name: 'Account' }).first();
		await expect.element(account).toBeVisible();
		await expect
			.element(account)
			.toHaveAttribute('href', '/portal/engagements/engagement-1/sign-in-address');
	});

	it('consults the stored push preference once, wherever the Client happens to land', async () => {
		await setup({ pathname: '/portal/engagements/engagement-1/contract' });

		expect(registerPushSubscriptionIfEnabled).toHaveBeenCalledWith(
			'/api/portal/engagements/engagement-1/notification-preference',
			'/api/portal/engagements/engagement-1/push-subscriptions',
			expect.any(Function)
		);
	});

	it('signs out of the current Engagement and lands on the portal login screen', async () => {
		await setup();

		const signOutButton = await openAvatarMenu();
		await signOutButton.click();

		expect(signOutOfSession).toHaveBeenCalledWith(
			expect.objectContaining({
				unsubscribeURL: '/api/portal/engagements/engagement-1/push-subscriptions'
			})
		);
		// The portal door, not the Staff one -- a Client sent to /login
		// would be looking at a screen that is not theirs.
		expect(goto).toHaveBeenCalledWith('/portal/login');
		// Otherwise a Back press to the Engagement URL would reuse the
		// still-signed-in load result instead of re-checking the session
		// (#487).
		expect(invalidateAll).toHaveBeenCalled();
	});

	it('stays put and reports a sign-out that failed', async () => {
		await setup({ outcome: { ok: false, message: 'Sign-out failed.' } });

		const signOutButton = await openAvatarMenu();
		await signOutButton.click();

		await expect.element(page.getByRole('alert')).toHaveTextContent('Sign-out failed.');
		expect(goto).not.toHaveBeenCalled();
	});

	/*
	 * The bar is a fixed height whatever it holds, so it paints before the
	 * Practice's identity arrives and the page below never moves. Since
	 * #487, that identity is preloaded (`engagements/[engagementId]/
	 * +layout.ts`) rather than fetched by this component, so a refusal
	 * there is the load's own responsibility (`redirect`/`error`) --
	 * this component only has to stay resilient to `page.data` not
	 * carrying the identity yet.
	 */
	it("still draws the bar when the Practice's identity is not yet known", async () => {
		await setup({ identityUnknown: true });

		await expect.element(page.getByRole('banner')).toBeVisible();
		await expect.element(page.getByRole('button', { name: /Your account/ })).not.toBeInTheDocument();
	});
});
