<script lang="ts">
	/*
	 * The signed-out landing, `/`, its one consumer (#1645). Every word on
	 * it is fixed product copy and none is Practice content, so there is no
	 * hostile value to put in (ADR-0025); the longest greeting, the
	 * late-night one, is the one shown, and the switch below shows the
	 * others.
	 */
	import LandingPage from '#lib/components/templates/LandingPage.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import DoorLink from '#lib/components/atoms/DoorLink.svelte';

	import { GREETINGS } from '#lib/greeting.js';

	const greetings = [GREETINGS.night, GREETINGS.morning, GREETINGS.afternoon, GREETINGS.evening];
	let index = $state(0);
	const greeting = $derived(greetings[index]);
</script>

{#snippet content()}
	<stack-l space="var(--space-4)">
		<DoorLink
			href="/style-guide/landing-page"
			label="Staff log in"
			description="For doulas and practice owners."
			variant="primary"
		/>
		<grid-l min="var(--door-min)" space="var(--space-4)">
			<DoorLink
				href="/style-guide/landing-page"
				label="Client portal log in"
				description="Your contract, your birth plan, and your invoices."
			/>
			<DoorLink
				href="/style-guide/landing-page"
				label="Set up a Practice"
				description="New here? Your first three clients are free."
			/>
		</grid-l>
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
	title="Sign in or set up a Practice"
	{greeting}
	lede="Welcome to DoulaCloud."
	{content}
/>

<style>
	@layer components {
		/* Not part of the Template -- a switch so each greeting can be seen
		   without editing this file. */
		.controls {
			display: flex;
			gap: var(--space-3);
			padding: var(--space-3) var(--space-4);
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
			background-color: var(--color-surface-container);
		}
	}
</style>
