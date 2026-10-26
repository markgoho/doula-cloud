<script module lang="ts">
	import type { InputWidth } from '#lib/components/atoms/TextInput.svelte';

	export interface ControlProperties {
		id: string;
		describedBy: string | undefined;
		invalid: boolean;
		/*
		 * This field's `width`, handed on for the control to take
		 * (`<TextInput {width} />`). Stated on the field, beside its label
		 * and hint, because what the answer holds is a fact about the
		 * question; the control is what sizes to it (#1216).
		 */
		width: InputWidth | undefined;
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
		/*
		 * How many characters the answer holds, on the scale `TextInput`
		 * names and argues. This component draws nothing with it: it is
		 * passed through to the control (#1216).
		 */
		width?: InputWidth;
		children: Snippet<[ControlProperties]>;
	}

	const generatedId = $props.id();

	let { id = generatedId, label, hint, error, orientation = 'stacked', width, children }: Properties = $props();

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
	{@render children({ id, describedBy, invalid: isInvalid, width })}
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
			second line below the control -- an unlabeled control followed by
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

			The `auto` track stays shrink-to-fit on purpose (#1216): it sizes
			to the control rather than the other way round, which is right for
			the checkbox and the toggle this row holds. A text input's 100%
			base resolves against that intrinsic track, so an inline field is
			at the control's own width, not the column's; a `width` passed
			here is still a cap on it, never a stretch.
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
			/* The one box-to-text gap (#2014), shared with RadioGroup and
			   CheckboxGroup -- see tokens.css. */
			gap: var(--choice-gap);
		}

		.inline-row > :first-child {
			grid-area: control;
		}

		/* Level with the control's middle on its first line, and wrapping
		   downward from there, as RadioGroup's text is (#1518): a checkbox
		   and a text input are both `--choice-size` tall, so the text is
		   pushed down by half of what they are taller than one line. A
		   switch is drawn shorter than that, so its label keeps the top. */
		.inline-row > label {
			grid-area: label;
			padding-block: calc((var(--choice-size) - 1lh) / 2);
		}

		.inline-row:has(> :global([role='switch'])) > label {
			padding-block: 0;
		}
	}
</style>
