import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { PracticeOption } from '#lib/components/molecules/PracticeSwitcher.svelte';
import type { SignOutOutcome } from '#lib/signOut.js';
import {
	contentBottom,
	panelHolding,
	withoutAnchorPositioning
} from '#lib/components/molecules/MenuButton.testing.js';
import { CONFORMANCE_COMMITMENT, findBlockSpill } from '../../../routes/style-guide/continuum.js';
import StaffTopBar from './StaffTopBar.svelte';
// The app's whole cascade, not the tokens alone: where a panel lands
// turns on its border-box width, and `border-box` is reset.css's (#2151).
import '#lib/styles/app.css';

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

async function setup({
	firstName = 'Mark',
	lastName = 'Goho',
	practices = PRACTICES,
	currentPracticeId = 'p1',
	navItems = NAV_ITEMS
} = {}) {
	await page.viewport(...WIDE);
	const signOut = vi.fn<() => Promise<SignOutOutcome>>().mockResolvedValue({ ok: true });
	await render(StaffTopBar, {
		navItems,
		practices,
		currentPracticeId,
		firstName,
		lastName,
		email: 'mark@example.test',
		accountHref: '/account',
		signOut
	});
	return { signOut };
}

describe('StaffTopBar', () => {
	it('carries the lockup', async () => {
		await setup();

		await expect.element(page.getByText('DoulaCloud').first()).toBeVisible();
	});

	it.each(NAV_ITEMS.map((item) => item.label))('offers %s', async (label) => {
		await setup();

		await expect.element(page.getByRole('link', { name: label }).first()).toBeInTheDocument();
	});

	it('marks the current section with more than color', async () => {
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
async function setupNarrow(options: Parameters<typeof setup>[0] = {}) {
	const result = await setup(options);
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

	/*
	 * #1573 anchors a menu's panel to the box around its button, which a top
	 * bar stretches to the bar's height. The sheet lays its switcher out in
	 * a column, which would stretch that box across the whole sheet and hang
	 * the panel off the sheet's far edge rather than off the button.
	 */
	it('opens the switcher panel from the switcher, not from the far edge', async () => {
		const { hamburger } = await setupNarrow();
		await hamburger.click();

		const sheet = page.getByRole('dialog');
		const switcher = sheet.getByRole('button', { name: 'Riverside Doula Collective' });
		await switcher.click();

		const other = sheet.getByRole('link', { name: 'Finger Lakes Birth Support' });
		await expect.element(other).toBeVisible();
		const buttonBox = switcher.element().getBoundingClientRect();
		// The panel is wider than the button and the switcher sits at the
		// sheet's start edge, so it flips to open from the button's start.
		expect(panelHolding(other).getBoundingClientRect().left).toBeCloseTo(buttonBox.left, 0);
	});

	/*
	 * #673: a route scoped to the person rather than to a Practice
	 * (/account, #484) hands the bar no Practice at all, and the switcher
	 * renders nothing. The heading and the divider above it go too --
	 * otherwise the sheet announces a section with nothing in it.
	 */
	it('drops the whole Practice block when there is no Practice to name', async () => {
		const { hamburger } = await setupNarrow({ practices: [], currentPracticeId: '' });

		await hamburger.click();

		await expect
			.element(page.getByRole('dialog').getByText('Practice', { exact: true }))
			.not.toBeInTheDocument();
	});

	it('still opens, traps, and still closes, on a route with no Practice', async () => {
		const { hamburger } = await setupNarrow({ practices: [], currentPracticeId: '' });

		await hamburger.click();

		const sheet = page.getByRole('dialog');
		await expect.element(sheet).toBeVisible();
		for (const item of NAV_ITEMS) {
			await expect.element(sheet.getByRole('link', { name: item.label })).toBeVisible();
		}
		// A block removed from the sheet is a block removed from the focus
		// order, so the trap is asserted here too and not only on a route
		// that has a Practice in it.
		expect(sheet.element().contains(document.activeElement)).toBe(true);
		await page.getByRole('button', { name: 'Close menu' }).click();
		expect(document.activeElement).toBe(hamburger.element());
	});

	/*
	 * One Membership is not "nothing to name": she still sees which
	 * Practice she is in, so the heading stays. Only the caret goes, which
	 * is PracticeSwitcher's own rule.
	 */
	it('keeps the heading for a person who belongs to exactly one Practice', async () => {
		const { hamburger } = await setupNarrow({
			practices: [PRACTICES[0]!],
			currentPracticeId: 'p1'
		});

		await hamburger.click();

		const sheet = page.getByRole('dialog');
		await expect.element(sheet.getByText('Practice', { exact: true })).toBeVisible();
		await expect.element(sheet.getByText('Riverside Doula Collective')).toBeVisible();
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
	 * #2235: following a section left the sheet open over the page it
	 * chose. The router is stood in for by a listener on the document,
	 * where SvelteKit's own sits, so the press is a real link press that
	 * does not take the test runner's frame with it. The sheet shuts
	 * before the router runs, so the focus the router then resets to the
	 * new page is not taken back to the hamburger by a later close.
	 */
	it.each(NAV_ITEMS.map((item) => item.label))('shuts when %s is followed', async (label) => {
		const { hamburger } = await setupNarrow();
		await hamburger.click();
		let openWhenRouted: boolean | undefined;
		const route = (event: MouseEvent) => {
			event.preventDefault();
			openWhenRouted = document.querySelector('dialog')?.open;
		};
		document.addEventListener('click', route);

		try {
			await page.getByRole('dialog').getByRole('link', { name: label }).click();
		} finally {
			document.removeEventListener('click', route);
		}

		await expect.element(page.getByRole('button', { name: 'Close menu' })).not.toBeInTheDocument();
		expect(openWhenRouted).toBe(false);
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

/*
 * #1837: the agreement screen while an agreement is due (#1549) and
 * /account both hand the bar no section. A Menu button there opened a
 * sheet whose only landmark held no link, and the one thing the sheet
 * could still offer -- the Practice switcher -- is already in the narrow
 * bar itself.
 */
describe('StaffTopBar, on a screen that offers no section (#1837)', () => {
	it('draws no Practice nav at the wide end', async () => {
		await setup({ navItems: [] });

		await expect
			.element(page.getByRole('navigation', { name: 'Practice' }))
			.not.toBeInTheDocument();
	});

	it('draws no Menu button under the narrow floor, and keeps the switcher in the bar', async () => {
		await setupNarrow({ navItems: [] });

		await expect.element(page.getByRole('button', { name: 'Menu' })).not.toBeInTheDocument();
		await expect
			.element(page.getByRole('button', { name: 'Riverside Doula Collective' }))
			.toBeVisible();
		await expect.element(page.getByRole('navigation', { name: 'Practice' })).not.toBeInTheDocument();
	});
});

/*
 * #1573: the bar held a fixed 3.75rem, so a long Practice name wrapped in
 * the switcher and drew out of the bar, above and below it. The bar is a
 * minimum now, and grows. 320px is the narrow tree at the conformance
 * commitment; 1056px puts the bar's content box (the header less its two
 * gutters, which is what the container query reads) just above its
 * floor, where the wide tree leaves the name the least room it ever gets.
 * The names are the style-guide demo's own.
 */
const LONG_NAME = 'Highland Midwifery & Birth Support Collective of Western New York';
const LONG_PRACTICES: PracticeOption[] = [
	{ practiceId: 'p1', practiceName: LONG_NAME, roles: ['owner', 'admin'], href: '/p1' },
	{
		practiceId: 'p2',
		practiceName: 'Finger Lakes Birth Support and Postpartum Care Cooperative',
		roles: ['doula'],
		href: '/p2'
	}
];
const JUST_ABOVE_THE_FLOOR = 1056;

// Each panel the bar opens, by its trigger, and something only that panel
// holds -- the other Practice's link, or the avatar menu's sign-out.
const PANELS = [
	[LONG_NAME, 'link', LONG_PRACTICES[1].practiceName],
	['Your account, Mark Goho', 'button', 'Sign out']
] as const;

async function setupLong(width: number) {
	const result = await setup({ practices: LONG_PRACTICES });
	await page.viewport(width, 900);
	// Geometry, which the accessible tree does not carry: the banner is the
	// box the name has to stay inside.
	const header = page.getByRole('banner').element() as HTMLElement;
	return { ...result, header };
}

/*
 * #1772 made the wide bar two rows, and the switcher and the avatar are on
 * the first. Their panels open below that row -- over the nav row, the way
 * a menu opens over a page -- rather than from the middle of a row grown
 * tall by a wrapped name (#1573). The nav's top edge is that row's bottom
 * edge.
 */
function firstRowBottom(): number {
	return page.getByRole('navigation', { name: 'Practice' }).element().getBoundingClientRect().top;
}

describe('StaffTopBar, with a Practice name long enough to wrap (#1573)', () => {
	it.each([CONFORMANCE_COMMITMENT, JUST_ABOVE_THE_FLOOR])(
		'grows to hold the whole name at %ipx',
		async (width) => {
			const { header } = await setupLong(width);

			const switcher = page.getByRole('button', { name: LONG_NAME });
			await expect.element(switcher).toBeVisible();
			const switcherBox = switcher.element().getBoundingClientRect();
			const headerBox = header.getBoundingClientRect();
			expect(switcherBox.top).toBeGreaterThanOrEqual(headerBox.top);
			expect(switcherBox.bottom).toBeLessThanOrEqual(headerBox.bottom);
			expect(findBlockSpill(header)).toBeUndefined();
		}
	);

	it('keeps the current section’s accent rule on the bar’s own bottom edge', async () => {
		const { header } = await setupLong(JUST_ABOVE_THE_FLOOR);

		const current = page.getByRole('link', { name: 'Overview' });
		await expect.element(current).toBeVisible();
		expect(current.element().getBoundingClientRect().bottom).toBeCloseTo(contentBottom(header), 0);
	});

	it.each(PANELS)('opens the %s panel below its own row', async (trigger, role, inside) => {
		await setupLong(JUST_ABOVE_THE_FLOOR);

		await page.getByRole('button', { name: trigger }).click();

		const content = page.getByRole(role, { name: inside });
		await expect.element(content).toBeVisible();
		expect(panelHolding(content).getBoundingClientRect().top).toBeGreaterThanOrEqual(
			firstRowBottom()
		);
	});

	it.each(PANELS)(
		'opens the %s panel below its own row where there is no anchor positioning',
		async (trigger, role, inside) => {
			await setupLong(JUST_ABOVE_THE_FLOOR);
			const withoutAnchors = withoutAnchorPositioning();
			try {
				await page.getByRole('button', { name: trigger }).click();

				const content = page.getByRole(role, { name: inside });
				await expect.element(content).toBeVisible();
				expect(panelHolding(content).getBoundingClientRect().top).toBeCloseTo(firstRowBottom(), 0);
			} finally {
				withoutAnchors.remove();
			}
		}
	);
});

/*
 * #2151: #2141 put the Practice's name beside the Menu button, near the
 * narrow bar's inline start. Its panel is wider than the room on either
 * side of it at 320px, and the browser shoved it against the window's
 * edge. The gutter is read off the bar as drawn -- how far in from the
 * banner's own edge the Menu button sits -- and the panel has to keep it
 * from both edges of the window.
 */
async function setupGutter(width: number) {
	await setupNarrow();
	await page.viewport(width, 844);
	const header = page.getByRole('banner').element().getBoundingClientRect();
	const menu = page.getByRole('button', { name: 'Menu' }).element().getBoundingClientRect();
	return { gutter: menu.left - header.left };
}

describe('StaffTopBar, the narrow bar’s panels and the page gutter (#2151)', () => {
	it.each([CONFORMANCE_COMMITMENT, NARROW[0]])(
		'keeps the gutter on both sides of the switcher’s panel at %ipx',
		async (width) => {
			const { gutter } = await setupGutter(width);

			await page.getByRole('button', { name: 'Riverside Doula Collective' }).click();

			const other = page.getByRole('link', { name: 'Finger Lakes Birth Support' });
			await expect.element(other).toBeVisible();
			const panel = panelHolding(other).getBoundingClientRect();
			expect(panel.left).toBeGreaterThanOrEqual(gutter);
			expect(window.innerWidth - panel.right).toBeGreaterThanOrEqual(gutter);
		}
	);

	// The avatar has room to open toward the inline start, so it still
	// lines up with its own button's end edge, as it did before.
	it.each([CONFORMANCE_COMMITMENT, NARROW[0]])(
		'still ends the avatar’s panel at its button at %ipx',
		async (width) => {
			await setupGutter(width);
			const avatar = page.getByRole('button', { name: 'Your account, Mark Goho' });

			await avatar.click();

			const signOut = page.getByRole('button', { name: 'Sign out' });
			await expect.element(signOut).toBeVisible();
			expect(panelHolding(signOut).getBoundingClientRect().right).toBeCloseTo(
				avatar.element().getBoundingClientRect().right,
				0
			);
		}
	);
});

/*
 * #2161: the sheet's own bar padded its edges less than the narrow bar it
 * covers, so opening it moved the avatar 8px toward the window's edge and
 * the Close button 8px in from where the Menu button was, and the avatar's
 * panel, which ends at its button, sat inside the gutter. The sheet's bar
 * is drawn over the narrow bar, so each control has to land where the one
 * it stands in for already was.
 */
const SHEET_WIDTHS = [CONFORMANCE_COMMITMENT, NARROW[0], 768];
const AVATAR = 'Your account, Mark Goho';

/*
 * The inline edges, exactly. The block center is held to the nearest
 * pixel: at 320px the narrow bar's switcher makes that bar a fraction of
 * a pixel taller than the sheet's, which has no switcher in it.
 */
function expectSamePlace(open: Element, closed: DOMRect) {
	const box = open.getBoundingClientRect();
	expect({ left: box.left, right: box.right }).toEqual({
		left: closed.left,
		right: closed.right
	});
	expect(box.top + box.height / 2).toBeCloseTo(closed.top + closed.height / 2, 0);
}

describe('StaffTopBar, the sheet’s bar over the narrow bar (#2161)', () => {
	it.each(SHEET_WIDTHS)(
		'puts Close and the avatar where Menu and the avatar were at %ipx',
		async (width) => {
			await setupGutter(width);
			const menu = page.getByRole('button', { name: 'Menu' });
			const closed = {
				menu: menu.element().getBoundingClientRect(),
				avatar: page.getByRole('button', { name: AVATAR }).element().getBoundingClientRect()
			};

			await menu.click();

			const sheet = page.getByRole('dialog');
			const close = sheet.getByRole('button', { name: 'Close menu' });
			await expect.element(close).toBeVisible();
			expectSamePlace(close.element(), closed.menu);
			expectSamePlace(sheet.getByRole('button', { name: AVATAR }).element(), closed.avatar);
		}
	);

	it.each(SHEET_WIDTHS)(
		'keeps the gutter on both sides of the sheet’s avatar panel at %ipx',
		async (width) => {
			const { gutter } = await setupGutter(width);
			await page.getByRole('button', { name: 'Menu' }).click();

			await page.getByRole('dialog').getByRole('button', { name: AVATAR }).click();

			const signOut = page.getByRole('dialog').getByRole('button', { name: 'Sign out' });
			await expect.element(signOut).toBeVisible();
			const panel = panelHolding(signOut).getBoundingClientRect();
			expect(panel.left).toBeGreaterThanOrEqual(gutter);
			expect(window.innerWidth - panel.right).toBeGreaterThanOrEqual(gutter);
		}
	);
});
