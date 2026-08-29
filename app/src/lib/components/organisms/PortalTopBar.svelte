<script lang="ts">
	import Link from '#lib/components/atoms/Link.svelte';
	import AvatarMenu from '#lib/components/molecules/AvatarMenu.svelte';
	import type { NavItem } from './StaffTopBar.svelte';
	import type { SignOutOutcome } from '#lib/signOut.js';

	/*
	 * The Client portal's bar (#431, #452). Deliberately not the Staff
	 * answer: the Practice's name is the portal's identity rather than
	 * `Doula Cloud`, because a Client's relationship is with her doula's
	 * practice and not with the software it runs on. There is no Practice
	 * switcher -- a Client belongs to exactly one Practice.
	 *
	 * Narrow, the four nav items become a full-width second row rather than
	 * a hamburger: four items need no container, so the portal does not
	 * inherit the Staff sheet.
	 */
	interface Properties {
		practiceName: string;
		navItems: NavItem[];
		name: string;
		signOut: () => Promise<SignOutOutcome>;
	}

	let { practiceName, navItems, name, signOut }: Properties = $props();
</script>

<header>
	<div class="bar">
		<div class="brand-and-nav">
			<p class="practice">{practiceName}</p>
			<nav class="wide" aria-label="Your care">
				{#each navItems as item (item.href)}
					<Link href={item.href} label={item.label} variant="nav" current={item.current} />
				{/each}
			</nav>
		</div>
		<AvatarMenu {name} {signOut} />
	</div>
	<nav class="narrow" aria-label="Your care">
		{#each navItems as item (item.href)}
			<Link href={item.href} label={item.label} variant="nav" current={item.current} />
		{/each}
	</nav>
</header>

<style>
	@layer components {
		header {
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
			background-color: var(--color-surface-bright);
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

		.practice {
			margin: 0;
			color: var(--color-on-surface);
			font-family: var(--font-family-base);
			font-size: var(--text-subheading-size);
			font-weight: var(--font-weight-semibold);
		}

		nav {
			display: flex;
			block-size: 100%;
		}

		/* The same four destinations, in the bar where there is room beside
		   the Practice's name and on their own row where there is not. One is
		   always display:none, so neither the tab order nor a screen reader
		   ever meets the pair. */
		.wide {
			display: none;
		}

		.narrow {
			block-size: var(--nav-row-height);
		}

		.narrow :global(a) {
			flex: 1;
			justify-content: center;
			padding-inline: var(--space-2);
		}

		@media (min-width: 48rem) {
			.wide {
				display: flex;
			}

			.narrow {
				display: none;
			}
		}
	}
</style>
