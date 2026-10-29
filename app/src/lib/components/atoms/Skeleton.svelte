<script lang="ts">
	interface Properties {
		/**
		How many placeholder lines to draw. Match the number of rows the real content will have.
		*/
		lines?: number;
		/**
		`row` reserves a table row's height, `text` a line of body copy, `heading` a page title's line.
		*/
		variant?: 'row' | 'text' | 'heading';
		/**
		What is loading, announced politely. "Loading" alone tells a screen-reader user nothing.
		Left out only for a second region of one load whose first Skeleton already announces it,
		such as a hub's rail beside its main column (#2248): unlabeled, it is hidden from
		assistive technology rather than a second status.
		*/
		label?: string;
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

	The bars show only after `--loading-delay` (#2248). The space is held
	and the status announced from the first frame, but a load that lands
	inside the Doherty budget never flashes a placeholder.
-->
<div
	role={label ? 'status' : undefined}
	aria-busy={label ? 'true' : undefined}
	aria-label={label}
	aria-hidden={label ? undefined : 'true'}
	class={variant}
>
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
			/* motion:ignore -- a reveal held back by `--loading-delay`, not
			   movement: nothing moves and the duration is zero, so none of
			   the three motion tokens describes it, and it stays under
			   prefers-reduced-motion: reduce. */
			animation: skeleton-appears 0s var(--loading-delay) backwards;
		}

		/* motion:ignore -- the bars' held-back reveal above. It changes
		   opacity once and moves nothing. */
		@keyframes skeleton-appears {
			from {
				opacity: 0;
			}
		}

		/* `--row-height`, the token DataTable's own rows read: 40px with a
		   fine pointer and 44px when any input can tap (#1843). No gap
		   between spans -- DataTable's own rows are
		   border-collapsed and adjacent, not gapped, and a placeholder
		   that reserves more than the real row costs is exactly the
		   layout shift this component exists to prevent. */
		.row {
			gap: 0;
		}

		/* Each row drawn as DataTable draws one (#2248): its rule, and a
		   line of copy inside it, rather than a solid slab that reads as
		   no table at all. Border-box, so the rule is inside the row's
		   height as it is in the table's. */
		.row span {
			display: flex;
			align-items: center;
			block-size: var(--row-height);
			background-color: transparent;
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
			border-radius: 0;
		}

		.row span::before {
			content: '';
			block-size: var(--text-body-size);
			inline-size: 70%;
			background-color: var(--color-surface-container-high);
			border-radius: var(--radius);
		}

		.row span:nth-child(3n + 2)::before {
			inline-size: 50%;
		}

		.row span:nth-child(3n)::before {
			inline-size: 60%;
		}

		/* A line of body copy, so a paragraph placeholder reserves the same
		   space the paragraph will: each line is a real line box, its
		   leading and its baseline included, with the bar the height of the
		   type sitting on it (#2255). A bar with no line box held a
		   one-line value in a `SummaryList` short of the line its figure
		   took, and a row aligned on its baseline had none to align to.
		   Narrower than full width on the last line, because a real last
		   line rarely reaches the margin. */
		.text {
			gap: 0;
		}

		.text span {
			font-size: var(--text-body-size);
			line-height: var(--text-body-leading);
			background-color: transparent;
		}

		.text span::before {
			content: '';
			display: inline-block;
			vertical-align: text-bottom;
			block-size: var(--text-body-size);
			inline-size: 100%;
			background-color: var(--color-surface-container-high);
			border-radius: var(--radius);
		}

		.text span:last-child::before {
			inline-size: 60%;
		}

		/* One line of `Heading`'s `page` variant, so a title that is the
		   record's own data holds the line it will take. */
		.heading span {
			block-size: calc(var(--text-heading-lg-size) * var(--text-heading-lg-leading));
			inline-size: 40%;
		}
	}
</style>
