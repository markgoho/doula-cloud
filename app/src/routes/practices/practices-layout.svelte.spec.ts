import { createRawSnippet } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { SignOutOutcome } from '#lib/signOut.js';
import { jsonResponse } from '#lib/testResponse.js';
import Layout from './+layout.svelte';

// Mutable rather than a fixed literal: the layout skips the push
// unregister on a screen with no Practice in its route, and the nav marks
// the current section off the pathname, so both need to move per test.
const pageState = vi.hoisted(() => ({
	params: {} as { practiceId?: string },
	url: new URL('http://localhost/practices/practice-1')
}));
vi.mock('$app/state', () => ({ page: pageState }));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const signOutOfSession = vi.hoisted(() => vi.fn<() => Promise<SignOutOutcome>>());
vi.mock('#lib/signOut.js', () => ({ signOutOfSession }));

// Two reads: `/api/staff/session` for the person behind the avatar menu
// and the Memberships behind the Practice switcher, and the Practice's own
// session for the roles that decide how many nav items there are.
const apiFetchWithSession = vi.hoisted(() => vi.fn());
const apiFetch = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetch, apiFetchWithSession }));

const ONE_MEMBERSHIP = [
	{ practiceId: 'practice-1', practiceName: 'Riverside Doula Collective', roles: ['owner'] }
];

interface SetupOptions {
	outcome?: SignOutOutcome;
	routeParameters?: { practiceId?: string };
	pathname?: string;
	roles?: string[];
	sessionRefuses?: boolean;
	staffSessionRefuses?: boolean;
	memberships?: { practiceId: string; practiceName: string; roles: string[] }[];
}

async function setup({
	outcome = { ok: true },
	routeParameters = { practiceId: 'practice-1' },
	pathname = '/practices/practice-1',
	roles = ['owner'],
	sessionRefuses = false,
	staffSessionRefuses = false,
	memberships = ONE_MEMBERSHIP
}: SetupOptions = {}) {
	// Pinned wide: the bar keeps both trees in the document with one
	// display:none, so which nav is reachable is a fact about the viewport.
	// The narrow sheet is StaffTopBar's own spec's business.
	await page.viewport(1440, 900);
	pageState.params = routeParameters;
	pageState.url = new URL(`http://localhost${pathname}`);
	goto.mockReset();
	signOutOfSession.mockReset();
	signOutOfSession.mockResolvedValue(outcome);
	apiFetchWithSession.mockReset();
	apiFetchWithSession.mockImplementation((path: string) =>
		Promise.resolve(
			path === '/api/staff/session'
				? jsonResponse(
						{ name: 'Mark Goho', email: 'mark@example.test', memberships },
						staffSessionRefuses ? 403 : 200
					)
				: jsonResponse({ roles }, sessionRefuses ? 403 : 200)
		)
	);
	await render(Layout, {
		children: createRawSnippet(() => ({ render: () => '<p>staff child content</p>' }))
	});
	return { avatar: page.getByRole('button', { name: 'Your account, Mark Goho' }) };
}

async function openAvatarMenu() {
	await page.getByRole('button', { name: 'Your account, Mark Goho' }).click();
	return page.getByRole('button', { name: 'Sign out' });
}

describe('Staff authenticated layout', () => {
	it('renders its children', async () => {
		await setup();

		await expect.element(page.getByText('staff child content')).toBeVisible();
	});

	it('gives the page a main landmark for the skip link to target', async () => {
		await setup();

		await expect.element(page.getByRole('main')).toBeVisible();
	});

	it('puts a skip link ahead of the nav, because six items is a bypass block', async () => {
		await setup();

		await expect
			.element(page.getByRole('link', { name: 'Skip to main content' }))
			.toHaveAttribute('href', '#main');
	});

	it('signs out of the current Practice and lands on the Staff login screen', async () => {
		await setup();

		const signOutButton = await openAvatarMenu();
		await signOutButton.click();

		expect(signOutOfSession).toHaveBeenCalledWith(
			expect.objectContaining({
				unsubscribeURL: '/api/practices/practice-1/push-subscriptions'
			})
		);
		expect(goto).toHaveBeenCalledWith('/login');
	});

	it('signs out without an unregister when the route carries no Practice', async () => {
		await setup({ routeParameters: {}, pathname: '/account' });

		const signOutButton = await openAvatarMenu();
		await signOutButton.click();

		expect(signOutOfSession).toHaveBeenCalledWith(
			expect.objectContaining({ unsubscribeURL: undefined })
		);
		expect(goto).toHaveBeenCalledWith('/login');
	});

	it('stays put and reports a sign-out that failed', async () => {
		await setup({ outcome: { ok: false, message: 'Sign-out failed.' } });

		const signOutButton = await openAvatarMenu();
		await signOutButton.click();

		await expect.element(page.getByRole('alert')).toHaveTextContent('Sign-out failed.');
		expect(goto).not.toHaveBeenCalled();
	});
});

