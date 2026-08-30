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
		/*
		 * What the field is for, in the reader's words -- GOV.UK's hint
		 * text. It sits between the label and the control and is joined to
		 * the control by aria-describedby, so a screen reader announces the
		 * question and then the help rather than leaving the help
		 * unreachable beside it.
		 */
		hint?: string;
		error?: string;
		orientation?: 'stacked' | 'inline';
		children: Snippet<[ControlProperties]>;
	}

	const generatedId = $props.id();

	let { id = generatedId, label, hint, error, orientation = 'stacked', children }: Properties = $props();

	const errorId = $derived(`${id}-error`);
	const hintId = $derived(`${id}-hint`);
	const isInvalid = $derived(Boolean(error));
	/*
	 * The hint first, then the error: both are announced, and the order is
	 * the order they are read on screen. A space-separated list is what
	 * aria-describedby takes, and undefined rather than an empty string
	 * when there is neither, so the attribute is absent instead of blank.
	 */
	const describedBy = $derived(
		[hint ? hintId : undefined, error ? errorId : undefined].filter(Boolean).join(' ') || undefined
	);
</script>

{#snippet control()}
	{@render children({ id, describedBy, invalid: isInvalid })}
{/snippet}

{#snippet fieldLabel()}
	<label for={id}>{label}</label>
{/snippet}

{#snippet fieldHint()}
	{#if hint}
		<p id={hintId} class="hint">{hint}</p>
	{/if}
{/snippet}

{#snippet fieldError()}
	<!--
		Above the control, not below it: GOV.UK's Error message sits between
		the hint and the thing it refuses, so a person reading down the page
		meets the reason before the box they have to correct rather than
		after it. It rendered below until #475 walked the pages (#425 found
		the same class of defect on the label).
	-->
	{#if error}
		<p id={errorId} role="alert">{error}</p>
	{/if}
{/snippet}

<stack-l space="var(--space-1)">
	{#if orientation === 'inline'}
		{@render fieldError()}
		<cluster-l>
			{@render control()}
			{@render fieldLabel()}
		</cluster-l>
	{:else}
		{@render fieldLabel()}
		{@render fieldHint()}
		{@render fieldError()}
		{@render control()}
	{/if}
</stack-l>

<style>
	@layer components {
		/*
		 * Block, so that "stacked" stacks. A <label> is inline by default,
		 * and a text input is inline-block, so the two shared a line and
		 * every stacked field in the app read as a label beside its
		 * control rather than above it (#425). The label sits above the
		 * thing it names -- GOV.UK's pattern, and the one the brief's form
		 * work is built on.
		 */
		label {
			display: block;
			font-weight: var(--font-weight-medium);
			color: var(--color-on-surface);
		}

		/*
		 * Quieter than the label it follows and than the control it
		 * describes -- it is help, not the question.
		 */
		.hint {
			margin: 0;
			color: var(--color-on-surface-muted);
			font-size: var(--text-body-sm-size);
		}

		p[role='alert'] {
			color: var(--color-error);
			font-size: var(--text-body-sm-size);
		}
	}
</style>
