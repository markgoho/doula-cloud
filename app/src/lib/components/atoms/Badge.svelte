<script lang="ts">
	import Icon from './Icon.svelte';
	import type { IconName } from './Icon/manifest.js';

	/*
	 * Each color means one thing, on every screen:
	 * - warning (amber): needs someone to act. Owing money is one such thing;
	 *   so are onboarding left unfinished, payouts on hold and a gap in cover.
	 * - error (red): late or failed. An overdue Invoice is late; an email that
	 *   could not be delivered and a collection that failed are failures.
	 * - info (blue): pending, or something to know; nobody has to act yet.
	 * - success (green): done and well.
	 * - neutral: settled, or no state worth a color.
	 * A new badge picks the color by that meaning, never by how loud it should look.
	 */
	interface Properties {
		label: string;
		variant: 'info' | 'success' | 'warning' | 'error' | 'neutral';
	}

	let { label, variant }: Properties = $props();

	const iconByVariant: Record<Properties['variant'], IconName> = {
		info: 'info',
		success: 'check',
		warning: 'warning',
		error: 'warning-circle',
		neutral: 'minus-circle'
	};
</script>

<span class={variant}>
	<Icon name={iconByVariant[variant]} size={16} />
	{label}
</span>

<style>
	@layer components {
		/* The intrinsic-width rule `Button.svelte` explains in full, for the
		   same reason: a badge is a pill around one short status word, and a
		   pill stretched across a whole row is not a pill. */
		:global(stack-l) > span {
			align-self: start;
		}

		span {
			display: inline-flex;
			align-items: center;
			gap: var(--space-1);
			/* Half a step of vertical padding, so a pill stands no taller
			   than a line of body text and a table row with a badge keeps
			   the rhythm of the rows around it. */
			padding: calc(var(--space-1) / 2) var(--space-3);
			border: var(--border-thin) solid transparent;
			border-radius: var(--radius-pill);
			font-family: var(--font-family-base);
			font-size: var(--text-body-sm-size);
			font-weight: var(--font-weight-medium);
			line-height: 1.2;
		}

		/* Mixed in oklab, not oklch -- #434. oklch interpolates hue along the
		   shorter polar arc, so a 12% mix drags every variant toward
		   --color-surface's own hue instead of its own. oklab has no hue
		   angle to spiral on, so each variant's tint stays distinct. */
		span.info {
			--badge-hue: var(--color-info);
		}

		/* --color-status is the shared green "positive" token (Notice's status
		   variant uses it too) -- no separate --color-success token needed. */
		span.success {
			--badge-hue: var(--color-status);
		}

		span.warning {
			--badge-hue: var(--color-warning);
		}

		span.error {
			--badge-hue: var(--color-error);
		}

		span.neutral {
			--badge-hue: var(--color-neutral);
		}

		/* Quiet and filled, as Stripe's are: a tint of the meaning's own
		   hue, text a step deeper toward the ink, and no outline (the
		   transparent border only keeps the box the size it was). One
		   meaning per color app-wide: amber is owes, or a warning that needs
		   someone to act; red is overdue or an error; blue is information,
		   and anything pending or waiting on someone else; green is
		   success; gray is neutral. */
		span {
			color: color-mix(in oklab, var(--badge-hue) 78%, var(--color-on-surface));
			background-color: color-mix(in oklab, var(--badge-hue) 20%, var(--color-surface-bright));
		}
	}
</style>
