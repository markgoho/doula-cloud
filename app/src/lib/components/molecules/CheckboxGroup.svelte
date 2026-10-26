<script module lang="ts">
	/**
	 * The id a group's own option carries (`id="{name}-{option.value}"`
	 * below) -- exported so a caller building a `FormError`'s `targetId`
	 * for this group's first option constructs the same string this
	 * component does, as `radioFieldId` is for `RadioGroup`.
	 */
	export function checkboxFieldId(name: string, value: string): string {
		return `${name}-${value}`;
	}
</script>

<script lang="ts" generics="T extends string">
	/*
	 * GOV.UK's Checkboxes (#2014): a question, an optional hint under it,
	 * one option per line, an optional hint under each option, and one
	 * refusal for the whole group. It was hand-rolled three times --
	 * ClientFieldAnswers' multi-select, MembershipFields' Roles, and
	 * PlanInstanceForm's multi-select, the last as a browser-boxed
	 * fieldset with the options run together on one line. `RadioGroup` is
	 * the sibling: the same props, the same markup, the same spacing, so
	 * the two kinds of question on one form read as one product.
	 */
	import type { HTMLInputAttributes } from 'svelte/elements';
	import Checkbox from '../atoms/Checkbox.svelte';

	interface Option<T> {
		value: T;
		label: string;
		/* What choosing this option means, in the reader's words --
		   GOV.UK's checkbox hint, as `RadioGroup`'s `description` is. */
		description?: string;
	}

	interface Properties<T> {
		/* Optional for the reason `RadioGroup`'s is: a question page's
		   Template may own the fieldset and its legend already. */
		legend?: string;
		/* GOV.UK's hint for the question itself, under the legend. Only
		   drawn with a legend, as `RadioGroup`'s is. */
		hint?: string;
		name?: string;
		options: readonly Option<T>[];
		/* The chosen values. Empty chooses nothing: a checkbox question
		   opens with nothing ticked unless an answer is stored. */
		value: readonly T[];
		/* Every chosen value after the change, in the options' own order,
		   so the same set always reads the same way whatever order the
		   boxes were ticked in. A stored value no option offers any more
		   -- a Practice that took an option out of its own question -- is
		   kept, after the rest, rather than lost to an unrelated tick. */
		onChange: (value: T[]) => void;
		/* The refusal for the group, which belongs to the question rather
		   than to any one box -- GOV.UK's position, under the legend. */
		error?: string;
		/*
		Whose data this is, handed to every box (#469).
		*/
		autocomplete?: HTMLInputAttributes['autocomplete'];
	}

	const generatedName = $props.id();

	let { legend, hint, name = generatedName, options, value, onChange, error, autocomplete }: Properties<T> = $props();

	const errorId = $derived(`${name}-error`);
	const hintId = $derived(`${name}-hint`);
	const describedBy = $derived([hint ? hintId : undefined, error ? errorId : undefined].filter(Boolean).join(' ') || undefined);

	function toggle(option: T, isChosen: boolean) {
		const offered = options.map((entry) => entry.value);
		const chosen = offered.filter((entry) => (entry === option ? isChosen : value.includes(entry)));
		onChange([...chosen, ...value.filter((entry) => !offered.includes(entry))]);
	}
</script>

{#snippet errorMessage()}
	{#if error}
		<p id={errorId} class="error" role="alert">{error}</p>
	{/if}
{/snippet}

{#snippet boxes()}
	<stack-l space="var(--space-5)">
		<!-- v8 ignore start: only the compiled branch for "was this keyed
		     <div> added/removed from the DOM since the last render" is
		     unreachable here (Svelte's own each-block diffing internals, not
		     app code) -- the loop body itself is exercised by
		     "renders a box for each entry in options" in
		     CheckboxGroup.svelte.spec.ts -->
		{#each options as option (option.value)}
			<div class="choice">
				<!-- The <label> wraps the box, as RadioGroup's wraps its
				     circle (#1518): the box, the gap and the text are one
				     click target that ends where the text ends. -->
				<label class="option">
					<Checkbox
						id={checkboxFieldId(name, option.value)}
						{name}
						checked={value.includes(option.value)}
						invalid={error !== undefined}
						describedBy={option.description ? `${name}-${option.value}-hint` : undefined}
						onChange={(isChosen) => toggle(option.value, isChosen)}
						{autocomplete}
					/>
					<span class="text">{option.label}</span>
				</label>
				{#if option.description}
					<p id="{name}-{option.value}-hint" class="description">{option.description}</p>
				{/if}
			</div>
		{/each}
		<!-- v8 ignore stop -->
	</stack-l>
{/snippet}

{#if legend === undefined}
	{@render errorMessage()}
	{@render boxes()}
{:else}
	<!-- aria-describedby only, for the reason RadioGroup gives: aria-invalid
	     is not supported on role="group". -->
	<fieldset aria-describedby={describedBy}>
		<legend>{legend}</legend>
		{#if hint}
			<p id={hintId} class="hint">{hint}</p>
		{/if}
		{@render errorMessage()}
		{@render boxes()}
	</fieldset>
{/if}

<style>
	@layer components {
		/* Every rule below is RadioGroup's, so the two groups keep one
		   rhythm: the reasons are written there. */
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

		.choice {
			display: grid;
			grid-template-columns: auto minmax(0, max-content);
			justify-content: start;
			column-gap: var(--choice-gap);
		}

		.option {
			display: grid;
			grid-column: 1 / -1;
			grid-template-columns: subgrid;
			align-items: start;
			cursor: pointer;
		}

		.text {
			padding-block: calc((var(--choice-size) - 1lh) / 2);
			overflow-wrap: anywhere;
		}

		/* GOV.UK's hover halo, as on a radio. */
		.option:hover :global(input:not(:disabled)) {
			box-shadow: 0 0 0 var(--space-2) var(--color-outline-variant);
		}

		/* The box dims itself (Checkbox); the text dims with it here. */
		.option:has(:global(input:disabled)) .text {
			cursor: not-allowed;
			opacity: var(--opacity-disabled);
		}

		.description {
			grid-column: 2;
			margin: var(--space-1) 0 0;
			color: var(--color-on-surface-muted);
			font-size: var(--text-body-sm-size);
		}
	}
</style>
