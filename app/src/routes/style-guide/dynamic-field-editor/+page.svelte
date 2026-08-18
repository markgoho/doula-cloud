<script lang="ts">
	import DynamicFieldEditor from '#lib/components/organisms/DynamicFieldEditor.svelte';
	import { addField, removeField, moveField, type Field, type FieldType } from '#lib/planTemplate.js';

	let fields = $state<Field[]>([
		{ id: 'a', type: 'short_text', label: 'Support people', order: 0 },
		{ id: 'b', type: 'single_select', label: 'Pain management', options: ['Unmedicated', 'Epidural'], order: 1 }
	]);

	function updateField(id: string, patch: Partial<Field>) {
		fields = fields.map((field) => (field.id === id ? { ...field, ...patch } : field));
	}
</script>

<stack-l space="var(--space-6)">
	<h1>Dynamic field editor</h1>

	<section>
		<h2>Default</h2>
		<DynamicFieldEditor
			{fields}
			onAdd={(type: FieldType) => (fields = addField(fields, crypto.randomUUID(), type))}
			onRemove={(id: string) => (fields = removeField(fields, id))}
			onMoveUp={(id: string) => (fields = moveField(fields, id, 'up'))}
			onMoveDown={(id: string) => (fields = moveField(fields, id, 'down'))}
			onLabelChange={(id: string, label: string) => updateField(id, { label })}
			onTypeChange={(id: string, type: FieldType) => updateField(id, { type })}
			onOptionsChange={(id: string, options: string[]) => updateField(id, { options })}
		/>
	</section>

	<section>
		<h2>Empty</h2>
		<DynamicFieldEditor
			fields={[]}
			onAdd={() => {}}
			onRemove={() => {}}
			onMoveUp={() => {}}
			onMoveDown={() => {}}
			onLabelChange={() => {}}
			onTypeChange={() => {}}
			onOptionsChange={() => {}}
		/>
	</section>
</stack-l>
