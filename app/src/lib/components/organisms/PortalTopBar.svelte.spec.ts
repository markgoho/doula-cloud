import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { SignOutOutcome } from '#lib/signOut.js';
import PortalTopBar from './PortalTopBar.svelte';
import '#lib/styles/tokens.css';

const NAV_ITEMS = [
	{ label: 'Your care', href: '/care', current: true },
	{ label: 'Messages', href: '/messages', current: false },
	{ label: 'Birth plan', href: '/birth-plan', current: false },
	{ label: 'Contract', href: '/contract', current: false }
];

async function setup({
	practiceName = 'Riverside Doula Collective',
	switcherLabel = 'Riverside Doula Collective, started Mar 12, 2026'
} = {}) {
	// Pinned rather than left to the runner's default: the nav renders twice
	// with one copy display:none, so which one is visible is a fact about
	// the viewport and should be stated by the test.
	await page.viewport(1440, 900);
	const signOut = vi.fn<() => Promise<SignOutOutcome>>().mockResolvedValue({ ok: true });
	await render(PortalTopBar, {
		practiceName,
		switcherLabel,
		navItems: NAV_ITEMS,
		name: 'Tasha Bell',
		signOut
	});
	return { signOut };
}

describe('PortalTopBar', () => {
	/*
	 * The Practice's name is the portal's identity, not `Doula Cloud`: a
	 * Client's relationship is with her doula's practice and not with the
	 * software it runs on.
	 */
	it('is named after the Practice, never after the product', async () => {
		await setup();

		await expect.element(page.getByText('Riverside Doula Collective')).toBeVisible();
		await expect.element(page.getByText('Doula Cloud')).not.toBeInTheDocument();
	});

	it.each(['Your care', 'Messages', 'Birth plan', 'Contract'])('offers %s', async (label) => {
		await setup();

		await expect.element(page.getByRole('link', { name: label }).first()).toBeVisible();
	});

	/*
	 * #310: the persistent, always-present way to the portal root -- a
	 * real link, not a modal or a dropdown that reimplements navigation.
	 * Its accessible name is `switcherLabel`, not "Your care": that text
	 * already names the nav item pointing at the hub, and two links with
	 * the same accessible name and different destinations on one screen
	 * would be a WCAG 2.4.4 failure.
	 */
	it('links the Practice name to the portal root', async () => {
		await setup();

		const link = page.getByRole('link', { name: 'Riverside Doula Collective, started Mar 12, 2026' });
		await expect.element(link).toBeVisible();
		expect(link.element()).toHaveAttribute('href', '/');
	});

	it("does not reuse \"Your care\" as the portal-root link's own name", async () => {
		await setup();

		await expect.element(page.getByRole('link', { name: 'Your care', exact: true })).toHaveAttribute(
			'href',
			'/care'
		);
	});

	it('marks where the person is with more than color', async () => {
		await setup();

		await expect
			.element(page.getByRole('link', { name: 'Your care' }).first())
			.toHaveAttribute('aria-current', 'page');
	});

	/*
	 * A Client belongs to exactly one Practice, so there is nothing to
	 * switch between and no switcher to offer.
	 */
	it('offers no Practice switcher', async () => {
		await setup();

		await expect
			.element(page.getByRole('button', { name: /Riverside Doula Collective/ }))
			.not.toBeInTheDocument();
	});

	it('carries sign-out behind the avatar', async () => {
		const { signOut } = await setup();

		await page.getByRole('button', { name: 'Your account, Tasha Bell' }).click();
		await page.getByRole('button', { name: 'Sign out' }).click();

		expect(signOut).toHaveBeenCalled();
	});

	/*
	 * The same four destinations render twice, one set always display:none.
	 * A hidden subtree is out of the accessibility tree too, so neither the
	 * tab order nor a screen reader ever meets the pair -- but only one of
	 * the two navigations is ever visible at a width.
	 */
	it('shows one nav at a time, whichever width it is at', async () => {
		await setup();

		const navigations = page.getByRole('navigation').all();
		const visible = navigations.filter((nav) => nav.element().checkVisibility());
		expect(visible).toHaveLength(1);
	});
});
