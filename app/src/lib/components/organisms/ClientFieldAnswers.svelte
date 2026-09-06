<script lang="ts">
	/*
	 * A Practice's own questions, asked (#466).
	 *
	 * ## Why this is not `DynamicFieldEditor`
	 *
	 * That organism *defines* a Practice's fields -- an Owner naming
	 * them, choosing types, adding options, reordering. This *collects*
	 * values against fields somebody else already defined. They share a
	 * type vocabulary and nothing else: one renders a label input beside
	 * a type picker, the other renders the control that type implies.
	 * #466 says so in as many words, and the Client edit route's
	 * carry-through of `fieldValues` as an opaque payload is the gap this
	 * fills.
	 *
	 * ## An organism, by #424's rule
	 *
	 * A whole page section rather than a part of one: a Practice-named
	 * step is this component and nothing else.
	 *
	 * ## The five types, and the control each gets
	 *
	 * `section_header` never reaches here -- `intakeJourney.ts` splits on
	 * it rather than passing it through, because a heading is not a
	 * question. Of the five that do:
	 *
	 * - **short_text** a text input, **long_text** a textarea.
	 * - **single_select** a `Select`, not radios. Radios are GOV.UK's
	 *   default for a small option set, but a Practice's list is
	 *   unbounded and, since the save is free (ADR-0017), every one of
	 *   these has to have a real unanswered state -- which a select's
	 *   placeholder is and a radio group is not.
	 * - **multi_select** a checkbox each, which is GOV.UK's Checkboxes
	 *   pattern and the only control where "none of them" and "several of
	 *   them" are both sayable.
	 * - **checkbox** one checkbox, its label the question.
	 */
	import Checkbox from '#lib/components/atoms/Checkbox.svelte';
	import Select from '#lib/components/atoms/Select.svelte';
	import Textarea from '#lib/components/atoms/Textarea.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import type { Field } from '#lib/clientFieldTemplate.js';
	import type { FieldValue } from '#lib/intakeDraft.svelte.js';

	interface Properties {
		fields: Field[];
		values: Record<string, FieldValue>;
		onChange: (fieldId: string, value: FieldValue) => void;
		/**
		 * Prefixes each control's id, so an error summary entry can link to
		 * it and so two sections cannot collide on a field id.
		 */
		idPrefix: string;
	}

	let { fields, values, onChange, idPrefix }: Properties = $props();

	function fieldId(field: Field): string {
		return `${idPrefix}-${field.id}`;
	}

	function textValue(field: Field): string {
		const value = values[field.id];
		return typeof value === 'string' ? value : '';
	}

	function listValue(field: Field): string[] {
		const value = values[field.id];
		return Array.isArray(value) ? value : [];
	}

	function isChecked(field: Field): boolean {
		return values[field.id] === true;
	}

	function toggleOption(field: Field, option: string, isChosen: boolean) {
		const chosen = listValue(field);
		onChange(
			field.id,
			isChosen ? [...chosen, option] : chosen.filter((entry) => entry !== option)
		);
	}
</script>

<stack-l space="var(--space-6)">
	{#each fields as field (field.id)}
		{#if field.type === 'checkbox'}
			<LabeledField id={fieldId(field)} label={field.label} orientation="inline">
				{#snippet children({ id, describedBy })}
					<Checkbox
						{id}
						{describedBy}
						checked={isChecked(field)}
						onChange={(isTicked) => onChange(field.id, isTicked)}
						autocomplete="off"
					/>
				{/snippet}
			</LabeledField>
		{:else if field.type === 'multi_select'}
			<!--
				Its own <fieldset>, unlike every other field here: the
				question names the group and each checkbox carries an
				option, so without one a screen reader announces four
				unrelated checkboxes and never the question they answer.
			-->
			<fieldset>
				<legend>{field.label}</legend>
				<stack-l space="var(--space-4)">
					<!-- v8 ignore start: only the compiled branch for "was this
					     keyed option added/removed from the DOM since the last
					     render" is unreachable here (Svelte's own each-block
					     diffing internals, not app code) -- the loop body is
					     exercised by "groups a multi-select under its own
					     question" in ClientFieldAnswers.svelte.spec.ts -->
					{#each field.options ?? [] as option (option)}
						<LabeledField
							id="{fieldId(field)}-{option}"
							label={option}
							orientation="inline"
						>
							{#snippet children({ id, describedBy })}
								<Checkbox
									{id}
									{describedBy}
									checked={listValue(field).includes(option)}
									ariaLabel="{field.label}: {option}"
									onChange={(isChosen) => toggleOption(field, option, isChosen)}
									autocomplete="off"
								/>
							{/snippet}
						</LabeledField>
					{/each}
					<!-- v8 ignore stop -->
				</stack-l>
			</fieldset>
		{:else if field.type === 'single_select'}
			<LabeledField id={fieldId(field)} label={field.label}>
				{#snippet children({ id, describedBy })}
					<Select
						{id}
						{describedBy}
						options={field.options ?? []}
						value={textValue(field)}
						placeholder="Not answered yet"
						onChange={(chosen) => onChange(field.id, chosen)}
						autocomplete="off"
					/>
				{/snippet}
			</LabeledField>
		{:else if field.type === 'long_text'}
			<LabeledField id={fieldId(field)} label={field.label}>
				{#snippet children({ id, describedBy })}
					<Textarea
						{id}
						{describedBy}
						value={textValue(field)}
						onInput={(value) => onChange(field.id, value)}
						autocomplete="off"
					/>
				{/snippet}
			</LabeledField>
		{:else}
			<LabeledField id={fieldId(field)} label={field.label}>
				{#snippet children({ id, describedBy })}
					<TextInput
						{id}
						{describedBy}
						value={textValue(field)}
						onInput={(value) => onChange(field.id, value)}
						autocomplete="off"
					/>
				{/snippet}
			</LabeledField>
		{/if}
	{/each}
</stack-l>

<style>
	@layer components {
		fieldset {
			margin: 0;
			padding: 0;
			border: 0;
			min-inline-size: 0;
		}

		legend {
			padding: 0;
			margin-block-end: var(--space-4);
			font-weight: var(--font-weight-medium);
			color: var(--color-on-surface);
		}
	}
</style>
