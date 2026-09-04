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
			/* The label track is `fit-content(40%)`, not `auto` (#725). A
			   Practice types its own field labels, so a label is free text
			   like the value beside it -- and a bare `auto` track sizes to
			   its max-content and is grown to that limit before the `1fr`
			   beside it sees any space at all, so one pasted URL as a label
			   takes the whole row and leaves the value nothing. 40% is a
			   share of whatever space this list is given rather than a
			   width: it names no space, it says how much of a row a key may
			   take from its own value, and it is inert for every ordinary
			   label, which sizes to its own text well under the cap. */
			grid-template-columns: fit-content(40%) minmax(0, 1fr);
			gap: 0 var(--space-4);
			margin: 0;
		}

		dt {
			font-weight: var(--font-weight-medium);
			color: var(--color-on-surface-variant);
			padding-block: var(--space-3);
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
			/* The same pairing the value already carries, for the same
			   reason one row down (#725). Every caller but one passes a
			   short developer-chosen label, but a Client Field Template's
			   own label is a question a Practice types, so it can be a
			   pasted URL like any other free text -- and the label track is
			   `auto`, whose minimum is the item's min-content size, so a run
			   with no break opportunity pushes the whole page wider. The
			   cap lives on the track above rather than here, since a
			   `max-inline-size` on the item leaves the track free to grow
			   past it; this half is what lets the label shrink to the cap
			   at all. */
			overflow-wrap: anywhere;
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
