<script lang="ts">
	/*
	 * Type step and color tone are separate axes on purpose (#417): a
	 * timestamp is `meta` because of what it IS, and quiet because of how
	 * much it matters -- two decisions that do not always move together.
	 *
	 * Both sets are closed. A route names a purpose; it never reaches past
	 * this atom for a raw --text-* token, which is what keeps the brief's
	 * scale editable in one place.
	 */
	interface Properties {
		text: string;
		step?: 'body' | 'body-sm' | 'label' | 'meta';
		tone?: 'default' | 'variant' | 'muted';
		/**
		 * Lets a caller join this paragraph to a control's `aria-describedby`
		 * (#257) -- e.g. explaining why a disabled button is disabled.
		 * Undefined by default: most callers render standalone prose no
		 * control needs to reference.
		 */
		id?: string;
	}

	let { text, step = 'body', tone = 'default', id }: Properties = $props();

	const classes = $derived(`step-${step} tone-${tone}`);
</script>

<p {id} class={classes}>{text}</p>

<style>
	@layer components {
		/*
		 * Capped at --measure, flush left, wherever it sits (#2020). It was
		 * an opt-in until then (#609), and the screens that forgot it ran a
		 * sentence to 1,260px: Contracts' void-requests line and Staff's
		 * work-state line, both in a list's uncapped content. A paragraph
		 * is read as prose in any column, and a cap never widens a column
		 * narrower than it.
		 */
		p {
			margin: 0;
			max-inline-size: var(--measure);
			font-family: var(--font-family-base);
		}

		/* A table cell opts out: its width is the column's, which the
		   table sizes against every row, and a cap on one cell's text
		   would wrap it short of the column it sits in. */
		:global(:is(td, th)) p {
			max-inline-size: none;
		}

		.step-body {
			font-size: var(--text-body-size);
			font-weight: var(--text-body-weight);
			line-height: var(--text-body-leading);
			letter-spacing: var(--text-body-tracking);
		}

		.step-body-sm {
			font-size: var(--text-body-sm-size);
			font-weight: var(--text-body-sm-weight);
			line-height: var(--text-body-sm-leading);
			letter-spacing: var(--text-body-sm-tracking);
		}

		.step-label {
			font-size: var(--text-label-size);
			font-weight: var(--text-label-weight);
			line-height: var(--text-label-leading);
			letter-spacing: var(--text-label-tracking);
		}

		.step-meta {
			font-size: var(--text-meta-size);
			font-weight: var(--text-meta-weight);
			line-height: var(--text-meta-leading);
			letter-spacing: var(--text-meta-tracking);
		}

		.tone-default {
			color: var(--color-on-surface);
		}

		.tone-variant {
			color: var(--color-on-surface-variant);
		}

		.tone-muted {
			color: var(--color-on-surface-muted);
		}
	}
</style>
