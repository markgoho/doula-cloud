import { createRawSnippet } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { SignOutOutcome } from '#lib/signOut.js';
import Layout from './+layout.svelte';

// Mutable rather than a fixed literal: the layout skips the push
// unregister on a screen with no Practice in its route, and that branch
// needs a test that can take practiceId away.
const pageState = vi.hoisted(() => ({ params: {} as { practiceId?: string } }));
vi.mock('$app/state', () => ({ page: pageState }));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const signOutOfSession = vi.hoisted(() => vi.fn<() => Promise<SignOutOutcome>>());
vi.mock('#lib/signOut.js', () => ({ signOutOfSession }));

// The layout reads its own roles so the temporary nav can hide the
// Owner-only links -- see the comment on the nav in +layout.svelte.
const apiFetchWithSession = vi.hoisted(() => vi.fn());
const apiFetch = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetch, apiFetchWithSession }));

interface SetupOptions {
	outcome?: SignOutOutcome;
	routeParameters?: { practiceId?: string };
	roles?: string[];
	sessionRefuses?: boolean;
}

async function setup({
	outcome = { ok: true },
	routeParameters = { practiceId: 'practice-1' },
	roles = ['owner'],
	sessionRefuses = false
}: SetupOptions = {}) {
	pageState.params = routeParameters;
	goto.mockReset();
	signOutOfSession.mockReset();
	signOutOfSession.mockResolvedValue(outcome);
	apiFetchWithSession.mockReset();
	apiFetchWithSession.mockResolvedValue({
		ok: !sessionRefuses,
		text: () => Promise.resolve('nope'),
		json: () => Promise.resolve({ roles })
	} as Response);
	await render(Layout, {
		children: createRawSnippet(() => ({ render: () => '<p>staff child content</p>' }))
	});
	return { signOutButton: page.getByRole('button', { name: 'Sign out' }) };
}

describe('Staff authenticated layout', () => {
	it('renders its children', async () => {
		await setup();

		await expect.element(page.getByText('staff child content')).toBeVisible();
	});

	it('offers a sign-out control alongside whatever screen is showing', async () => {
		const { signOutButton } = await setup();

		await expect.element(signOutButton).toBeVisible();
	});

	it('signs out of the current Practice and lands on the Staff login screen', async () => {
		const { signOutButton } = await setup();

		await signOutButton.click();

		expect(signOutOfSession).toHaveBeenCalledWith(
			expect.objectContaining({
				unsubscribeURL: '/api/practices/practice-1/push-subscriptions'
			})
		);
		expect(goto).toHaveBeenCalledWith('/login');
	});

	it('signs out without an unregister when the route carries no Practice', async () => {
		const { signOutButton } = await setup({ routeParameters: {} });

		await signOutButton.click();

		expect(signOutOfSession).toHaveBeenCalledWith(
			expect.objectContaining({ unsubscribeURL: undefined })
		);
		expect(goto).toHaveBeenCalledWith('/login');
	});

	it('stays put and reports a sign-out that failed', async () => {
		const { signOutButton } = await setup({
			outcome: { ok: false, message: 'Sign-out failed.' }
		});

		await signOutButton.click();

		await expect.element(page.getByRole('alert')).toHaveTextContent('Sign-out failed.');
		expect(goto).not.toHaveBeenCalled();
	});
});

describe('the temporary nav the shell will replace (#452)', () => {
	it('carries the links the Practice landing page used to hold itself', async () => {
		await setup({ roles: ['doula'] });

		const nav = page.getByRole('navigation', { name: 'Practice' });
		await expect.element(nav).toBeVisible();
		for (const label of ['Overview', 'Clients', 'Billing', 'Your offers', 'Payments']) {
			await expect.element(nav.getByRole('link', { name: label })).toBeVisible();
		}
	});

	const ownerOnly = [['Staff'], ['Invite a Staff member'], ['Plan Templates'], ['Contract Template']];

	it.each(ownerOnly)('shows %s to an Owner', async (label) => {
		await setup({ roles: ['owner'] });

		await expect.element(page.getByRole('link', { name: label, exact: true })).toBeVisible();
	});

	it.each(ownerOnly)('hides %s from a Doula', async (label) => {
		await setup({ roles: ['doula'] });

		expect(page.getByRole('link', { name: label, exact: true }).elements()).toHaveLength(0);
	});

	it('keeps the Owner links hidden when the session read refuses', async () => {
		await setup({ sessionRefuses: true });

		expect(page.getByRole('link', { name: 'Staff', exact: true }).elements()).toHaveLength(0);
	});

	it('renders no nav on a Staff screen with no Practice in its route', async () => {
		await setup({ routeParameters: {} });

		expect(page.getByRole('navigation', { name: 'Practice' }).elements()).toHaveLength(0);
	});
});
