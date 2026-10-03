<script lang="ts">
	import CloudMark from '#lib/components/atoms/CloudMark.svelte';

	/*
	 * The product's name, drawn once. Every bar that says who we are renders
	 * this rather than its own mark-and-words pair, so #338 can settle
	 * `Doula Cloud` versus `DoulaCloud` by editing one string. This ticket
	 * ships the current two-word form and decides nothing about it.
	 */
	interface Properties {
		size?: 'sm' | 'md' | 'lg';
	}

	let { size = 'sm' }: Properties = $props();

	const WORDMARK = 'Doula Cloud';
	// Built here rather than interpolated in the attribute, the way Heading
	// and Text already do it: Svelte compiles `class="lockup size-{size}"`
	// into a nullish check whose second arm a defaulted prop can never
	// reach, which the coverage gate then reports forever.
	const classes = $derived(`lockup size-${size}`);
</script>

<span class={classes}>
	<CloudMark {size} />
	<span class="wordmark">{WORDMARK}</span>
</span>

<style>
	@layer components {
		.lockup {
			display: inline-flex;
			align-items: center;
			gap: var(--space-3);
			color: var(--color-on-surface);
			font-family: var(--font-family-base);
			font-weight: var(--font-weight-semibold);
			/* The mark is optically centered on the wordmark's x-height, not
			   its box, so the pair does not need a baseline nudge. */
			line-height: 1.3;
		}

		/* The name is drawn as one unit, like the mark beside it (#1747).
		   Free to wrap, it broke onto two lines whenever the row holding
		   the lockup was short of room, so StaffTopBar's floor was measured
		   with a two-line name in the bar. Kept on one line, the lockup's
		   whole width is its min-content width, and a row that cannot hold
		   it overflows where the floor check can see it. */
		.wordmark {
			white-space: nowrap;
		}

		/* The one size whose mark and name together are wider than the
		   320px commitment: 120px of mark, the gap, and about 193px of name
		   at the display step. Where the row cannot hold both, the name
		   goes under the mark, whole, rather than either one shrinking.
		   Only `lg`: `sm` and `md` fit at 320px, and a bar holding `sm`
		   must overflow where it is short, not grow a second line the
		   floor check cannot see. */
		.size-lg {
			flex-wrap: wrap;
		}

		/* The canvas drew 17px, which is between two steps of the brief's
		   scale. The scale wins, the way the page frame's token won over
		   the drawing's 1360px on #424 -- an off-scale size is fine-tuning,
		   and the point of a closed type scale is that it does not grow one
		   step per drawing. */
		.size-sm .wordmark {
			font-size: var(--text-subheading-size);
			letter-spacing: var(--text-subheading-tracking);
		}

		.size-md .wordmark {
			font-size: var(--text-heading-size);
			letter-spacing: var(--text-heading-tracking);
		}

		.size-lg .wordmark {
			font-size: var(--text-display-size);
			letter-spacing: var(--text-display-tracking);
		}
	}
</style>
