<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiFetch } from '#lib/api.js';
	import ContractTemplateEditor from '#lib/ContractTemplateEditor.svelte';
	import {
		loadContractTemplate,
		saveContractTemplate,
		validateProse,
		type Fetcher
	} from '#lib/contractTemplate.js';

	let prose = $state('');
	let error = $state('');
	let isSaved = $state(false);

	async function fetcher(): Promise<Fetcher> {
		const user = getFirebaseAuth().currentUser;
		if (!user) {
			throw new Error('You must be logged in to manage the contract template');
		}
		const idToken = await user.getIdToken();
		return (path, init) => apiFetch(path, idToken, init);
	}

	async function load() {
		error = '';
		isSaved = false;
		try {
			const fetch = await fetcher();
			const template = await loadContractTemplate(fetch, page.params.practiceId!);
			prose = template.prose;
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load contract template';
		}
	}

	async function save() {
		error = '';
		isSaved = false;
		const validationError = validateProse(prose);
		if (validationError) {
			error = validationError;
			return;
		}
		try {
			const fetch = await fetcher();
			const template = await saveContractTemplate(fetch, page.params.practiceId!, prose);
			prose = template.prose;
			isSaved = true;
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to save contract template';
		}
	}

	onMount(() => {
		void load();
	});
</script>

<h1>Contract Template</h1>

{#if error}
	<p role="alert">{error}</p>
{/if}
{#if isSaved}
	<p>Saved.</p>
{/if}

<ContractTemplateEditor {prose} onProseChange={(value: string) => (prose = value)} />

<button type="button" onclick={save}>Save</button>
