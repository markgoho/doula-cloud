import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { PracticeOption } from '#lib/components/molecules/PracticeSwitcher.svelte';
import type { SignOutOutcome } from '#lib/signOut.js';
import StaffTopBar from './StaffTopBar.svelte';
import '#lib/styles/tokens.css';

const NAV_ITEMS = [
	{ label: 'Overview', href: '/overview', current: true },
	{ label: 'Clients', href: '/clients', current: false },
	{ label: 'Billing', href: '/billing', current: false },
	{ label: 'Staff', href: '/staff', current: false },
	{ label: 'Offers', href: '/offers', current: false },
	{ label: 'Settings', href: '/settings', current: false }
];

const PRACTICES: PracticeOption[] = [
	{ practiceId: 'p1', practiceName: 'Riverside Doula Collective', roles: ['owner'], href: '/p1' },
	{ practiceId: 'p2', practiceName: 'Finger Lakes Birth Support', roles: ['doula'], href: '/p2' }
];

/*
 * The viewport is pinned in every setup rather than left to the runner's
 * default. The bar has one breakpoint and both trees are always in the
 * document with one display:none, so a default narrower than 60rem makes
 * every assertion about the wide bar fail for a reason that has nothing to
 * do with the bar -- which is exactly what it did the first time.
 */
const WIDE = [1440, 900] as const;
const NARROW = [390, 844] as const;

async function setup({ name = 'Mark Goho' } = {}) {
	await page.viewport(...WIDE);
	const signOut = vi.fn<() => Promise<SignOutOutcome>>().mockResolvedValue({ ok: true });
	await render(StaffTopBar, {
		navItems: NAV_ITEMS,
		practices: PRACTICES,
		currentPracticeId: 'p1',
		name,
		email: 'mark@example.test',
		accountHref: '/account',
		signOut
	});
	return { signOut };
}

describe('StaffTopBar', () => {
	it('carries the lockup', async () => {
		await setup();

		await expect.element(page.getByText('Doula Cloud').first()).toBeVisible();
	});

	it.each(NAV_ITEMS.map((item) => item.label))('offers %s', async (label) => {
		await setup();

		await expect.element(page.getByRole('link', { name: label }).first()).toBeInTheDocument();
	});

	it('marks the current section with more than colour', async () => {
		await setup();

		await expect
			.element(page.getByRole('link', { name: 'Overview' }).first())
			.toHaveAttribute('aria-current', 'page');
	});

	it('carries sign-out behind the avatar', async () => {
		const { signOut } = await setup();

		await page.getByRole('button', { name: 'Your account, Mark Goho' }).first().click();
		await page.getByRole('button', { name: 'Sign out' }).first().click();

		expect(signOut).toHaveBeenCalled();
	});
});

/*
 * The narrow sheet is a <dialog> and not a popover, because it covers the
 * page: Tab reaching the content underneath would walk a person through
 * something they cannot see. showModal() is what traps focus, and the
 * three assertions below are the AC -- it opens, it traps, and closing it
 * puts focus back on the control that opened it.
 */
async function setupNarrow() {
	const result = await setup();
	await page.viewport(...NARROW);
	return { ...result, hamburger: page.getByRole('button', { name: 'Menu' }) };
}

describe('the narrow sheet', () => {
	// A closed <dialog> is not rendered at all, so there is no `dialog` role
	// to assert against -- what tells the sheet apart is its own controls.
	it('is shut until the hamburger is pressed', async () => {
		await setupNarrow();

		await expect.element(page.getByRole('button', { name: 'Close menu' })).not.toBeInTheDocument();
	});

	it('opens with every section in it', async () => {
		const { hamburger } = await setupNarrow();

		await hamburger.click();

		const sheet = page.getByRole('dialog');
		await expect.element(sheet).toBeVisible();
		for (const item of NAV_ITEMS) {
			await expect.element(sheet.getByRole('link', { name: item.label })).toBeVisible();
		}
	});

	it('carries the Practice switcher, which the narrow bar has no room for', async () => {
		const { hamburger } = await setupNarrow();

		await hamburger.click();

		await expect
			.element(page.getByRole('dialog').getByText('Practice', { exact: true }))
			.toBeVisible();
	});

	it('traps focus while it is open', async () => {
		const { hamburger } = await setupNarrow();

		await hamburger.click();

		// A modal dialog makes everything outside it inert, so the element
		// that opened the sheet is no longer reachable from inside it.
		expect(page.getByRole('dialog').element().contains(document.activeElement)).toBe(true);
	});

	it('closes again, and gives focus back to the hamburger', async () => {
		const { hamburger } = await setupNarrow();
		await hamburger.click();

		await page.getByRole('button', { name: 'Close menu' }).click();

		await expect.element(page.getByRole('button', { name: 'Close menu' })).not.toBeInTheDocument();
		expect(document.activeElement).toBe(hamburger.element());
	});

	/*
	 * A bottom tab bar was drawn and rejected: five slots cannot carry six
	 * sections without a `More`, and `More` is not a noun this domain has.
	 */
	it('renames and groups nothing the domain does not have', async () => {
		const { hamburger } = await setupNarrow();

		await hamburger.click();

		await expect.element(page.getByRole('link', { name: 'More' })).not.toBeInTheDocument();
	});
});
