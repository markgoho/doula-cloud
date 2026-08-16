<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiFetch } from '#lib/api.js';
	import { loadClientContract, type Contract } from '#lib/contract.js';
	import ContractView from '#lib/ContractView.svelte';

	let contract = $state<Contract | null | undefined>();
	let error = $state('');

	onMount(async () => {
		const user = getFirebaseAuth().currentUser;
		if (!user) {
			await goto(resolve('/portal/login'));
			return;
		}

		const idToken = await user.getIdToken();
		try {
			contract = await loadClientContract((path) => apiFetch(path, idToken), page.params.engagementId!);
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load Contract';
		}
	});
</script>

<a href={resolve('/portal/engagements/[engagementId]', { engagementId: page.params.engagementId! })}>Back</a>

{#if error}
	<p role="alert">{error}</p>
{:else if contract === undefined}
	<p>Loading...</p>
{:else if contract === null}
	<p>No Contract has been sent for this Engagement yet.</p>
{:else}
	<h1>Contract</h1>
	<p>Status: {contract.status}</p>
	<ContractView prose={contract.prose} values={contract.values} />
{/if}
