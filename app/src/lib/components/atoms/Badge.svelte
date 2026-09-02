<script lang="ts">
	import Icon from './Icon.svelte';
	import type { IconName } from './Icon/manifest.js';

	interface Properties {
		label: string;
		variant: 'info' | 'success' | 'warning' | 'error' | 'neutral';
	}

	let { label, variant }: Properties = $props();

	const iconByVariant: Record<Properties['variant'], IconName> = {
		info: 'info',
		success: 'check',
		warning: 'warning',
		error: 'x',
		neutral: 'minus-circle'
	};
</script>

<span class={variant}>
	<Icon name={iconByVariant[variant]} size={16} />
	{label}
</span>

<style>
	@layer components {
		span {
			display: inline-flex;
			align-items: center;
			gap: var(--space-1);
			padding: var(--space-1) var(--space-3);
			border: var(--border-thin) solid;
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
			color: var(--color-info);
			border-color: var(--color-info);
			background-color: color-mix(in oklab, var(--color-info) 12%, var(--color-surface));
		}

		/* --color-status is the shared green "positive" token (Notice's status
		   variant uses it too) -- no separate --color-success token needed. */
		span.success {
			color: var(--color-status);
			border-color: var(--color-status);
			background-color: color-mix(in oklab, var(--color-status) 12%, var(--color-surface));
		}

		span.warning {
			color: var(--color-warning);
			border-color: var(--color-warning);
			background-color: color-mix(in oklab, var(--color-warning) 12%, var(--color-surface));
		}

		span.error {
			color: var(--color-error);
			border-color: var(--color-error);
			background-color: color-mix(in oklab, var(--color-error) 12%, var(--color-surface));
		}

		span.neutral {
			color: var(--color-neutral);
			border-color: var(--color-neutral);
			background-color: color-mix(in oklab, var(--color-neutral) 12%, var(--color-surface));
		}
	}
</style>
