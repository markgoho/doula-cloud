import { createRawSnippet } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { SignOutOutcome } from '#lib/signOut.js';
import Layout from './+layout.svelte';

vi.mock('$app/state', () => ({ page: { params: { practiceId: 'practice-1' } } }));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const signOutOfSession = vi.hoisted(() => vi.fn<() => Promise<SignOutOutcome>>());
vi.mock('#lib/signOut.js', () => ({ signOutOfSession }));

interface SetupOptions {
	outcome?: SignOutOutcome;
}

async function setup({ outcome = { ok: true } }: SetupOptions = {}) {
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
			expect.objectContaining({ practiceId: 'practice-1' })
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
