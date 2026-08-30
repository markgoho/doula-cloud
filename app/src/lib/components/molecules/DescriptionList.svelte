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
	<!--
		Keyed on index, not on the label (#464). A record can legitimately
		show the same label twice -- two phone numbers, two Practice-defined
		fields a Practice named alike -- and a duplicate key collapses both
		rows into one silently. The array is positional anyway: a row has no
		identity beyond where it sits.
	-->
	{#each items as item, index (index)}
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
