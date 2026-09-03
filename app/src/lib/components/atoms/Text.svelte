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
		 * Caps at --measure, flush left (#609). Opt-in: most callers sit in
		 * a column already sized for the job (--form-max, --page-max, a
		 * table cell), and a default cap would narrow those uninvited. Set
		 * this only where the text is read as continuous prose in a column
		 * wider than --measure -- a lede, an intro.
		 */
		measure?: boolean;
	}

	let { text, step = 'body', tone = 'default', measure = false }: Properties = $props();

	const classes = $derived(`step-${step} tone-${tone}${measure ? ' measure' : ''}`);
</script>

<p class={classes}>{text}</p>

<style>
	@layer components {
		p {
			margin: 0;
			font-family: var(--font-family-base);
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

		.measure {
			max-inline-size: var(--measure);
		}
	}
</style>
