import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { SIGN_OUT_FAILED_MESSAGE, type SignOutOutcome } from '#lib/signOut.js';
import SignOutButton from './SignOutButton.svelte';

const FAILED: SignOutOutcome = { ok: false, message: SIGN_OUT_FAILED_MESSAGE };

interface SetupOptions {
	/**
	 * What each successive click resolves with; the last entry repeats.
	 */
	outcomes?: SignOutOutcome[];
	/**
	 * Leaves the first sign-out in flight until the returned settle() is
	 * called.
	 */
	pending?: boolean;
}

async function setup({ outcomes = [{ ok: true }], pending = false }: SetupOptions = {}) {
	const { promise: inFlight, resolve: settle } = Promise.withResolvers<SignOutOutcome>();
	const signOut = vi.fn<() => Promise<SignOutOutcome>>(() => inFlight);
	if (!pending) {
		for (const outcome of outcomes) signOut.mockResolvedValueOnce(outcome);
		signOut.mockResolvedValue(outcomes.at(-1)!);
	}
	await render(SignOutButton, { signOut });
	return { signOut, settle, signOutButton: page.getByRole('button', { name: 'Sign out' }) };
}

describe('SignOutButton', () => {
	it('signs out when clicked', async () => {
		const { signOut, signOutButton } = await setup();

		await signOutButton.click();

		expect(signOut).toHaveBeenCalled();
		await expect.element(page.getByRole('alert')).not.toBeInTheDocument();
	});

	it('cannot be clicked a second time while the first sign-out is still running', async () => {
		const { settle, signOutButton } = await setup({ pending: true });

		await signOutButton.click();

		await expect.element(signOutButton).toBeDisabled();
		settle({ ok: true });
	});

	it('reports a failed sign-out rather than appearing to succeed', async () => {
		const { signOutButton } = await setup({ outcomes: [FAILED] });

		await signOutButton.click();

		await expect.element(page.getByRole('alert')).toHaveTextContent(SIGN_OUT_FAILED_MESSAGE);
	});

	it('clears a previous failure when sign-out is retried', async () => {
		const { signOutButton } = await setup({ outcomes: [FAILED, { ok: true }] });

		await signOutButton.click();
		await expect.element(page.getByRole('alert')).toBeVisible();

		await signOutButton.click();

		await expect.element(page.getByRole('alert')).not.toBeInTheDocument();
	});
});
