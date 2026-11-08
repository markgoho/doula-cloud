<script lang="ts">
	/*
	 * The signed-out landing, `/`, its one consumer (#1645). Every word on
	 * it is fixed product copy and none is Practice content, so there is no
	 * hostile value to put in (ADR-0025); the longest greeting, the
	 * late-night one, is the one shown, and the switch below shows the
	 * others.
	 */
	import { PORTAL_DOOR_LINE } from '#lib/portalDoor.js';
	import LandingPage from '#lib/components/templates/LandingPage.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import DoorLink from '#lib/components/atoms/DoorLink.svelte';
	import Link from '#lib/components/atoms/Link.svelte';

	import { GREETINGS } from '#lib/greeting.js';

	const greetings = [GREETINGS.night, GREETINGS.morning, GREETINGS.afternoon, GREETINGS.evening];
	let index = $state(0);
	const greeting = $derived(greetings[index]);
</script>

{#snippet content()}
	<stack-l space="var(--space-6)">
		<grid-l min="var(--door-min)" space="var(--space-4)">
			<DoorLink
				href="/style-guide/landing-page"
				label="Staff sign in"
				description="For doulas, practice owners, and office staff."
			/>
			<DoorLink
				href="/style-guide/landing-page"
				label="Client sign in"
				description={PORTAL_DOOR_LINE}
			/>
		</grid-l>
		<p class="new-here">
			New here? <Link href="/style-guide/landing-page" label="Set up a Practice" />. Your first
			three clients are free.
		</p>
	</stack-l>
{/snippet}

<div class="controls">
	<Button
		label="Show the next greeting"
		variant="secondary"
		size="sm"
		onClick={() => (index = (index + 1) % greetings.length)}
	/>
</div>

<LandingPage
	title="Sign in"
	{greeting}
	lede="Welcome to DoulaCloud."
	{content}
/>

<style>
	@layer components {
		/* Not part of the Template -- a switch so each greeting can be seen
		   without editing this file. */
		.new-here {
			margin: 0;
			color: var(--color-on-surface-variant);
		}

		.controls {
			display: flex;
			gap: var(--space-3);
			padding: var(--space-3) var(--space-4);
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
			background-color: var(--color-surface-container);
		}
	}
</style>
