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
			/* minmax(0, 1fr), not a bare 1fr, on the value track -- the same
			   fix as LabeledField's own inline-row (#510) for the identical
			   reason (ADR-0025's minimum-size note): a bare 1fr's automatic
			   minimum is its content's min-content size, so one unbreakable
			   value (a pasted URL, #530) refuses to shrink below that run's
			   width and pushes every ancestor, up to the page, wider with it. */
			grid-template-columns: auto minmax(0, 1fr);
			gap: 0 var(--space-4);
			margin: 0;
		}

		dt {
			font-weight: var(--font-weight-medium);
			color: var(--color-on-surface-variant);
			padding-block: var(--space-3);
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
		}

		dd {
			margin: 0;
			color: var(--color-on-surface);
			padding-block: var(--space-3);
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
			/* The companion to the shrinkable track above (#530): a shrinkable
			   track alone still lets a wide available space stretch the value
			   onto one very long line, so --measure (this repo's existing cap
			   on a readable line, DataTable's own answer to the same
			   question) bounds it, and overflow-wrap: anywhere lets a value
			   with no natural break -- a pasted URL -- actually shrink to fit
			   rather than sit at its min-content width regardless of the cap. */
			max-inline-size: var(--measure);
			overflow-wrap: anywhere;
		}
	}
</style>
