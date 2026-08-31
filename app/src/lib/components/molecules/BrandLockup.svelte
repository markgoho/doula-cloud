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
			/* The mark is optically centred on the wordmark's x-height, not
			   its box, so the pair does not need a baseline nudge. */
			line-height: 1.3;
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
