import { createRawSnippet } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { SignOutOutcome } from '#lib/signOut.js';
import { jsonResponse } from '#lib/testResponse.js';
import Layout from './+layout.svelte';

// The nav marks the current section off the pathname, so the URL moves per
// test the way the route parameters do.
const pageState = vi.hoisted(() => ({
	params: { engagementId: 'engagement-1' },
	url: new URL('http://localhost/portal/engagements/engagement-1')
}));
vi.mock('$app/state', () => ({ page: pageState }));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const signOutOfSession = vi.hoisted(() => vi.fn<() => Promise<SignOutOutcome>>());
vi.mock('#lib/signOut.js', () => ({ signOutOfSession }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
const apiFetch = vi.hoisted(() => vi.fn());
const apiBaseURL = vi.hoisted(() => vi.fn(() => ''));
vi.mock('#lib/api.js', () => ({ apiFetch, apiFetchWithSession, apiBaseURL }));

// Push registration moved up here from the hub page, so a Client who lands
// straight on her Contract is registered too. Mocked rather than exercised:
// it is fire-and-forget by design (#61).
const registerPushSubscription = vi.hoisted(() => vi.fn());
vi.mock('#lib/pushRegistration.js', () => ({
	registerPushSubscription,
	unregisterPushSubscription: vi.fn(),
	portalPushSubscriptionsPath: (engagementId: string) =>
		`/api/portal/engagements/${engagementId}/push-subscriptions`
}));

interface SetupOptions {
	outcome?: SignOutOutcome;
	pathname?: string;
	detailRefuses?: boolean;
}

async function setup({
	outcome = { ok: true },
	pathname = '/portal/engagements/engagement-1',
	detailRefuses = false
}: SetupOptions = {}) {
	pageState.url = new URL(`http://localhost${pathname}`);
	goto.mockReset();
	registerPushSubscription.mockReset();
	signOutOfSession.mockReset();
	signOutOfSession.mockResolvedValue(outcome);
	apiFetchWithSession.mockReset();
	apiFetchWithSession.mockResolvedValue(
		jsonResponse(
			{ practiceName: 'Riverside Doula Collective', clientName: 'Tasha Bell' },
			detailRefuses ? 403 : 200
		)
	);
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
	 * Messages is its own destination now (#452). Before this it rendered
	 * inside the hub, and a nav item pointing at a section of another page
	 * is a nav item that lies about where it goes.
	 */
	it.each(['Your care', 'Messages', 'Birth plan', 'Contract'])('offers %s', async (label) => {
		await setup();

		await expect.element(page.getByRole('link', { name: label }).first()).toBeVisible();
	});

	it('registers for push once, wherever the Client happens to land', async () => {
		await setup({ pathname: '/portal/engagements/engagement-1/contract' });

		expect(registerPushSubscription).toHaveBeenCalledWith(
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
	 * Practice's name arrives and the page below never moves.
	 */
	it('still draws the bar when the Engagement read refuses', async () => {
		await setup({ detailRefuses: true });

		await expect.element(page.getByRole('banner')).toBeVisible();
		await expect.element(page.getByRole('button', { name: /Your account/ })).not.toBeInTheDocument();
	});
});
