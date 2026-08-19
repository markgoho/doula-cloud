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

interface SetupOptions {
	outcome?: SignOutOutcome;
	routeParameters?: { practiceId?: string };
}

async function setup({
	outcome = { ok: true },
	routeParameters = { practiceId: 'practice-1' }
}: SetupOptions = {}) {
	pageState.params = routeParameters;
	goto.mockReset();
	signOutOfSession.mockReset();
	signOutOfSession.mockResolvedValue(outcome);
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