describe('the nav', () => {
	it.each(['Overview', 'Clients', 'Billing', 'Staff', 'Offers', 'Settings'])(
		'offers %s to an Owner',
		async (label) => {
			await setup({ roles: ['owner'] });

			await expect.element(page.getByRole('link', { name: label, exact: true })).toBeVisible();
		}
	);

	/*
	 * The drawing (#431) shows an Owner's bar. `GET .../billing` and
	 * `GET .../staff` are both `ownerAndAdmin` on the BFF, so offering a
	 * Doula those two would be a promise the endpoint refuses -- the same
	 * rule #423 applied to the landing page's rail.
	 */
	it.each(['Billing', 'Staff'])('hides %s from a Doula', async (label) => {
		await setup({ roles: ['doula'] });

		await expect.element(page.getByRole('link', { name: label, exact: true })).not.toBeInTheDocument();
	});

	it.each(['Overview', 'Clients', 'Offers', 'Settings'])(
		'still offers %s to a Doula',
		async (label) => {
			await setup({ roles: ['doula'] });

			await expect.element(page.getByRole('link', { name: label, exact: true })).toBeVisible();
		}
	);

	it('keeps the admin-only items hidden when the Practice session read refuses', async () => {
		await setup({ sessionRefuses: true });

		await expect.element(page.getByRole('link', { name: 'Billing', exact: true })).not.toBeInTheDocument();
	});

	it('marks the current section, and marks it with more than colour', async () => {
		await setup({ pathname: '/practices/practice-1/clients/new' });

		await expect
			.element(page.getByRole('link', { name: 'Clients', exact: true }).first())
			.toHaveAttribute('aria-current', 'page');
	});

	/*
	 * Overview is the only exact match. Every other section is a prefix, so
	 * a Client's own screen still marks Clients -- but /practices/[id]/staff
	 * must not also light up Overview, which a prefix rule would do.
	 */
	it('does not mark Overview current on a screen below it', async () => {
		await setup({ pathname: '/practices/practice-1/staff' });

		await expect
			.element(page.getByRole('link', { name: 'Overview', exact: true }).first())
			.not.toHaveAttribute('aria-current');
	});
});

describe('the avatar menu', () => {
	it('names the person and the account she is signed in as', async () => {
		await setup();

		await openAvatarMenu();

		// `.first()`: the bar renders the avatar menu three times -- wide,
		// narrow and inside the sheet -- with two of the three display:none.
		await expect.element(page.getByText('mark@example.test').first()).toBeVisible();
	});

	/*
	 * The drawing showed identity and Sign out alone. /account is the one
	 * screen where a Doula corrects her own work state (#437), and with the
	 * temporary header of links gone this menu is the only chrome that
	 * reaches it -- so it is here, deliberately, and for the drawing's own
	 * stated principle: the person, never the Practice.
	 */
	it('reaches the account screen', async () => {
		await setup();

		await openAvatarMenu();

		await expect
			.element(page.getByRole('link', { name: 'Account', exact: true }))
			.toHaveAttribute('href', '/account');
	});

	it('holds the 44px and nothing else until the session lands', async () => {
		await setup({ staffSessionRefuses: true });

		await expect
			.element(page.getByRole('button', { name: /Your account/ }))
			.not.toBeInTheDocument();
	});
});

describe('the Practice switcher', () => {
	it('names the Practice without a menu when there is only one', async () => {
		await setup();

		await expect.element(page.getByText('Riverside Doula Collective').first()).toBeVisible();
		await expect
			.element(page.getByRole('button', { name: /Riverside Doula Collective/ }))
			.not.toBeInTheDocument();
	});

	it('lists one row per Membership, with the roles held at each', async () => {
		await setup({
			memberships: [
				...ONE_MEMBERSHIP,
				{ practiceId: 'practice-2', practiceName: 'Finger Lakes Birth Support', roles: ['doula'] }
			]
		});

		await page.getByRole('button', { name: /Riverside Doula Collective/ }).first().click();

		await expect
			.element(page.getByRole('link', { name: 'Finger Lakes Birth Support' }).first())
			.toHaveAttribute('href', '/practices/practice-2');
		await expect.element(page.getByText('Doula', { exact: true }).first()).toBeVisible();
	});
});
