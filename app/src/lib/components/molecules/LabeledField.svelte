<script lang="ts">
	import type { Snippet } from 'svelte';

	interface ControlProperties {
		id: string;
		describedBy: string | undefined;
		invalid: boolean;
	}

	interface Properties {
		id?: string;
		label: string;
		error?: string;
		orientation?: 'stacked' | 'inline';
		children: Snippet<[ControlProperties]>;
	}

	const generatedId = $props.id();

	let { id = generatedId, label, error, orientation = 'stacked', children }: Properties = $props();

	const errorId = $derived(`${id}-error`);
	const isInvalid = $derived(Boolean(error));
	const describedBy = $derived(error ? errorId : undefined);
</script>

<stack-l space="var(--space-1)">
	{#if orientation === 'inline'}
		<cluster-l space="var(--space-2)" align="center">
			{@render children({ id, describedBy, invalid: isInvalid })}
			<label for={id}>{label}</label>
		</cluster-l>
	{:else}
		<label for={id}>{label}</label>
		{@render children({ id, describedBy, invalid: isInvalid })}
	{/if}
	{#if error}
		<p id={errorId} role="alert">{error}</p>
	{/if}
</stack-l>

<style>
	@layer components {
		label {
			font-weight: var(--font-weight-medium);
			color: var(--color-text);
		}

		p[role='alert'] {
			color: var(--color-error);
			font-size: var(--text-sm);
		}
	}
</style>
