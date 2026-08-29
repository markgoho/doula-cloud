<script module lang="ts">
	export interface ControlProperties {
		id: string;
		describedBy: string | undefined;
		invalid: boolean;
	}
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';

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

{#snippet control()}
	{@render children({ id, describedBy, invalid: isInvalid })}
{/snippet}

{#snippet fieldLabel()}
	<label for={id}>{label}</label>
{/snippet}

<stack-l>
	{#if orientation === 'inline'}
		<cluster-l>
			{@render control()}
			{@render fieldLabel()}
		</cluster-l>
	{:else}
		{@render fieldLabel()}
		{@render control()}
	{/if}
	{#if error}
		<p id={errorId} role="alert">{error}</p>
	{/if}
</stack-l>

<style>
	@layer components {
		label {
			font-weight: var(--font-weight-medium);
			color: var(--color-on-surface);
		}

		p[role='alert'] {
			color: var(--color-error);
			font-size: var(--text-body-sm-size);
		}
	}
</style>
