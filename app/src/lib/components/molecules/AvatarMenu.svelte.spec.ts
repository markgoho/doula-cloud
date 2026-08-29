import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { SignOutOutcome } from '#lib/signOut.js';
import AvatarMenu from './AvatarMenu.svelte';

interface SetupOptions {
	name?: string;
	// Booleans rather than optional values: passing `email: undefined` would
	// take the default instead of taking the email away, which is exactly
	// how the portal case first tested nothing.
	hasEmail?: boolean;
	hasAccountScreen?: boolean;
	outcome?: SignOutOutcome;
}

async function setup({
	name = 'Mark Goho',
	hasEmail = true,
	hasAccountScreen = true,
	outcome = { ok: true }
}: SetupOptions = {}) {
	const email = hasEmail ? 'mark@example.test' : undefined;
	const accountHref = hasAccountScreen ? '/account' : undefined;
	const signOut = vi.fn<() => Promise<SignOutOutcome>>().mockResolvedValue(outcome);
	await render(AvatarMenu, { name, email, accountHref, signOut });
	return { signOut, trigger: page.getByRole('button', { name: `Your account, ${name}` }) };
}

describe('AvatarMenu', () => {
	it('names the person on the trigger, so the avatar never has to', async () => {
		const { trigger } = await setup();

		await expect.element(trigger).toBeVisible();
	});

	it('shows who is signed in, and which account that is', async () => {
		const { trigger } = await setup();

		await trigger.click();

		// `.first()` because the trigger carries the same name as real, if
		// visually hidden, DOM text -- which is the point of it.
		await expect.element(page.getByText('Mark Goho').first()).toBeVisible();
		await expect.element(page.getByText('mark@example.test')).toBeVisible();
	});

	/*
	 * The drawing showed identity and Sign out alone. /account is the one
	 * screen where a Doula corrects her own work state (#437), and with the
	 * temporary header of links gone this menu is the only chrome that
	 * reaches it -- the drawing's own principle, the person and never the
	 * Practice, applied rather than departed from.
	 */
	it('reaches the account screen', async () => {
		const { trigger } = await setup();

		await trigger.click();

		await expect.element(page.getByRole('link', { name: 'Account' })).toHaveAttribute('href', '/account');
	});

	it('signs the person out', async () => {
		const { trigger, signOut } = await setup();

		await trigger.click();
		await page.getByRole('button', { name: 'Sign out' }).click();

		expect(signOut).toHaveBeenCalled();
	});

	/*
	 * The Client portal has no per-person screen to link to, and does not
	 * ask a Client to check the address she signed in with.
	 */
	it('drops the email and the account link where there are none', async () => {
		const { trigger } = await setup({ hasEmail: false, hasAccountScreen: false });

		await trigger.click();

		await expect.element(page.getByText('mark@example.test')).not.toBeInTheDocument();
		await expect.element(page.getByRole('link', { name: 'Account' })).not.toBeInTheDocument();
	});

	/*
	 * For one paint there is nobody to name. Holding the same 44px keeps the
	 * bar's inline end still rather than having the avatar shove the row
	 * when the session lands.
	 */
	it('holds its space and offers no control before the session lands', async () => {
		await setup({ name: '' });

		await expect.element(page.getByRole('button')).not.toBeInTheDocument();
	});
});
