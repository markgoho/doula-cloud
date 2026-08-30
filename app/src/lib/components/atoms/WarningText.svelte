<script lang="ts">
	import Icon from './Icon.svelte';

	interface Properties {
		message: string;
	}

	let { message }: Properties = $props();
</script>

<!-- role="status" (polite), not "alert" -- this precedes an action rather
     than reporting a failure, so it should not interrupt like Notice's
     error variant does. -->
<p role="status">
	<Icon name="warning" size={20} />
	<span class="visually-hidden">Warning</span>
	{message}
</p>

<style>
	@layer components {
		p {
			display: flex;
			align-items: flex-start;
			gap: var(--space-2);
			margin: 0;
			font-family: var(--font-family-base);
			font-size: var(--text-body-sm-size);
			color: var(--color-on-surface);
		}

		/* tokens:ignore -- the WCAG clip technique's own geometry, matching
		   Button.svelte's, not a design value. */
		.visually-hidden {
			position: absolute;
			inline-size: 1px;
			block-size: 1px;
			margin: -1px;
			padding: 0;
			overflow: hidden;
			clip: rect(0, 0, 0, 0);
			white-space: nowrap;
			border: 0;
		}
	}
</style>
