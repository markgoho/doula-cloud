<script lang="ts">
	/*
	 * One group, so each box reads as the end of "Show only". FilterBar draws
	 * the legend on the boxes' own row. Consumers: the Schedule, the On call
	 * roster and the Clients list.
	 */
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();
</script>

<!-- eslint-disable-next-line svelte/no-restricted-html-elements -- a named box of a GET form beside the bar's own fields: CheckboxGroup stacks one option to a line and carries its value as an array -->
<fieldset>
	<legend>Show only</legend>
	<div class="show-only-boxes">
		{@render children()}
	</div>
</fieldset>

<style>
	@layer components {
		/* The boxes wrap among themselves beside FilterBar's floated legend. */
		.show-only-boxes {
			display: flex;
			flex-wrap: wrap;
			align-items: center;
			gap: var(--space-1) var(--space-related);
		}
	}
</style>
