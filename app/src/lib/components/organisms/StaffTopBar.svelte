<script module lang="ts">
	export interface NavItem {
		label: string;
		href: string;
		current: boolean;
	}
</script>

<script lang="ts">
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import AvatarMenu from '#lib/components/molecules/AvatarMenu.svelte';
	import BrandLockup from '#lib/components/molecules/BrandLockup.svelte';
	import PracticeSwitcher, {
		currentPracticeOf,
		type PracticeOption
	} from '#lib/components/molecules/PracticeSwitcher.svelte';
	import type { SignOutOutcome } from '#lib/signOut.js';

	/*
	 * The Staff shell's bar (#431, #452). Wide, it is two rows (#1772): the
	 * lockup with the Practice switcher and the avatar at the far end, and
	 * under them a flat nav of up to ten sections. Its first row grows when
	 * the Practice's name wraps (#1573) rather than letting the name draw
	 * out of it over the page.
	 *
	 * Narrow, the nav and the switcher move into a full-screen sheet behind
	 * a hamburger. A bottom tab bar was drawn and rejected -- five slots
	 * cannot carry six sections without a `More`, and `More` is not a noun
	 * this domain has.
	 */
	interface Properties {
		navItems: NavItem[];
		practices: PracticeOption[];
		currentPracticeId: string;
		firstName: string;
		lastName: string;
		email?: string;
		accountHref?: string;
		signOut: () => Promise<SignOutOutcome>;
	}

	let {
		navItems,
		practices,
		currentPracticeId,
		firstName,
		lastName,
		email,
		accountHref,
		signOut
	}: Properties = $props();

	/*
	 * `<dialog>` rather than a popover, because the sheet is the one place
	 * in the shell that has to trap focus: it covers the page, so Tab
	 * reaching the page underneath would walk a person through content they
	 * cannot see. showModal() traps, Escape closes, and the browser returns
	 * focus to the hamburger -- none of which a popover promises.
	 */
	let sheet = $state<HTMLDialogElement>();

	/*
	 * The sheet's Practice block is a heading, a switcher, and a divider
	 * above the pair. On a route scoped to the person rather than to a
	 * Practice (/account, #484) the switcher renders nothing, so the whole
	 * block goes rather than just the words (#673): a heading with nothing
	 * under it announces an empty section, and hiding only the label would
	 * leave the divider drawing a line under nothing. The condition is that
	 * there is a Practice to name, not that there is somewhere to switch to
	 * -- one Membership still puts her Practice's name under the heading.
	 */
	const hasPracticeToName = $derived(currentPracticeOf(practices, currentPracticeId) !== undefined);

	/*
	 * #1837: a screen can offer no section at all -- the agreement screen
	 * while an agreement is due (#1549), where each section would only send
	 * her back to it, and /account, which is scoped to the person. Then
	 * there is no nav to draw, wide or narrow, and no sheet to open: the
	 * one other thing the sheet carries, the Practice switcher, is already
	 * in the narrow bar itself, so a Menu button would open an empty
	 * landmark and a second copy of a control she can already see. GOV.UK's
	 * Service navigation draws no menu toggle when a service has no
	 * navigation links, for the same reason.
	 */
	const hasSections = $derived(navItems.length > 0);

	function openSheet() {
		sheet?.showModal();
	}

	function closeSheet() {
		sheet?.close();
	}
</script>

<!--
	The wide tree in reading order: the lockup and the account on the
	first row, the nav on the row under them, so Tab walks the bar the way
	the eye does.
