<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';

	// GOV.UK's text input: the label, an optional hint, the error above the
	// control, each tied to it by aria-describedby.
	let {
		id,
		label,
		hint,
		error,
		value = $bindable(''),
		...attributes
	}: {
		id: string;
		label: string;
		hint?: string;
		error?: string;
		value?: string;
	} & Omit<HTMLInputAttributes, 'id' | 'value'> = $props();

	const describedBy = $derived(
		[hint ? `${id}-hint` : undefined, error ? `${id}-error` : undefined].filter(Boolean).join(' ') || undefined
	);
</script>

<div class="field" class:invalid={error}>
	<label for={id}>{label}</label>
	{#if hint}
		<p class="hint" id={`${id}-hint`}>{hint}</p>
	{/if}
	{#if error}
		<p class="error" id={`${id}-error`}>{error}</p>
	{/if}
	<input {id} bind:value aria-describedby={describedBy} aria-invalid={error ? 'true' : undefined} {...attributes} />
</div>

<style>
	.field {
		display: flex;
		flex-direction: column;
		gap: var(--space-label);
	}

	.field.invalid {
		padding-inline-start: var(--space-3);
		border-inline-start: var(--border-active) solid var(--color-error);
	}

	label {
		font-weight: bold;
	}

	.hint {
		color: var(--color-on-surface-variant);
	}

	.error {
		color: var(--color-error);
		font-weight: bold;
	}

	input {
		inline-size: 100%;
		max-inline-size: var(--measure);
		min-block-size: var(--control-height);
		padding: var(--space-2) var(--space-3);
		color: var(--color-on-surface);
		background-color: var(--color-surface);
		border: var(--border-thin) solid var(--color-outline);
		border-radius: var(--radius);
	}

	.invalid input {
		border-color: var(--color-error);
	}

	input:focus-visible {
		outline: var(--focus-ring-width) solid var(--color-primary);
		outline-offset: var(--focus-ring-offset);
	}
</style>
