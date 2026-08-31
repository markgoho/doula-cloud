<script lang="ts">
	/*
	 * FINISHED (#534). The answer to `ContactCardStart.svelte`. Diff the
	 * two files: everything the exercise teaches is in that diff, and
	 * nothing else in either file differs.
	 */
	interface Field {
		label: string;
		value: string;
	}

	interface Properties {
		fields: Field[];
	}

	let { fields }: Properties = $props();
</script>

<dl class="card">
	{#each fields as field, index (index)}
		<!--
			A `div` grouping a `dt` with its `dd` is valid inside a `dl`, and
			it is what lets the pair travel as one thing: the grid places
			pairs, and each pair decides for itself whether its label sits
			beside its value or above it.
		-->
		<div class="field">
			<dt>{field.label}</dt>
			<dd>{field.value}</dd>
		</div>
	{/each}
</dl>

<style>
	@layer components {
		.card {
			display: grid;
			/*
			 * A quantum layout (CONTEXT.md): `auto-fit` resolves how many
			 * columns exist from the room there is, so this card has many
			 * configurations rather than one and nobody authored the moments
			 * it changes between them.
			 *
			 * The content floor is 24ch: below about that, a value has too
			 * few characters per line to read as a value at all -- a street
			 * address starts wrapping every three words. `min(..., 100%)` is
			 * what keeps the floor from becoming an overflow of its own when
			 * the card is given less room than the floor asks for.
			 */
			grid-template-columns: repeat(auto-fit, minmax(min(24ch, 100%), 1fr));
			gap: var(--space-3) var(--space-4);
			margin: 0;
		}

		.field {
			display: grid;
			gap: var(--space-1);
			/*
			 * A grid item's automatic minimum size is its content, so without
			 * this the pair refuses to be narrower than its longest word and
			 * the floor above never gets to decide anything.
			 */
			min-inline-size: 0;
			padding-block-end: var(--space-3);
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
		}

		dt {
			font-weight: var(--font-weight-medium);
			color: var(--color-on-surface-variant);
		}

		dd {
			margin: 0;
			color: var(--color-on-surface);
			/*
			 * A pasted URL offers a browser no break opportunity it will
			 * take, so a floor alone cannot save it: the value has to be
			 * allowed to break mid-word. This is the half of the fix that is
			 * about the content rather than about the layout.
			 */
			overflow-wrap: anywhere;
		}
	}
</style>
