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
		<!--
			A two-track grid, not cluster-l (#510): cluster-l is flex-wrap, and
			a flex row that cannot hold both items drops the whole label to a
			second line below the control -- an unlabelled control followed by
			a stray sentence, worst on SignContract's long consent label. Two
			other shapes were rejected. Keeping the wrap and indenting the
			label (text-indent/padding hanging-indent) still lets the label
			leave the control behind on line one, and has to guess a fixed
			indent that only coincidentally matches whichever control is
			rendered (a 1.5rem checkbox, a 2.5rem toggle). Falling back to a
			stacked layout below some width is a breakpoint under another
			name, which ADR-0024 rules out for a sizing problem. `auto
			minmax(0, 1fr)` needs no query at all: the control keeps its own
			intrinsic width and the label wraps in whatever is left, at any
			width -- ADR-0024's rule 1, and ADR-0025's minimum-size note is why
			the label's track is `minmax(0, 1fr)` rather than a bare `1fr`.
		-->
		<div class="inline-row">
			{@render control()}
			{@render fieldLabel()}
		</div>
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

		.inline-row {
			display: grid;
			grid-template-areas: 'control label';
			grid-template-columns: auto minmax(0, 1fr);
			align-items: start;
			gap: var(--space-4);
		}

		.inline-row > :first-child {
			grid-area: control;
		}

		.inline-row > label {
			grid-area: label;
		}
	}
</style>
