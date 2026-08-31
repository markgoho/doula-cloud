<script lang="ts">
	interface Properties {
		/**
		How many placeholder lines to draw. Match the number of rows the real content will have.
		*/
		lines?: number;
		/**
		`row` reserves a table row's height, `text` a line of body copy.
		*/
		variant?: 'row' | 'text';
		/**
		What is loading, announced politely. "Loading" alone tells a screen-reader user nothing.
		*/
		label: string;
	}

	let { lines = 3, variant = 'row', label }: Properties = $props();
</script>

<!--
	The brief's "loading is skeletal, not spinning, and only where content
	will actually appear". The point of a skeleton is not that it looks
	busy -- it is that it occupies the space the content is about to
	occupy, so nothing jumps when the data lands. That makes the height of
	these bars the whole feature, which is why `variant` is a closed set
	tied to the two densities the brief fixes (a 40px table row, a line of
	body copy) rather than a free-form height a caller can get wrong.

	Deliberately static: no shimmer, no pulse. The brief spends its motion
	budget on movement that "explains a change of state or a change of
	place", and a shimmer explains neither -- it is decoration on a
	surface whose entire job is to be ignored. See #418 and ADR-0020.
-->
<div role="status" aria-busy="true" aria-label={label} class={variant}>
	{#each { length: lines }, line (line)}
		<span aria-hidden="true"></span>
	{/each}
</div>

<style>
	@layer components {
		div {
			display: flex;
			flex-direction: column;
			gap: var(--space-2);
		}

		span {
			display: block;
			inline-size: 100%;
			background-color: var(--color-surface-container-high);
			border-radius: var(--radius);
		}

		/* 40px, the row height the brief's Density section fixes for a
		   table, with no gap between spans -- DataTable's own rows are
		   border-collapsed and adjacent, not gapped, and a placeholder
		   that reserves more than the real row costs is exactly the
		   layout shift this component exists to prevent. */
		.row {
			gap: 0;
		}

		.row span {
			block-size: 2.5rem;
		}

		/* A line of body copy, so a paragraph placeholder reserves the same
		   space the paragraph will. Narrower than full width on the last
		   line, because a real last line rarely reaches the margin. */
		.text span {
			block-size: var(--text-body-size);
		}

		.text span:last-child {
			inline-size: 60%;
		}
	}
</style>
