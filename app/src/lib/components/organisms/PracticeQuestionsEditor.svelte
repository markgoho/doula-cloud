<script module lang="ts">
	import type { FieldType } from '#lib/planTemplate.js';

	/** One of a Practice's own questions, as both a Plan Template field and
	 * a Client Field Template field carry it. The editor reads nothing else
	 * of either, so each consumer passes its own `Field` through as is. */
	export interface PracticeQuestion {
		id: string;
		type: FieldType;
		label: string;
		options?: string[];
	}
</script>

<script lang="ts">
	/*
	 * The list of a Practice's own questions (#2024, the #2001 audit's
	 * F-B-18 to F-B-21): a Plan Template's on the Plan Templates screen,
	 * and a Client Field Template's on the Client Fields screen. The two
	 * screens were two near-copies of one bulleted row of unlabeled boxes;
	 * they are one component now because they should change together
	 * (ADR-0018's 2026-10-25 bar), named for the one thing it edits.
	 *
	 * What differs stays with the caller. Removing a question is the
	 * caller's act and the caller's word (`removeLabel`): a Plan Template
	 * field is removed at once, a Client field is archived after a
	 * confirmation, which `ClientFieldTemplateEditor` owns with its
	 * archived list. `lockedTypeIds` is the Client Fields rule that a
	 * saved field's kind of answer is fixed, because the Go BFF refuses a
	 * type change once answers are read live against it.
	 *
	 * Each question is a card headed by its place in the list, with a
	 * visible label on every control (GOV.UK Question pages). Adding is
	 * one act, one button per kind of answer (#1252), so no add control
	 * opens with a kind already chosen (ADR-0055). Reordering and removing
	 * are secondary and quiet: the screen's one primary is its Save.
	 */
	import { tick } from 'svelte';
	import { FIELD_TYPE_NAMES, FIELD_TYPE_OPTIONS, FIELD_TYPES, isSelectType } from '#lib/planTemplate.js';
	import { focusAfterMove, moveAnnouncement } from '#lib/fieldMoveFocus.js';
	import Button from '../atoms/Button.svelte';
	import Heading from '../atoms/Heading.svelte';
	import Select from '../atoms/Select.svelte';
	import Textarea from '../atoms/Textarea.svelte';
	import TextInput from '../atoms/TextInput.svelte';
	import EmptyState from '../molecules/EmptyState.svelte';
	import LabeledField from '../molecules/LabeledField.svelte';

	interface Properties {
		questions: readonly PracticeQuestion[];
		/**
		The line an empty list shows, ending in where to add one.
		*/
		emptyMessage: string;
		/**
		The word on each card's remove control: "Remove", "Archive".
		*/
		removeLabel: string;
		/**
		Questions whose kind of answer can no longer change.
		*/
		lockedTypeIds?: ReadonlySet<string>;
		onAdd: (type: FieldType) => void;
		onRemove: (id: string) => void;
		onMoveUp: (id: string) => void;
		onMoveDown: (id: string) => void;
		onLabelChange: (id: string, label: string) => void;
		onTypeChange: (id: string, type: FieldType) => void;
		onOptionsChange: (id: string, options: string[]) => void;
	}

	let {
		questions,
		emptyMessage,
		removeLabel,
		lockedTypeIds = new Set(),
		onAdd,
		onRemove,
		onMoveUp,
		onMoveDown,
		onLabelChange,
		onTypeChange,
		onOptionsChange
	}: Properties = $props();

	// Bound inside an `{#if}`, so it is reassigned when the list first
	// appears; only `move` reads it, and only once there are two cards.
	let list = $state<HTMLElement>();
	let announcement = $state('');

	// A moved card drops focus to the page (#1920); `focusAfterMove` puts
	// it back on the pressed button once the caller has reordered. The
	// region is emptied first so that a text identical to the last one
	// (two "Untitled question" cards) is still a change, and is read
	// (#1955).
	async function move(id: string, direction: 'up' | 'down') {
		announcement = '';
		if (direction === 'up') {
			onMoveUp(id);
		} else {
			onMoveDown(id);
		}
		await tick();
		focusAfterMove(list!, id, direction);
		announcement = moveAnnouncement(questions, id, questionName);
	}

	// Form Design Patterns' "Add another" (its read-back, item 38): focus
	// goes to what was added, so the next key a person presses types the
	// new question.
	async function add(type: FieldType) {
		onAdd(type);
		await tick();
		const added = questions.at(-1);
		if (added) {
			list?.querySelector<HTMLElement>(`#${CSS.escape(questionInputId(added))}`)?.focus();
		}
	}

	function questionInputId(question: PracticeQuestion): string {
		return `${question.id}-question`;
	}

	// "Move up", "Move down" and "Remove" read as the same bare word on
	// every card (#515); a visually hidden sibling names the question each
	// one acts on, and is the one fallback for an unnamed question.
	function questionName(question: PracticeQuestion): string {
		return question.label || 'Untitled question';
	}

	function optionsText(question: PracticeQuestion): string {
		return (question.options ?? []).join('\n');
	}

	function handleOptionsInput(id: string, value: string) {
		onOptionsChange(
			id,
			value.split('\n').map((line) => line.trim())
		);
	}
