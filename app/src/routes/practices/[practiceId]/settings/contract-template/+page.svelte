<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { isOwner } from '#lib/roles.js';
	import ContractTemplateEditor from '#lib/components/organisms/ContractTemplateEditor.svelte';
	import { loadContractTemplate, saveContractTemplate, validateProse } from '#lib/contractTemplate.js';
	import Text from '#lib/components/atoms/Text.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import type { PracticeSession } from '../../+layout.js';

	// #970: the mount now refuses this write to everyone but an Owner
	// (staffauth.OwnerOnly). The read stays open to every Staff member, so
	// this screen keeps showing the template to all of them -- only the
	// Save action a non-Owner could never complete disappears, in
	// addition to the API's own refusal, never instead of it.
	const session = $derived((page.data as { session: PracticeSession }).session);
	const isPracticeOwner = $derived(isOwner(session));

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

<!--
	#865, the sibling of the Plan Templates intro. The reassurance is the
	same shape but a different mechanism: `contracts/contract.go` copies
	this prose into the Contract row at **creation**, so the freeze happens
	when a Contract is written, earlier than sending and earlier than
	signing. The copy says "already written" for that reason -- "already
	sent" would under-claim what is actually safe. The seeded prose is real
	boilerplate (`staffauth/signup.go`), so "meant to be replaced" is true
	rather than aspirational.
-->
{#snippet intro()}
	<Text
		text="Every Contract this Practice sends starts from the terms written here, with the Client's name, dates and price filled in. Doula Cloud wrote a first set of terms to start from, and it is meant to be replaced with this Practice's own. A Contract already written for a Client keeps the terms it was written from, so nothing changed here reaches a Contract already made."
	/>
{/snippet}

{#snippet editor()}
	{#if error}
		<Notice variant="error" message={error} />
	{/if}
	{#if isSaved}
		<Text text="Saved." />
	{/if}
	{#if !isPracticeOwner}
		<Notice variant="info" message="Only a Practice Owner can change these terms." />
	{/if}
	<ContractTemplateEditor {prose} onProseChange={(value: string) => (prose = value)} />
{/snippet}

{#snippet actions()}
	{#if isPracticeOwner}
		<Button label="Save" onClick={save} />
	{/if}
{/snippet}

<FormPage title="Contract Template" {intro} fieldsets={[{ content: editor }]} {actions} />
