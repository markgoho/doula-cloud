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
	}
</style>
