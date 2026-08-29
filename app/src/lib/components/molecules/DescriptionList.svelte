<script lang="ts">
	interface Item {
		label: string;
		value: string;
	}

	interface Properties {
		items: Item[];
	}

	let { items }: Properties = $props();
</script>

<dl>
	{#each items as item (item.label)}
		<dt>{item.label}</dt>
		<dd>{item.value}</dd>
	{/each}
</dl>

<style>
	@layer components {
		dl {
			display: grid;
			grid-template-columns: auto 1fr;
			gap: var(--space-1) var(--space-4);
			margin: 0;
		}

		dt {
			font-weight: var(--font-weight-medium);
			color: var(--color-on-surface-variant);
		}

		dd {
			margin: 0;
			color: var(--color-on-surface);
		}
	}
</style>
