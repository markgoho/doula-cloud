<script lang="ts">
	import type { Field, FieldType } from '#lib/clientFieldTemplate.js';
	import Button from '../atoms/Button.svelte';
	import Heading from '../atoms/Heading.svelte';
	import ConfirmDialog from '../molecules/ConfirmDialog.svelte';
	import SummaryList, { type SummaryListRow } from '../molecules/SummaryList.svelte';
	import PracticeQuestionsEditor from './PracticeQuestionsEditor.svelte';

	/**
	 * ADR-0017's Client Field Template, on `PracticeQuestionsEditor`
	 * (#2024). This holds only what a Client field does that a Plan
	 * Template field does not: removing a field archives it, never deletes
	 * it, and a field that existed when the template was loaded
	 * (existingIds) keeps its kind of answer, since the Go BFF refuses a
	 * type change once its values are read live against it.
	 *
	 * An archived field is not part of the form an Owner is building but a
	 * record of a question this Practice no longer asks (#461), so it is a
	 * separate list under its own heading, with "Collect again" to restore
	 * it. Archiving is gated behind ConfirmDialog, the one destructive act
	 * this screen offers, so the consequence is read before it happens;
	 * collecting again is not, since it only adds the question back.
	 */
	let {
		fields,
		existingIds,
		onAdd,
		onArchiveToggle,
		onMoveUp,
		onMoveDown,
		onLabelChange,
		onTypeChange,
		onOptionsChange
	}: {
		fields: Field[];
		existingIds: ReadonlySet<string>;
		onAdd: (type: FieldType) => void;
		onArchiveToggle: (id: string) => void;
		onMoveUp: (id: string) => void;
		onMoveDown: (id: string) => void;
		onLabelChange: (id: string, label: string) => void;
		onTypeChange: (id: string, type: FieldType) => void;
		onOptionsChange: (id: string, options: string[]) => void;
	} = $props();

	/**
	The id of the field whose archive confirmation is open, or '' for none.
	*/
	let confirmArchiveId = $state('');

	// Move up and Move down walk the active list only (clientFieldTemplate's
	// moveField), so the editor is given that list, and a position it
	// announces is counted in it.
	let activeFields = $derived(fields.filter((f) => !f.archived));
	let archivedRows = $derived(
		fields
			.filter((f) => f.archived)
			.map((f) => ({ id: f.id, label: fieldName(f), value: 'No longer asked', changes: fieldName(f) }))
	);
	let fieldToArchive = $derived(fields.find((f) => f.id === confirmArchiveId));

	function fieldName(field: Field): string {
		return field.label || 'Untitled question';
	}
</script>

<stack-l space="var(--space-6)">
	<PracticeQuestionsEditor
		questions={activeFields}
		emptyMessage="No extra questions yet. Add one below."
		removeLabel="Archive"
		lockedTypeIds={existingIds}
		{onAdd}
		onRemove={(id) => (confirmArchiveId = id)}
		{onMoveUp}
		{onMoveDown}
		{onLabelChange}
		{onTypeChange}
		{onOptionsChange}
	/>

	<!-- v8 ignore start: the setter half of this get/set bind:open only ever
	     runs with value=false (Dialog closes itself, it never reopens itself),
	     so the branch guarding the true case is Svelte-internal plumbing no
	     app-level test can reach. -->
	<ConfirmDialog
		bind:open={
			() => fieldToArchive !== undefined,
			(value) => {
				if (!value) confirmArchiveId = '';
			}
		}
		title={`Archive "${fieldToArchive ? fieldName(fieldToArchive) : ''}"?`}
		consequence="A Client who already answered this keeps that answer, read-only on the Client's record. New Clients will not be asked this question, and you can collect it again later from the archived questions."
		confirmLabel="Archive question"
		onConfirm={() => onArchiveToggle(confirmArchiveId)}
	/>
	<!-- v8 ignore stop -->

	{#if archivedRows.length > 0}
		<stack-l space="var(--space-3)">
			<Heading level={2} text="Archived questions" />
			<SummaryList rows={archivedRows}>
				{#snippet action(row: SummaryListRow & { id: string }, describedBy: string)}
					<Button label="Collect again" variant="link" {describedBy} onClick={() => onArchiveToggle(row.id)} />
				{/snippet}
			</SummaryList>
		</stack-l>
	{/if}
</stack-l>
