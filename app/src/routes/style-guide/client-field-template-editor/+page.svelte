<script lang="ts">
	import ClientFieldTemplateEditor from '#lib/components/organisms/ClientFieldTemplateEditor.svelte';
	import { addField, archiveField, unarchiveField, moveField, type Field, type FieldType } from '#lib/clientFieldTemplate.js';

	/*
	 * The longest realistic value, not a representative one (ADR-0025): a
	 * Practice writes these labels and options itself, so each one is the
	 * whole question it means to ask rather than a two-word heading.
	 */
	let fields = $state<Field[]>([
		{
			id: 'a',
			type: 'short_text',
			label: 'What the intake call turned up that the Contract does not say',
			order: 0,
			archived: false
		},
		{
			id: 'b',
			type: 'single_select',
			label: 'How did this Client hear about the Practice?',
			options: ['Referred by Rochester General Hospital Birthing Center', 'Word of mouth'],
			order: 1,
			archived: false
		},
		{
			id: 'c',
			type: 'short_text',
			label: 'Emergency contact, and their phone number',
			order: 2,
			archived: true
		}
	]);

	function updateField(id: string, patch: Partial<Field>) {
		fields = fields.map((field) => (field.id === id ? { ...field, ...patch } : field));
	}
</script>

<stack-l space="var(--space-6)">
	<h1>Client field template editor</h1>

	<section>
		<h2>Default -- active and archived fields, and a locked type on existing ones</h2>
		<ClientFieldTemplateEditor
			{fields}
			existingIds={new Set(fields.map((f) => f.id))}
			onAdd={(type: FieldType) => (fields = addField(fields, crypto.randomUUID(), type))}
			onArchiveToggle={(id: string) => {
				const field = fields.find((f) => f.id === id);
				fields = field?.archived ? unarchiveField(fields, id) : archiveField(fields, id);
			}}
			onMoveUp={(id: string) => (fields = moveField(fields, id, 'up'))}
			onMoveDown={(id: string) => (fields = moveField(fields, id, 'down'))}
			onLabelChange={(id: string, label: string) => updateField(id, { label })}
			onTypeChange={(id: string, type: FieldType) => updateField(id, { type })}
			onOptionsChange={(id: string, options: string[]) => updateField(id, { options })}
		/>
	</section>

	<section>
		<h2>Empty -- a new Practice's field list</h2>
		<ClientFieldTemplateEditor
			fields={[]}
			existingIds={new Set()}
			onAdd={() => {}}
			onArchiveToggle={() => {}}
			onMoveUp={() => {}}
			onMoveDown={() => {}}
			onLabelChange={() => {}}
			onTypeChange={() => {}}
			onOptionsChange={() => {}}
		/>
	</section>
</stack-l>