-->
<header>
	<div class="wide">
		<div class="brand">
			<BrandLockup />
		</div>
		<div class="account">
			<PracticeSwitcher {practices} {currentPracticeId} />
			<AvatarMenu {firstName} {lastName} {email} {accountHref} {signOut} />
		</div>
		{#if hasSections}
			<nav aria-label="Practice">
				{#each navItems as item (item.href)}
					<Link href={item.href} label={item.label} variant="nav" current={item.current} />
				{/each}
			</nav>
		{/if}
	</div>

	<div class="narrow">
		{#if hasSections}
			<Button label="Menu" icon="list" iconOnly variant="bare" onClick={openSheet} />
		{:else if !hasPracticeToName}
			<!-- /account (#1837): no section and no Practice, so the room the
				Menu button and the switcher took holds the lockup, which the
				sheet used to be the only narrow place to show. -->
			<BrandLockup />
		{/if}
		<PracticeSwitcher {practices} {currentPracticeId} />
		<AvatarMenu {firstName} {lastName} {email} {accountHref} {signOut} />
	</div>
</header>

{#if hasSections}
	<dialog bind:this={sheet} class="sheet" aria-label="Practice menu">
		<div class="sheet-bar">
			<Button label="Close menu" icon="x" iconOnly variant="bare" onClick={closeSheet} />
			<BrandLockup />
			<AvatarMenu {firstName} {lastName} {email} {accountHref} {signOut} />
		</div>
		<nav aria-label="Practice">
			{#each navItems as item (item.href)}
				<Link href={item.href} label={item.label} variant="sheet" current={item.current} />
			{/each}
		</nav>
		{#if hasPracticeToName}
			<div class="sheet-switcher">
				<p class="sheet-switcher-label">Practice</p>
				<PracticeSwitcher {practices} {currentPracticeId} />
			</div>
		{/if}
	</dialog>
{/if}

<style>
	@layer components {
		/* The bar is a container, so the switch below reads the room the
		   bar itself has rather than the room the window has. It is named
		   because `body` is a containment context too (#540): an unnamed
		   query that failed to find this declaration would silently
		   resolve against the page and be a viewport query again, and no
		   test could tell.

		   A minimum, never a fixed height (#1573): the Practice's name in
		   the switcher is her own data of any length, and wrapping it is the
		   correct outcome (GLOSSARY.md's Content floor entry), so a long one
		   makes the bar taller. */
		header {
			container: staff-top-bar / inline-size;
			display: flex;
			align-items: center;
			min-block-size: var(--top-bar-height);
			padding-inline: var(--page-gutter);
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
			background-color: var(--color-surface-bright);
		}

		/* #280: a printed page (the Birth Plan's own print control, on the
		   Practice side) is the record, not this bar -- ADR-0024 rule 3
		   permits `print` as a stated user preference, so this is the one
		   media query this file owns. */
		@media print {
			header {
				display: none;
			}
		}

		/* The base size re-resolved against the bar (#544): a `cqi`
		   resolves against the nearest ANCESTOR container, so `header`
		   cannot answer its own, and text inside it would otherwise carry
		   the size computed for the page. */
		header > * {
			font-size: var(--text-body-size);
		}

		/* Stretched to the bar rather than a percentage of it, which against
		   a height the bar no longer fixes would resolve to auto. A
		   single-line flex container still stretches them to its minimum. */
		.wide,
		.narrow {
			align-self: stretch;
			align-items: center;
			inline-size: 100%;
		}

		/* Two rows (#1772): the lockup and the account on the first, the nav
		   alone on the second. On one row the ten sections an Owner sees
		   (942px of links at a 1280px window) left the account 74px of a
		   1216px bar, so the Practice's name was squeezed to a column one
		   word wide and the avatar was pushed 61px past the window's edge:
		   the page scrolled sideways. The six-section demo the floor below
		   was measured on never showed it. Its own row gives the nav the
		   whole width of the bar, and the name no longer competes with it. */
		.wide {
			display: none;
			grid-template-areas:
				'brand account'
				'nav nav';
			grid-template-columns: auto minmax(0, 1fr);
			column-gap: var(--space-10);
		}

		.narrow {
			display: flex;
			gap: var(--space-1);
			/* The Practice name takes the room the nav does not need here, so
			   the hamburger and the avatar stay pinned to the two edges. */
			justify-content: space-between;
		}

		/* Unavoidable (#564): the wide nav row and the menu-button sheet
		   are two landmarks, one always display:none, because a nav row
		   collapsing to a hamburger is a genuinely different DOM tree --
		   the same exception DataTable's own comment names, not the
		   ordinary case Every Layout's own objection to container
		   queries (#520) is about (rearranging one tree, which this
		   never does).

		   The content floor, re-measured 2026-09-01 in the canonical
		   environment (#564): the previous 60rem (960px) was never
		   measured against this failure at all, it was part of the
		   shared 60rem set (#523). A first measurement on 2026-08-31,
		   swept with `overflow-wrap` neutralized on
		   /style-guide/staff-top-bar's own demo, read six nav items plus
		   a Practice name and the account controls as stopping overflow
		   at 784px, 49rem. That number held on the machine that measured
		   it but read as insufficient on CI's own runner: the same font
		   bytes rasterize wider on CI's Linux/Chromium, so the bar needs
		   788px, not 784px, to stop overflowing. 49.25rem is that fixed
		   point, measured in CI's own Linux/Chromium, the one named
		   environment a floor's minimality is judged against (GLOSSARY.md's
		   Content floor entry), with no margin added beyond it. It is the
		   bar's own inline size that is measured, never a device width
		   (ADR-0024). Below the floor the same items are in the sheet,
		   which is why both trees are in the document and one is
		   display:none -- a hidden subtree is out of the accessibility
		   tree too, so nothing is announced twice.

		   Unchanged by #1573, which let the bar grow to a wrapped Practice
		   name: the floor is an inline measurement, and a taller bar needs
		   no more inline room than a fixed one did.

		   Re-measured 2026-10-03 (#1743): 49.25rem was taken while the
		   switcher's caret could shrink, and at 788px it gave up 15px of
		   its own 20px to make the row fit. With every icon held at the
		   size it declares, the same content needs more, and what it needs
		   grows with the room it is given, because the bar's text is sized
		   against the bar itself (#544). On macOS it fits at 808px; on
		   CI's Linux/Chromium it needed 811px at 808px. 50.75rem (812px) is
		   the next step of the sweep's 4px resolution, measured in CI, again
		   with no margin added beyond it.

		   Re-measured 2026-10-03 (#1747): 50.75rem was still taken with the
		   brand lockup shrunk. Its mark drew 34.9px of its 40, because
		   reset.css's percentage cap took the mark out of the lockup's
		   width, and the name "DoulaCloud" wrapped onto two lines in the
		   bar, at 1440px as at the floor. With the mark at its full width
		   and the name on one line, the wide tree needs 911px at 908px on
		   macOS; on CI's Linux/Chromium it needed 916px at 912px. 57.5rem
		   (920px) is the next step of the sweep's 4px resolution, measured
		   in CI, again with no margin added beyond it.

		   Re-measured 2026-10-24 (#1772), and the cause of that ticket's
		   sideways scroll named: every number above was taken on the
		   style-guide demo's six sections, while an Owner's bar has carried
		   ten since Schedule, On call and Contracts joined it. With ten,
		   the one-row tree needed 1297px at this 920px floor, so every
		   window from 920px to about 1500px drew the wide tree wider than
		   itself. The demo now holds the ten, and the wide tree is two rows,
		   so what stops fitting here is the ten sections on a row of their
		   own. On macOS they fit at 908px and need 908px at 904px; on CI's
		   Linux/Chromium they needed 912px at 908px. 57rem (912px) is the
		   next step of the sweep's 4px resolution, measured in CI, again
		   with no margin added beyond it. */
		@container staff-top-bar (min-width: 57rem) {
			.wide {
				display: grid;
			}

			.narrow {
				display: none;
			}
		}

		/* The first row is the bar's own 60px band at its smallest. */
		.brand {
			display: flex;
			grid-area: brand;
			align-items: center;
			min-block-size: var(--top-bar-height);
		}

		/* No gap: each item carries its own padding, and only one of them is
		   ever current, so two accent rules never meet. The canvas's 2px was
		   drawing the space between two boxes, which CSS gets from the
		   padding instead. The nav is the bar's last row, so the current
		   item's accent rule sits on the bar's own bottom edge whatever
		   height the first row took. */
		.wide nav {
			display: flex;
			grid-area: nav;
			min-block-size: var(--nav-row-height);
		}

		/* Stretched to its row, so the switcher's and the avatar's menus --
		   which stretch to the row holding them -- open below that row rather
		   than below a centered group (#1573), over the nav row the way a
		   menu opens over a page. */
		.account {
			display: flex;
			grid-area: account;
			align-items: center;
			align-self: stretch;
			justify-self: end;
			gap: var(--space-4);
			min-inline-size: 0;
		}

		/* The sheet is the whole screen, so the UA's default caps on a modal
		   dialog (max-inline-size: calc(100% - 6px - 2em)) have to go, and so
		   does the auto margin that centers it. */
		.sheet {
			inset: 0;
			inline-size: 100%;
			max-inline-size: 100%;
			block-size: 100%;
			max-block-size: 100%;
			margin: 0;
			padding: 0;
			border: 0;
			background-color: var(--color-surface-bright);
			color: var(--color-on-surface);
		}

		.sheet-bar {
			display: flex;
			align-items: center;
			justify-content: space-between;
			gap: var(--space-1);
			block-size: var(--top-bar-height);
			padding-inline: var(--space-2);
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
		}

		.sheet nav {
			display: flex;
			flex-direction: column;
			gap: 0;
			padding-block: var(--space-2);
		}

		/* A grid whose items start at the inline start, so the switcher's
		   menu box keeps its button's width rather than spanning the sheet:
		   its panel hangs off that box (#1573), and a full-width box would
		   hang it off the sheet's far edge. Not a column flex, where the
		   box's own `align-self: stretch` -- which makes it fill a top bar's
		   height -- would stretch it across the sheet instead. */
		.sheet-switcher {
			display: grid;
			justify-items: start;
			gap: var(--space-2);
			padding: var(--space-3) var(--space-5) 0;
			border-block-start: var(--border-thin) solid var(--color-outline-variant);
		}

		.sheet-switcher-label {
			margin: 0;
			color: var(--color-on-surface-muted);
			font-family: var(--font-family-base);
			font-size: var(--text-meta-size);
			letter-spacing: var(--text-meta-tracking);
			text-transform: uppercase;
		}
	}
</style>
