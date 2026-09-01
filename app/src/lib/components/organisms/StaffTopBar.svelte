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
		type PracticeOption
	} from '#lib/components/molecules/PracticeSwitcher.svelte';
	import type { SignOutOutcome } from '#lib/signOut.js';

	/*
	 * The Staff shell's bar (#431, #452). A 60px band that does not grow:
	 * the lockup, a six-item flat nav, then the Practice switcher and the
	 * avatar at the far end.
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
		name: string;
		email?: string;
		accountHref?: string;
		signOut: () => Promise<SignOutOutcome>;
	}

	let {
		navItems,
		practices,
		currentPracticeId,
		name,
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

	function openSheet() {
		sheet?.showModal();
	}

	function closeSheet() {
		sheet?.close();
	}
</script>

<header>
	<div class="wide">
		<div class="brand-and-nav">
			<BrandLockup />
			<nav aria-label="Practice">
				{#each navItems as item (item.href)}
					<Link href={item.href} label={item.label} variant="nav" current={item.current} />
				{/each}
			</nav>
		</div>
		<div class="account">
			<PracticeSwitcher {practices} {currentPracticeId} />
			<AvatarMenu {name} {email} {accountHref} {signOut} />
		</div>
	</div>

	<div class="narrow">
		<Button label="Menu" icon="list" iconOnly variant="bare" onClick={openSheet} />
		<PracticeSwitcher {practices} {currentPracticeId} />
		<AvatarMenu {name} {email} {accountHref} {signOut} />
	</div>
</header>

<dialog bind:this={sheet} class="sheet" aria-label="Practice menu">
	<div class="sheet-bar">
		<Button label="Close menu" icon="x" iconOnly variant="bare" onClick={closeSheet} />
		<BrandLockup />
		<AvatarMenu {name} {email} {accountHref} {signOut} />
	</div>
	<nav aria-label="Practice">
		{#each navItems as item (item.href)}
			<Link href={item.href} label={item.label} variant="sheet" current={item.current} />
		{/each}
	</nav>
	<div class="sheet-switcher">
		<p class="sheet-switcher-label">Practice</p>
		<PracticeSwitcher {practices} {currentPracticeId} />
	</div>
</dialog>

<style>
	@layer components {
		/* The bar is a container, so the switch below reads the room the
		   bar itself has rather than the room the window has. It is named
		   because `body` is a containment context too (#540): an unnamed
		   query that failed to find this declaration would silently
		   resolve against the page and be a viewport query again, and no
		   test could tell. */
		header {
			container: staff-top-bar / inline-size;
			display: flex;
			align-items: center;
			block-size: var(--top-bar-height);
			padding-inline: var(--page-gutter);
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
			background-color: var(--color-surface-bright);
		}

		/* The base size re-resolved against the bar (#544): a `cqi`
		   resolves against the nearest ANCESTOR container, so `header`
		   cannot answer its own, and text inside it would otherwise carry
		   the size computed for the page. */
		header > * {
			font-size: var(--text-body-size);
		}

		.wide,
		.narrow {
			align-items: center;
			inline-size: 100%;
			block-size: 100%;
		}

		.wide {
			display: none;
			justify-content: space-between;
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
		   environment a floor's minimality is judged against (CONTEXT.md's
		   Content floor entry), with no margin added beyond it. It is the
		   bar's own inline size that is measured, never a device width
		   (ADR-0024). Below the floor the same items are in the sheet,
		   which is why both trees are in the document and one is
		   display:none -- a hidden subtree is out of the accessibility
		   tree too, so nothing is announced twice. */
		@container staff-top-bar (min-width: 49.25rem) {
			.wide {
				display: flex;
			}

			.narrow {
				display: none;
			}
		}

		.brand-and-nav {
			display: flex;
			align-items: center;
			gap: var(--space-10);
			block-size: 100%;
		}

		/* No gap: each item carries its own padding, and only one of them is
		   ever current, so two accent rules never meet. The canvas's 2px was
		   drawing the space between two boxes, which CSS gets from the
		   padding instead. */
		nav {
			display: flex;
			block-size: 100%;
		}

		.account {
			display: flex;
			align-items: center;
			gap: var(--space-4);
		}

		/* The sheet is the whole screen, so the UA's default caps on a modal
		   dialog (max-inline-size: calc(100% - 6px - 2em)) have to go, and so
		   does the auto margin that centres it. */
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
			block-size: auto;
			padding-block: var(--space-2);
		}

		.sheet-switcher {
			display: flex;
			flex-direction: column;
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
