import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { SignOutOutcome } from '#lib/signOut.js';
import {
	contentBottom,
	panelHolding,
	withoutAnchorPositioning
} from '#lib/components/molecules/MenuButton.testing.js';
import { CONFORMANCE_COMMITMENT, findBlockSpill } from '../../../routes/style-guide/continuum.js';
import PortalTopBar from './PortalTopBar.svelte';
import '#lib/styles/tokens.css';

const NAV_ITEMS = [
	{ label: 'Your care', href: '/care', current: true },
	{ label: 'Messages', href: '/messages', current: false },
	{ label: 'Birth plan', href: '/birth-plan', current: false },
	{ label: 'Contract', href: '/contract', current: false },
	{ label: 'Notifications', href: '/notifications', current: false }
];

async function setup({
	practiceName = 'Riverside Doula Collective',
	switcherLabel = 'Riverside Doula Collective, started Mar 12, 2026',
	width = 1440
} = {}) {
	// Pinned rather than left to the runner's default: the nav renders twice
	// with one copy display:none, so which one is visible is a fact about
	// the viewport and should be stated by the test.
	await page.viewport(width, 900);
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
	 * The Practice's name is the portal's identity, not `DoulaCloud`: a
	 * Client's relationship is with her doula's practice and not with the
	 * software it runs on.
	 */
	it('is named after the Practice, never after the product', async () => {
		await setup();

		await expect.element(page.getByText('Riverside Doula Collective')).toBeVisible();
		await expect.element(page.getByText('DoulaCloud')).not.toBeInTheDocument();
	});

	it.each(['Your care', 'Messages', 'Birth plan', 'Contract', 'Notifications'])('offers %s', async (label) => {
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
	 * The same destinations render twice, one set always display:none.
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

	/*
	 * #1568: five items on one line ended near 396px, so every signed-in
	 * portal screen scrolled sideways at 320px. The narrow row wraps
	 * instead, and every item stays visible -- no menu, nothing hidden.
	 */
	it('wraps its narrow row rather than overflowing at 320px', async () => {
		await setup({ width: 320 });

		const header = page.getByRole('banner').element();
		expect(header.scrollWidth).toBeLessThanOrEqual(header.clientWidth);
		for (const { label } of NAV_ITEMS) {
			await expect.element(page.getByRole('link', { name: label, exact: true })).toBeVisible();
		}
	});
});

/*
 * #1573: the bar held a fixed 3.75rem, so a long Practice name wrapped onto
 * several lines and drew out of it over the page under it. The bar is a
 * minimum now, and grows. 320px is the narrow tree at the conformance
 * commitment; 740px puts the bar (the header is the container, and its
 * gutters sit on the row inside it) just above its 44.5rem floor, where the
 * wide tree leaves the name the least room it ever gets.
 */
describe('PortalTopBar, with a Practice name long enough to wrap (#1573)', () => {
	const LONG_NAME = 'Highland Midwifery & Birth Support Collective of Western New York';
	const JUST_ABOVE_THE_FLOOR = 740;

	async function setupLong(width: number) {
		const result = await setup({
			practiceName: LONG_NAME,
			switcherLabel: `${LONG_NAME}, started Mar 12, 2026`,
			width
		});
		// Geometry, which the accessible tree does not carry: the banner is
		// the box the name has to stay inside.
		const header = page.getByRole('banner').element() as HTMLElement;
		return { ...result, header };
	}

	it.each([CONFORMANCE_COMMITMENT, JUST_ABOVE_THE_FLOOR])(
		'grows to hold the whole name at %ipx',
		async (width) => {
			const { header } = await setupLong(width);

			const name = page.getByRole('link', { name: `${LONG_NAME}, started Mar 12, 2026` });
			const nameBox = name.element().getBoundingClientRect();
			const headerBox = header.getBoundingClientRect();
			expect(nameBox.top).toBeGreaterThanOrEqual(headerBox.top);
			expect(nameBox.bottom).toBeLessThanOrEqual(headerBox.bottom);
			expect(findBlockSpill(header)).toBeUndefined();
		}
	);

	it('keeps the current item’s accent rule on the bar’s own bottom edge', async () => {
		const { header } = await setupLong(JUST_ABOVE_THE_FLOOR);

		const current = page.getByRole('link', { name: 'Your care', exact: true });
		await expect.element(current).toBeVisible();
		expect(current.element().getBoundingClientRect().bottom).toBeCloseTo(contentBottom(header), 0);
	});

	it('opens the account menu below the bar', async () => {
		const { header } = await setupLong(JUST_ABOVE_THE_FLOOR);

		await page.getByRole('button', { name: 'Your account, Tasha Bell' }).click();

		const signOutButton = page.getByRole('button', { name: 'Sign out' });
		await expect.element(signOutButton).toBeVisible();
		expect(panelHolding(signOutButton).getBoundingClientRect().top).toBeGreaterThanOrEqual(
			contentBottom(header)
		);
	});

	it('opens the account menu below the bar where there is no anchor positioning', async () => {
		const { header } = await setupLong(JUST_ABOVE_THE_FLOOR);
		const withoutAnchors = withoutAnchorPositioning();
		try {
			await page.getByRole('button', { name: 'Your account, Tasha Bell' }).click();

			const signOutButton = page.getByRole('button', { name: 'Sign out' });
			await expect.element(signOutButton).toBeVisible();
			expect(panelHolding(signOutButton).getBoundingClientRect().top).toBeCloseTo(
				contentBottom(header), 0
			);
		} finally {
			withoutAnchors.remove();
		}
	});
});
