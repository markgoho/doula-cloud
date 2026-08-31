<script lang="ts">
	/*
	 * START (#534). Broken on purpose -- do not fix this file. It is the
	 * one thing in the style guide that is meant to fail the continuum
	 * check, and it is not enumerated by that check or by the drag
	 * surface, because neither of them sees a `/style-guide/<slug>` route
	 * with no matching component under `src/lib/components`.
	 *
	 * The break is the one #530 measured on a real screen, reproduced at
	 * the smallest size that still shows it: a two-track grid where the
	 * value track's automatic minimum is the widest unbreakable thing in
	 * it, so a pasted URL pushes the whole card past its own edge and
	 * keeps pushing.
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
		<dt>{field.label}</dt>
		<dd>{field.value}</dd>
	{/each}
</dl>

<style>
	@layer components {
		.card {
			display: grid;
			/*
			 * One configuration at every available space: a label track as
			 * wide as its widest label, and a value track taking whatever is
			 * left. Nothing here is a content floor, so nothing ever switches.
			 */
			grid-template-columns: auto 1fr;
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
		}
	}
</style>
