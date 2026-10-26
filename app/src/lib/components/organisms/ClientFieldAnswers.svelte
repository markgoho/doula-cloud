<script lang="ts">
	/*
	 * A Practice's own questions, asked (#466) -- the one renderer for a
	 * Practice-defined field, whoever is answering it.
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
	 * ## Two consumers, one renderer (#2014)
	 *
	 * The Client details sections ask a Practice's Client fields here,
	 * and `PlanInstanceForm` asks a Care Plan's and a Birth Plan's fields
	 * through this same component, one call per section. The two drew the
	 * same five field types two ways until #2014 found the Plan's version
	 * bulleted, boxed and run together; they change together now
	 * (ADR-0018's bar). So `fields` takes only what both kinds of field
	 * carry, and `values` takes whatever was stored, read by type below.
	 *
	 * ## An organism, by #424's rule
	 *
	 * A whole page section rather than a part of one: a Practice-named
	 * step is this component and nothing else.
	 *
	 * ## The five types, and the control each gets
	 *
	 * `section_header` never reaches here -- `intakeJourney.ts` and
	 * `PlanInstanceForm` both split on it rather than passing it through,
	 * because a heading is not a question. Of the five that do:
	 *
	 * - **short_text** a text input, **long_text** a textarea.
	 * - **single_select** a `Select`, not radios. Radios are GOV.UK's
	 *   default for a small option set, but a Practice's list is
	 *   unbounded and, since the save is free (ADR-0017), every one of
	 *   these has to have a real unanswered state. It is the first
	 *   option, "Not answered yet", and it stays selectable, so an answer
	 *   given by mistake can be taken back -- a disabled placeholder
	 *   could not be chosen again once left. #1253 turns these into
	 *   radios, in this one place.
	 * - **multi_select** a `CheckboxGroup`, which is GOV.UK's Checkboxes
	 *   pattern and the only control where "none of them" and "several of
	 *   them" are both sayable.
	 * - **checkbox** one checkbox, its label the question.
	 */
	import Checkbox from '#lib/components/atoms/Checkbox.svelte';
	import Select from '#lib/components/atoms/Select.svelte';
	import Textarea from '#lib/components/atoms/Textarea.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import CheckboxGroup from '#lib/components/molecules/CheckboxGroup.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import type { Field as ClientField } from '#lib/clientFieldTemplate.js';
	import type { Field as PlanField } from '#lib/planTemplate.js';
	import type { FieldValue } from '#lib/intakeDraft.svelte.js';

	/*
	A Client field or a Plan field: this reads only what the two share.
	*/
	type AskedField = ClientField | PlanField;

	interface Properties {
		fields: readonly AskedField[];
		values: Readonly<Record<string, unknown>>;
		onChange: (fieldId: string, value: FieldValue) => void;
		/**
		 * Prefixes each control's id, so an error summary entry can link to
		 * it and so two sections cannot collide on a field id.
		 */
		idPrefix: string;
	}

	let { fields, values, onChange, idPrefix }: Properties = $props();

	const UNANSWERED = { value: '', label: 'Not answered yet' };

	function fieldId(field: AskedField): string {
		return `${idPrefix}-${field.id}`;
	}

	function textValue(field: AskedField): string {
		const value = values[field.id];
		return typeof value === 'string' ? value : '';
	}

	function listValue(field: AskedField): string[] {
		const value = values[field.id];
		return Array.isArray(value) ? value : [];
	}

	function isChecked(field: AskedField): boolean {
		return values[field.id] === true;
	}

	function choiceOptions(field: AskedField) {
		return (field.options ?? []).map((option) => ({ value: option, label: option }));
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
			<CheckboxGroup
				legend={field.label}
				name={fieldId(field)}
				options={choiceOptions(field)}
				value={listValue(field)}
				onChange={(chosen) => onChange(field.id, chosen)}
				autocomplete="off"
			/>
		{:else if field.type === 'single_select'}
			<LabeledField id={fieldId(field)} label={field.label}>
				{#snippet children({ id, describedBy })}
					<Select
						{id}
						{describedBy}
						options={[UNANSWERED, ...choiceOptions(field)]}
						value={textValue(field)}
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
