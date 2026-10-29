<script module lang="ts">
	/** The ids of the page's controls, so an error summary's link reaches
	 * the one it names. */
	export function practiceQuestionIds(name: string) {
		return {
			label: `${name}-label`,
			type: `${name}-type`,
			options: `${name}-options`,
			position: `${name}-position`
		};
	}
</script>

<script lang="ts">
	/*
	 * One of a Practice's own questions, on its own page (#2193): its words,
	 * its kind of answer, the answers to choose from on a choice question,
	 * and its place in the list. The Plan Templates and the Client Fields
	 * question pages both draw it inside `QuestionPage`'s fieldset, whose
	 * legend is the page's <h1>; what saving does, and removing, stay with
	 * each page.
	 *
	 * Reordering is the "Place in the list" select (#2193): GOV.UK has no
	 * pattern for reordering (docs/design/govuk-alignment.md, Reorder), so it
	 * is one more question about this question, asked on its page, rather
	 * than a Move up and Move down pair on every row of the list, which would
	 * add two decisions a row back to the screen #2193 rebuilt. A new
	 * question is added at the end, so it draws no place to choose until it
	 * is saved (ADR-0055: a question opens with nothing chosen).
	 */
	import { FIELD_TYPE_OPTIONS, isSelectType, type FieldType } from '#lib/planTemplate.js';
	import Select from '../atoms/Select.svelte';
	import Textarea from '../atoms/Textarea.svelte';
	import TextInput from '../atoms/TextInput.svelte';
	import LabeledField from '../molecules/LabeledField.svelte';

	interface Properties {
		/**
		The stem of every control's id (`practiceQuestionIds`).
		*/
		name: string;
		type: FieldType;
		label: string;
		/**
		The answers box's text, one answer on each line.
		*/
		optionsText: string;
		/**
		Its place, from 1; absent for a question not yet saved.
		*/
		position?: number;
		/**
		How many places there are to choose from.
		*/
		positionCount?: number;
		/**
		A saved Client field keeps its kind of answer.
		*/
		isKindLocked?: boolean;
		/**
		The refusal against each control, by its id.
		*/
		errorFor?: (id: string) => string | undefined;
		onLabelInput: (label: string) => void;
		onTypeChange: (type: FieldType) => void;
		onOptionsInput: (text: string) => void;
		onPositionChange?: (position: number) => void;
	}

	let {
		name,
		type,
		label,
		optionsText,
		position,
		positionCount = 0,
		isKindLocked = false,
		errorFor,
		onLabelInput,
		onTypeChange,
		onOptionsInput,
		onPositionChange
	}: Properties = $props();

	const ids = $derived(practiceQuestionIds(name));
	const positionOptions = $derived(
		Array.from({ length: positionCount }, (_, index) => ({
			value: String(index + 1),
			label: `${index + 1} of ${positionCount}`
		}))
	);
</script>

<stack-l space="var(--space-field)">
	<LabeledField id={ids.label} label={type === 'section_header' ? 'Heading' : 'Question'} error={errorFor?.(ids.label)}>
		{#snippet children({ id, describedBy, invalid })}
			<TextInput {id} {describedBy} {invalid} value={label} onInput={onLabelInput} />
		{/snippet}
	</LabeledField>
	<LabeledField
		id={ids.type}
		label="Kind of answer"
		hint={isKindLocked ? 'You cannot change this once it is saved.' : undefined}
		width={20}
	>
		{#snippet children({ id, describedBy, invalid, width })}
			<Select
				{id}
				{describedBy}
				{invalid}
				{width}
				value={type}
				options={FIELD_TYPE_OPTIONS}
				disabled={isKindLocked}
				onChange={(value) => onTypeChange(value as FieldType)}
			/>
		{/snippet}
	</LabeledField>
	{#if isSelectType(type)}
		<LabeledField
			id={ids.options}
			label="Answers to choose from"
			hint="One answer on each line"
			error={errorFor?.(ids.options)}
		>
			{#snippet children({ id, describedBy, invalid })}
				<Textarea {id} {describedBy} {invalid} value={optionsText} onInput={onOptionsInput} rows={4} />
			{/snippet}
		</LabeledField>
	{/if}
	{#if position !== undefined && positionCount > 1}
		<LabeledField id={ids.position} label="Place in the list" hint="1 comes first." width={10}>
			{#snippet children({ id, describedBy, invalid, width })}
				<Select
					{id}
					{describedBy}
					{invalid}
					{width}
					value={String(position)}
					options={positionOptions}
					onChange={(value) => onPositionChange?.(Number(value))}
				/>
			{/snippet}
		</LabeledField>
	{/if}
</stack-l>
