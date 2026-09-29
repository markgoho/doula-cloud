<script lang="ts">
	import { resolve } from '$app/paths';
	import Link from '#lib/components/atoms/Link.svelte';
	import AvatarMenu from '#lib/components/molecules/AvatarMenu.svelte';
	import type { NavItem } from './StaffTopBar.svelte';
	import type { SignOutOutcome } from '#lib/signOut.js';

	/*
	 * The Client portal's bar (#431, #452). Deliberately not the Staff
	 * answer: the Practice's name is the portal's identity rather than
	 * `Doula Cloud`, because a Client's relationship is with her doula's
	 * practice and not with the software it runs on. There is no Practice
	 * switcher -- a Client's own Client record belongs to exactly one
	 * Practice, so there is nothing to pick between the way Staff picks a
	 * Practice. What the Practice name is instead (#310) is a plain link
	 * to the portal root, the address a Client whose Portal Account
	 * reaches more than one Client (ADR-0015, one per Practice) uses to
	 * move between her Engagements -- a real address, not a dropdown that
	 * reimplements navigation.
	 *
	 * Narrow, the nav items become a full-width second row rather than a
	 * hamburger: a handful of items need no container, so the portal does
	 * not inherit the Staff sheet. That row wraps (#1568) rather than
	 * holding one line, because the count is not fixed -- Birth plan is
	 * conditional, and a fifth item, Notifications, already pushed a
	 * one-line row past a 320px screen.
	 */
	interface Properties {
		practiceName: string;
		/**
		 * The accessible name for the portal-root link above, built by
		 * `clientRegister.ts`'s `engagementLabel` from the same
		 * Practice name plus when this Engagement began -- the one function
		 * the root list and the login/accept-invite choosers also read
		 * (#310), so all of them agree. Kept off the visible text: the
		 * bar's own content floor (#564) is measured against `practiceName`
		 * alone, and adding visible text here would need a fresh measurement.
		 */
		switcherLabel: string;
		navItems: NavItem[];
		name: string;
		/**
		Where "Account" in the menu goes -- the Client portal's own account
		screen (#619's sign-in address), the same slot the Staff bar already
		fills. It belongs here rather than in `navItems`: the nav names the
		care she is receiving, an account setting is not one of those, and
		the measured floor below is what five destinations cost. Absent
		means the menu shows Sign out alone, which is what the bar did
		before there was such a screen.
		*/
		accountHref?: string;
		signOut: () => Promise<SignOutOutcome>;
	}

	let { practiceName, switcherLabel, navItems, name, accountHref, signOut }: Properties = $props();
</script>

<header>
	<div class="bar">
		<div class="brand-and-nav">
			<Link href={resolve('/(signed-out)')} label={practiceName} ariaLabel={switcherLabel} variant="brand" />
			<nav class="wide" aria-label="Your care">
				{#each navItems as item (item.href)}
					<Link href={item.href} label={item.label} variant="nav" current={item.current} />
				{/each}
			</nav>
		</div>
		<AvatarMenu {name} {accountHref} {signOut} />
	</div>
	<nav class="narrow" aria-label="Your care">
		{#each navItems as item (item.href)}
			<Link href={item.href} label={item.label} variant="nav" current={item.current} />
		{/each}
	</nav>
</header>

<style>
	@layer components {
		/* The bar is a container, so the switch below reads the room the
		   bar itself has rather than the room the window has -- it is the
		   element both navs are inside, so it is the one that can be asked.
		   Named because `body` is a containment context too (#540): an
		   unnamed query that failed to find this declaration would silently
		   resolve against the page and be a viewport query again, and no
		   test could tell. */
		header {
			container: portal-top-bar / inline-size;
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
			background-color: var(--color-surface-bright);
		}

		/* #280: the portal's own Birth Plan page already asks for a printed
		   sheet with no chrome (#306) -- this bar is the chrome, so it
		   never printed. ADR-0024 rule 3 permits `print` as a stated user
		   preference, so this is the one media query this file owns. */
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

		.bar {
			display: flex;
			align-items: center;
			justify-content: space-between;
			block-size: var(--top-bar-height);
			padding-inline: var(--page-gutter);
		}

		.brand-and-nav {
			display: flex;
			align-items: center;
			gap: var(--space-10);
			block-size: 100%;
		}

		nav {
			display: flex;
			block-size: 100%;
		}

		/* The same destinations, in the bar where there is room beside
		   the Practice's name and on their own row where there is not. One is
		   always display:none, so neither the tab order nor a screen reader
		   ever meets the pair. */
		.wide {
			display: none;
		}

		/* Wraps rather than scrolls (#1568): each item keeps its own label
		   on one line, so an item is what moves to the next line, never a
		   word. The row's height comes from its items, and each item keeps
		   the row's full height as its own target size, so a wrapped line
		   is as easy to hit as the first. On a wrapped row the current
		   item's accent rule sits under that item rather than on the bar's
		   own edge -- still the item's own marker, beside aria-current. */
		.narrow {
			flex-wrap: wrap;
			block-size: auto;
		}

		.narrow :global(a) {
			flex: 1 1 auto;
			justify-content: center;
			min-block-size: var(--nav-row-height);
			padding-inline: var(--space-2);
		}

		/* Unavoidable (#564): the same shape as StaffTopBar's own -- a wide
		   nav row and a narrow stacked row are two landmarks, one always
		   display:none, a different DOM tree rather than one tree
		   rearranged, which is the ordinary case an intrinsic mechanism
		   would otherwise handle.

		   The content floor, re-measured 2026-09-01 in the canonical
		   environment (#564): swept with `overflow-wrap` neutralized on
		   /style-guide/portal-top-bar's own demo -- the previous 48rem
		   (768px) was never measured against this failure at all, and a
		   first measurement on 2026-08-31 read the four portal nav items
		   (beside the Practice's name and the sign-out control) as
		   stopping overflow at 572px, 35.75rem. That number held on the
		   machine that measured it but read as insufficient on CI's own
		   runner: the same font bytes rasterize wider on CI's Linux/
		   Chromium, so the bar needs 584px, not 572px, to stop
		   overflowing. 36.5rem is that fixed point, measured in CI's own
		   Linux/Chromium, the one named environment a floor's minimality
		   is judged against (CONTEXT.md's Content floor entry), with no
		   margin added beyond it.

		   Measured again 2026-09-29 (#1568), because 36.5rem was what FOUR
		   items cost and the layout had passed five since Notifications
		   joined (#716): the demo now carries all five, and five need
		   702px, 43.875rem, on macOS/Chromium -- PENDING CI's Linux/Chromium
		   measurement, which replaces this sentence before merge.
		   It is the bar's own inline size that is measured, never a device
		   width (ADR-0024). Below the floor the
		   same items are in the narrow row, which is why both trees are
		   in the document and one is display:none. */
		@container portal-top-bar (min-width: 43.875rem) {
			.wide {
				display: flex;
			}

			.narrow {
				display: none;
			}
		}
	}
</style>
