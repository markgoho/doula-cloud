<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { isOwnerOrAdmin } from '#lib/roles.js';
	import ClientFieldTemplateEditor from '#lib/components/organisms/ClientFieldTemplateEditor.svelte';
	import type { PracticeSession } from '../../+layout.js';
	import Text from '#lib/components/atoms/Text.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import {
		loadTemplate,
		saveTemplate,
		addField,
		archiveField,
		unarchiveField,
		moveField,
		validateFields,
		fieldCountWarning,
		type Field,
		type FieldType
	} from '#lib/clientFieldTemplate.js';

	let fields = $state<Field[]>([]);
	/** Ids present the last time the template was loaded or saved -- the
	 * editor locks the type select on these, since the Go BFF refuses a
	 * type change on a field that already exists. */
	let existingIds = $state<ReadonlySet<string>>(new Set());
	let error = $state('');
	let isSaved = $state(false);

	/* Read is every Staff member (ADR-0017), write is Owner or Admin
	 * (#460's RequireOwnerOrAdmin) -- the same "load for everyone, gate the
	 * write controls" split settings/payments/+page.svelte uses. Server-side
	 * enforcement (RequireOwnerOrAdmin) is what actually holds the line;
	 * this is UX only. Resolved once by practices/[practiceId]/+layout.ts
	 * (#835), not a fetch of this page's own. */
	const session = $derived((page.data as { session: PracticeSession }).session);
	let isPracticeOwnerOrAdmin = $derived(isOwnerOrAdmin(session));

	let countWarning = $derived(fieldCountWarning(fields));

	async function load() {
		error = '';
		isSaved = false;
		try {
			const template = await loadTemplate(apiFetchWithSession, page.params.practiceId!);
			fields = template.fields;
			existingIds = new Set(template.fields.map((f) => f.id));
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load Client Field Template';
		}
	}

	async function save() {
		error = '';
		isSaved = false;
		const validationError = validateFields(fields);
		if (validationError) {
			error = validationError;
			return;
		}
		try {
			const template = await saveTemplate(apiFetchWithSession, page.params.practiceId!, fields);
			fields = template.fields;
			existingIds = new Set(template.fields.map((f) => f.id));
			isSaved = true;
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to save Client Field Template';
		}
	}

	function updateField(id: string, patch: Partial<Field>) {
		fields = fields.map((f) => (f.id === id ? { ...f, ...patch } : f));
	}

	onMount(() => {
		void load();
	});
</script>

<!--
	#865 aligned this with its two siblings, which had no intro at all until
	that ticket. Three changes, all of them the same rule: say what is true
	of *this* screen. "this Practice", not "your Practice", so the three
	settings screens and the Settings hub speak with one voice. The list
	starts empty rather than seeded -- `staffauth/signup.go` seeds Plan
	Templates and the Contract Template and deliberately seeds no Client
	fields, which `clientfieldtemplate/template.go` calls the designed state
	for a new Practice -- so the "a default meant to be changed" sentence the
	other two carry would be false here. And the reassurance is the archive
	rule, not ADR-0001's snapshot: CONTEXT.md is explicit that a Client's
	values are read live against the current list, so archiving is the only
	thing standing between a removed field and a recorded fact.
-->
{#snippet intro()}
	<Text
		text="Extra facts this Practice records about every Client, beyond name, contact details and address. Nothing is here to start with, and every field is staff-only: a Client never sees one. Removing a field archives it, so what was already recorded about a Client stays."
	/>
{/snippet}

{#snippet editor()}
	{#if error}
		<Notice variant="error" message={error} />
	{/if}
	{#if isSaved}
		<Text text="Saved." />
	{/if}
	{#if countWarning}
		<Notice variant="info" message={countWarning} />
	{/if}

	{#if isPracticeOwnerOrAdmin}
		<ClientFieldTemplateEditor
			{fields}
			{existingIds}
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
	{:else}
		<ul>
			{#each fields as field (field.id)}
				<li>
					<Text text="{field.label || 'Untitled field'}{field.archived ? ' (archived)' : ''}" />
				</li>
			{/each}
		</ul>
		<Text text="Ask a Practice Owner or Admin to edit Client fields." />
	{/if}
{/snippet}

{#snippet actions()}
	{#if isPracticeOwnerOrAdmin}
		<Button label="Save" onClick={save} />
	{/if}
{/snippet}

<FormPage title="Client Fields" {intro} fieldsets={[{ content: editor }]} {actions} />
