<script lang="ts">
	import Icon from './Icon.svelte';
	import type { IconName } from './Icon/manifest.js';

	interface Properties {
		message: string;
		variant: 'error' | 'status' | 'info';
	}

	let { message, variant }: Properties = $props();

	// Matches the role="alert" (assertive) vs role="status" (polite) split
	// already in use ad hoc across the app -- errors interrupt, status/info don't.
	const role = $derived(variant === 'error' ? 'alert' : 'status');

	const iconByVariant: Record<Properties['variant'], IconName> = {
		error: 'warning',
		status: 'check',
		info: 'info'
	};
</script>

<p {role} class={variant}>
	<Icon name={iconByVariant[variant]} size={20} />
	{message}
</p>

<style>
	@layer components {
		p {
			display: flex;
			align-items: flex-start;
			gap: var(--space-2);
			margin: 0;
			padding: var(--space-3) var(--space-4);
			border: var(--border-thin) solid;
			border-radius: var(--radius);
			font-family: var(--font-family-base);
			font-size: var(--text-body-sm-size);
		}

		p.error {
			color: var(--color-error);
			border-color: var(--color-error);
			background-color: color-mix(in oklch, var(--color-error) 12%, var(--color-surface));
		}

		p.status {
			color: var(--color-status);
			border-color: var(--color-status);
			background-color: color-mix(in oklch, var(--color-status) 12%, var(--color-surface));
		}

		p.info {
			color: var(--color-info);
			border-color: var(--color-info);
			background-color: color-mix(in oklch, var(--color-info) 12%, var(--color-surface));
		}
	}
</style>
