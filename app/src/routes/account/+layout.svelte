<script lang="ts">
	/*
	 * Navigation chrome for the account route, not Template content or page
	 * content (#474). The account route sits above the Practice-scoped tree
	 * and inherits only the bare root layout, so it is the one authenticated
	 * Staff screen with no nav of its own -- this supplies exactly that, and
	 * nothing else `StaffTopBar` already carries (no top bar, no sign-out:
	 * out of scope for #474).
	 *
	 * loadAccountSession() memoizes the fetch this layout and +page.svelte
	 * both need, so mounting together on every visit to /account costs one
	 * request, not two.
	 */
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import type { Membership } from '#lib/landing.js';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import { loadAccountSession } from './session.svelte.js';

	let { children } = $props();

	let memberships = $state<Membership[]>([]);
	let isLoaded = $state(false);

	onMount(async () => {
		const result = await loadAccountSession();
		if (!result.ok) return;

		memberships = result.session.memberships;
		isLoaded = true;
	});
</script>

{@render children()}

{#if isLoaded}
	<!--
		A way back. The session response already carries every Practice she
		belongs to, so this screen can return her to the one she came from
		without a second read -- and a top-level route outside the Practice
		layout would otherwise be a place with no exit but the back button.
		One Practice, one link; several, several.
	-->
	<nav aria-label="Your practices">
		<Heading level={2} variant="section" text="Back to your practices" />
		<ul>
			{#each memberships as membership (membership.practiceId)}
				<li>
					<Link
						href={resolve('/practices/[practiceId]', { practiceId: membership.practiceId })}
						label={membership.practiceName}
					/>
				</li>
			{/each}
		</ul>
	</nav>
{/if}

<style>
	@layer components {
		ul {
			margin: 0;
			padding: 0;
			list-style: none;
			display: flex;
			flex-wrap: wrap;
			gap: var(--space-3);
		}

		nav {
			padding: var(--space-6) var(--page-gutter);
		}
	}
</style>
