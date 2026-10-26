<script module lang="ts">
	import type { DemoVariant } from '../drag-surface/dragSurface.js';

	interface Properties {
		isEmpty?: boolean;
		isKindLocked?: boolean;
	}

	export const variants: readonly DemoVariant<Properties>[] = [
		{ name: 'Practice questions editor, empty', props: { isEmpty: true } },
		{ name: 'Practice questions editor, every kind of answer fixed once saved', props: { isKindLocked: true } }
	];
</script>

<script lang="ts">
	import PracticeQuestionsEditor from '#lib/components/organisms/PracticeQuestionsEditor.svelte';
	import { addField, removeField, moveField, type Field, type FieldType } from '#lib/planTemplate.js';

	let { isEmpty = false, isKindLocked = false }: Properties = $props();

	/*
	 * The longest realistic value, not a representative one (ADR-0025): a
	 * Practice writes its own questions, so a label is the whole question
	 * and an option is the whole answer. One question of each shape the
	 * card draws differently: a plain answer, a choice with its answers
	 * box, and a section heading, whose box is labeled "Heading".
	 */
	const seeded: Field[] = [
		{
			id: 'a',
			type: 'long_text',
			label: 'Who do you want in the room with you, and is there anyone you do not want there at all?',
			order: 0
		},
		{
			id: 'b',
			type: 'single_select',
			label: 'What pain relief do you want offered, and when?',
			options: ['Nothing unless I ask for it', 'Epidural, as early as it can be given'],
			order: 1
		},
		{ id: 'c', type: 'section_header', label: 'After the birth', order: 2 }
	];

	// The page's state is its props' first value; the picker on the drag
	// surface mounts each variant afresh, so this never needs to track them.
	// svelte-ignore state_referenced_locally
	let fields = $state<Field[]>(isEmpty ? [] : seeded);
	const lockedTypeIds: ReadonlySet<string> = $derived(new Set(isKindLocked ? seeded.map((field) => field.id) : []));

	function updateField(id: string, patch: Partial<Field>) {
		fields = fields.map((field) => (field.id === id ? { ...field, ...patch } : field));
	}
</script>

<stack-l space="var(--space-6)">
	<h1>Practice questions editor</h1>

	<section>
		<PracticeQuestionsEditor
			questions={fields}
			emptyMessage="No questions yet. Add one below."
			removeLabel="Remove"
			{lockedTypeIds}
			onAdd={(type: FieldType) => (fields = addField(fields, crypto.randomUUID(), type))}
			onRemove={(id: string) => (fields = removeField(fields, id))}
			onMoveUp={(id: string) => (fields = moveField(fields, id, 'up'))}
			onMoveDown={(id: string) => (fields = moveField(fields, id, 'down'))}
			onLabelChange={(id: string, label: string) => updateField(id, { label })}
			onTypeChange={(id: string, type: FieldType) => updateField(id, { type })}
			onOptionsChange={(id: string, options: string[]) => updateField(id, { options })}
		/>
	</section>
</stack-l>
