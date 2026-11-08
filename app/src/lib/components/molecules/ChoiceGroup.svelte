<script lang="ts">
	/*
	 * The question around a group of choices: GOV.UK's fieldset, legend,
	 * hint and one refusal for the whole group. `RadioGroup` and
	 * `CheckboxGroup` put their options inside it, so the two kinds of
	 * question on one form keep one rhythm.
	 */
	import type { Snippet } from 'svelte';

	interface Properties {
		legend?: string;
		hint?: string;
		error?: string;
		name: string;
		children: Snippet;
	}

	let { legend, hint, error, name, children }: Properties = $props();

	const errorId = $derived(`${name}-error`);
	const hintId = $derived(`${name}-hint`);
	const describedBy = $derived([hint ? hintId : undefined, error ? errorId : undefined].filter(Boolean).join(' ') || undefined);
</script>

{#snippet errorMessage()}
	{#if error}
		<p id={errorId} class="error" role="alert">{error}</p>
	{/if}
{/snippet}

{#if legend === undefined}
	{@render errorMessage()}
	{@render children()}
{:else}
	<!-- aria-describedby only: aria-invalid is not supported on role="group",
	     which <fieldset> carries implicitly. The refusal reaches the reader
	     through the description and through role="alert". -->
	<fieldset aria-describedby={describedBy}>
		<legend>{legend}</legend>
		{#if hint}
			<p id={hintId} class="hint">{hint}</p>
		{/if}
		{@render errorMessage()}
		{@render children()}
	</fieldset>
{/if}

<style>
	@layer components {
		fieldset {
			margin: 0;
			padding: 0;
			border: none;
			min-inline-size: 0;
		}

		legend {
			padding: 0;
			margin-block-end: var(--space-5);
			font-weight: var(--font-weight-medium);
			color: var(--color-on-surface);
		}

		fieldset:disabled > legend {
			opacity: var(--opacity-disabled);
		}

		legend:has(+ .hint) {
			margin-block-end: var(--space-1);
		}

		.hint {
			margin: 0 0 var(--space-5);
			color: var(--color-on-surface-muted);
			font-size: var(--text-body-sm-size);
		}

		.error {
			margin: 0 0 var(--space-3);
			color: var(--color-error);
			font-size: var(--text-body-sm-size);
		}
	}
</style>
