<script lang="ts">
	interface Properties {
		label: string;
		variant: 'info' | 'success' | 'warning' | 'error' | 'neutral';
	}

	let { label, variant }: Properties = $props();
</script>

<span class={variant}>{label}</span>

<style>
	@layer components {
		span {
			display: inline-flex;
			align-items: center;
			padding: var(--space-1) var(--space-3);
			border: var(--border-thin) solid;
			border-radius: 999px;
			font-family: var(--font-family-base);
			font-size: var(--text-sm);
			font-weight: var(--font-weight-medium);
			line-height: 1.2;
		}

		span.info {
			color: var(--color-info);
			border-color: var(--color-info);
			background-color: color-mix(in oklch, var(--color-info) 12%, var(--color-bg));
		}

		/* --color-status is the shared green "positive" token (Notice's status
		   variant uses it too) -- no separate --color-success token needed. */
		span.success {
			color: var(--color-status);
			border-color: var(--color-status);
			background-color: color-mix(in oklch, var(--color-status) 12%, var(--color-bg));
		}

		span.warning {
			color: var(--color-warning);
			border-color: var(--color-warning);
			background-color: color-mix(in oklch, var(--color-warning) 12%, var(--color-bg));
		}

		span.error {
			color: var(--color-error);
			border-color: var(--color-error);
			background-color: color-mix(in oklch, var(--color-error) 12%, var(--color-bg));
		}

		span.neutral {
			color: var(--color-neutral);
			border-color: var(--color-neutral);
			background-color: color-mix(in oklch, var(--color-neutral) 12%, var(--color-bg));
		}
	}
</style>
