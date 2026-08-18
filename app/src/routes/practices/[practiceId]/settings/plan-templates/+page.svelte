<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiFetch } from '#lib/api.js';
	import DynamicFieldEditor from '#lib/components/organisms/DynamicFieldEditor.svelte';
	import {
		loadTemplate,
		saveTemplate,
		addField,
		removeField,
		moveField,
		validateFields,
		type Field,
		type FieldType,
		type Fetcher
	} from '#lib/planTemplate.js';

	const planTypes = [
		{ value: 'care_plan', label: 'Care Plan' },
		{ value: 'birth_plan', label: 'Birth Plan' }
	] as const;

	let planType = $state<'care_plan' | 'birth_plan'>('care_plan');
	let fields = $state<Field[]>([]);
	let error = $state('');
	let isSaved = $state(false);

	async function fetcher(): Promise<Fetcher> {
		const user = getFirebaseAuth().currentUser;
		if (!user) {
			throw new Error('You must be logged in to manage plan templates');
		}
		const idToken = await user.getIdToken();
		return (path, init) => apiFetch(path, idToken, init);
	}

	async function load() {
		error = '';
		isSaved = false;
		try {
			const fetch = await fetcher();
			const template = await loadTemplate(fetch, page.params.practiceId!, planType);
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
			const fetch = await fetcher();
			const template = await saveTemplate(fetch, page.params.practiceId!, planType, fields);
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

<h1>Plan Templates</h1>

<nav>
	{#each planTypes as pt (pt.value)}
		<button type="button" onclick={() => selectPlanType(pt.value)} disabled={planType === pt.value}>
			{pt.label}
		</button>
	{/each}
</nav>

{#if error}
	<p role="alert">{error}</p>
{/if}
{#if isSaved}
	<p>Saved.</p>
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

<button type="button" onclick={save}>Save</button>
