<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { apiFetchWithSession } from '#lib/api.js';
	import DynamicFieldEditor from '#lib/components/organisms/DynamicFieldEditor.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import {
		loadTemplate,
		saveTemplate,
		addField,
		removeField,
		moveField,
		validateFields,
		type Field,
		type FieldType
	} from '#lib/planTemplate.js';

	const planTypes = [
		{ value: 'care_plan', label: 'Care Plan' },
		{ value: 'birth_plan', label: 'Birth Plan' }
	] as const;

	let planType = $state<'care_plan' | 'birth_plan'>('care_plan');
	let fields = $state<Field[]>([]);
	let error = $state('');
	let isSaved = $state(false);

	async function load() {
		error = '';
		isSaved = false;
		try {
			const template = await loadTemplate(apiFetchWithSession, page.params.practiceId!, planType);
			fields = template.fields;
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load plan template';
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
			const template = await saveTemplate(apiFetchWithSession, page.params.practiceId!, planType, fields);
			fields = template.fields;
			isSaved = true;
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to save plan template';
		}
	}

	function selectPlanType(value: 'care_plan' | 'birth_plan') {
		planType = value;
		void load();
	}

	function updateField(id: string, patch: Partial<Field>) {
		fields = fields.map((f) => (f.id === id ? { ...f, ...patch } : f));
	}

	onMount(() => {
		void load();
	});
</script>

{#snippet planTypeSelector()}
	<nav>
		{#each planTypes as pt (pt.value)}
			<Button label={pt.label} onClick={() => selectPlanType(pt.value)} disabled={planType === pt.value} />
		{/each}
	</nav>
{/snippet}

{#snippet editor()}
	{#if error}
		<Notice variant="error" message={error} />
	{/if}
	{#if isSaved}
		<Text text="Saved." />
	{/if}

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
{/snippet}

{#snippet actions()}
	<Button label="Save" onClick={save} />
{/snippet}

<FormPage
	title="Plan Templates"
	fieldsets={[{ content: planTypeSelector }, { content: editor }]}
	{actions}
/>
