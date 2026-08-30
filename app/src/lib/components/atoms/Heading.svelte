<script lang="ts">
	/*
	 * Two axes, two owners (#417). `level` is document structure -- it sets
	 * the screen-reader outline, and only the page's author knows it.
	 * `variant` is what the heading IS, and the design system decides how
	 * big that looks. The map below is the single place a size lives.
	 *
	 * The brief's `display` step is deliberately unreachable here: it is
	 * allowed on "the one page title on a hub", so the OverviewHub Template
	 * (#422) owns it. A one-per-app rule that depends on everyone
	 * remembering it is not a rule.
	 */
	interface Properties {
		level: 1 | 2 | 3 | 4 | 5 | 6;
		variant?: 'page' | 'section' | 'card';
		text: string;
		/**
		Lets a caller point an `aria-labelledby` at this heading -- RecordDetail's section landmarks (#507).
		*/
		id?: string;
	}

	let { level, variant, text, id }: Properties = $props();

	const fallbackByLevel = {
		1: 'page',
		2: 'section',
		3: 'card',
		4: 'card',
		5: 'card',
		6: 'card'
	} as const;
	const resolved = $derived(variant ?? fallbackByLevel[level]);
	const tag = $derived(`h${level}`);
	const classes = $derived(`variant-${resolved}`);
</script>

<svelte:element this={tag} {id} class={classes}>{text}</svelte:element>

<style>
	@layer components {
		.variant-page,
		.variant-section,
		.variant-card {
			margin: 0;
			font-family: var(--font-family-base);
			color: var(--color-on-surface);
		}

		.variant-page {
			font-size: var(--text-heading-lg-size);
			font-weight: var(--text-heading-lg-weight);
			line-height: var(--text-heading-lg-leading);
			letter-spacing: var(--text-heading-lg-tracking);
		}

		.variant-section {
			font-size: var(--text-heading-size);
			font-weight: var(--text-heading-weight);
			line-height: var(--text-heading-leading);
			letter-spacing: var(--text-heading-tracking);
		}

		.variant-card {
			font-size: var(--text-subheading-size);
			font-weight: var(--text-subheading-weight);
			line-height: var(--text-subheading-leading);
			letter-spacing: var(--text-subheading-tracking);
		}
	}
</style>