</script>

<stack-l space="var(--space-6)">
	{#if questions.length === 0}
		<!-- Every Practice's Client Fields start here (#2015, F-B-21). No
		     action of its own: one per kind of answer is the next thing on
		     the page (#1252), and EmptyState takes one. -->
		<EmptyState message={emptyMessage} />
	{:else}
		<stack-l space="var(--space-4)" bind:this={list}>
			{#each questions as question, index (question.id)}
				{@const headingId = `${question.id}-heading`}
				{@const isTypeLocked = lockedTypeIds.has(question.id)}
				<!-- v8 ignore start: Svelte-compiled attribute-diffing branches for
				     the per-question id strings below are reached only by Svelte's
				     own reactivity internals (no test renames a question's id), the
				     exception both editors this replaced took. -->
				<div class="question" role="group" aria-labelledby={headingId}> <!-- spelling:ignore: aria-labelledby is an ARIA attribute name -->
					<stack-l space="var(--space-4)">
						<Heading level={2} variant="card" id={headingId} text="Question {index + 1}" />
						<LabeledField
							id={questionInputId(question)}
							label={question.type === 'section_header' ? 'Heading' : 'Question'}
						>
							{#snippet children({ id, describedBy, invalid })}
								<TextInput
									{id}
									{describedBy}
									{invalid}
									value={question.label}
									onInput={(value) => onLabelChange(question.id, value)}
								/>
							{/snippet}
						</LabeledField>
						<LabeledField
							label="Kind of answer"
							hint={isTypeLocked ? 'You cannot change this once it is saved.' : undefined}
							width={20}
						>
							{#snippet children({ id, describedBy, invalid, width })}
								<Select
									{id}
									{describedBy}
									{invalid}
									{width}
									value={question.type}
									options={FIELD_TYPE_OPTIONS}
									disabled={isTypeLocked}
									onChange={(value) => onTypeChange(question.id, value as FieldType)}
								/>
							{/snippet}
						</LabeledField>
						{#if isSelectType(question.type)}
							<LabeledField label="Answers to choose from" hint="One answer on each line">
								{#snippet children({ id, describedBy, invalid })}
									<Textarea
										{id}
										{describedBy}
										{invalid}
										value={optionsText(question)}
										onInput={(next) => handleOptionsInput(question.id, next)}
										rows={3}
									/>
								{/snippet}
							</LabeledField>
						{/if}
						<!-- The destructive act sits apart from the reorder pair, at
						     the card's far end, and is quiet (the Button row of
						     docs/design/govuk-alignment.md). -->
						<div class="actions">
							<cluster-l space="var(--space-2)">
								<Button
									label="Move up"
									variant="secondary"
									size="sm"
									describedBy="{question.id}-move-up-name"
									onClick={() => move(question.id, 'up')}
									ariaDisabled={index === 0}
								/>
								<span class="visually-hidden" id="{question.id}-move-up-name">{questionName(question)}</span>
								<Button
									label="Move down"
									variant="secondary"
									size="sm"
									describedBy="{question.id}-move-down-name"
									onClick={() => move(question.id, 'down')}
									ariaDisabled={index === questions.length - 1}
								/>
								<span class="visually-hidden" id="{question.id}-move-down-name">{questionName(question)}</span>
							</cluster-l>
							<Button
								label={removeLabel}
								variant="link"
								describedBy="{question.id}-remove-name"
								onClick={() => onRemove(question.id)}
							/>
							<span class="visually-hidden" id="{question.id}-remove-name">{questionName(question)}</span>
						</div>
					</stack-l>
				</div>
				<!-- v8 ignore stop -->
			{/each}
		</stack-l>
	{/if}
	<!-- In the page from the first render, empty, so the first move is read (#1955). -->
	<p class="visually-hidden" role="status">{announcement}</p>

	<stack-l space="var(--space-3)">
		<Heading level={2} text="Add a question" />
		<cluster-l space="var(--space-2)">
			{#each FIELD_TYPES as type (type)}
				<Button label={FIELD_TYPE_NAMES[type].addLabel} variant="secondary" onClick={() => add(type)} />
			{/each}
		</cluster-l>
	</stack-l>
</stack-l>

<style>
	@layer components {
		/* The brief's card: a 1px hairline and the one radius, no shadow. */
		.question {
			padding: var(--space-4);
			border: var(--border-thin) solid var(--color-outline-variant);
			border-radius: var(--radius);
			background-color: var(--color-surface-bright);
		}

		/* Wraps rather than shrinks: at a narrow width the remove control
		   drops under the reorder pair and stays at the line's start. */
		.actions {
			display: flex;
			flex-wrap: wrap;
			align-items: center;
			justify-content: space-between;
			gap: var(--space-3);
		}
	}
</style>
