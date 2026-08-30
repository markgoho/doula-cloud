<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { apiFetchWithSession } from '#lib/api.js';
	import ContractTemplateEditor from '#lib/components/organisms/ContractTemplateEditor.svelte';
	import { loadContractTemplate, saveContractTemplate, validateProse } from '#lib/contractTemplate.js';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';

	let prose = $state('');
	let error = $state('');
	let isSaved = $state(false);

	async function load() {
		error = '';
		isSaved = false;
		try {
			const template = await loadContractTemplate(apiFetchWithSession, page.params.practiceId!);
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
			const template = await saveContractTemplate(apiFetchWithSession, page.params.practiceId!, prose);
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

<PageTitle page="Contract Template" />
<Heading level={1} text="Contract Template" />

{#if error}
	<Notice variant="error" message={error} />
{/if}
{#if isSaved}
	<Text text="Saved." />
{/if}

<ContractTemplateEditor {prose} onProseChange={(value: string) => (prose = value)} />

<Button label="Save" onClick={save} />
