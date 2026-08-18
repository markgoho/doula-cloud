<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiFetch } from '#lib/api.js';
	import { loadClientBirthPlan, type Instance } from '#lib/planInstance.js';
	import BirthPlanView from '#lib/BirthPlanView.svelte';

	let instance = $state<Instance | null | undefined>();
	let error = $state('');

	onMount(async () => {
		const user = getFirebaseAuth().currentUser;
		if (!user) {
			await goto(resolve('/portal/login'));
			return;
		}

		const idToken = await user.getIdToken();
		try {
			instance = await loadClientBirthPlan(
				(path) => apiFetch(path, idToken),
				page.params.engagementId!
			);
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load Birth Plan';
		}
	});
</script>

<div class="no-print">
	<a
		href={resolve('/portal/(authenticated)/engagements/[engagementId]', { engagementId: page.params.engagementId! })}
		>Back</a
	>
</div>

{#if error}
	<p role="alert" class="no-print">{error}</p>
{:else if instance === undefined}
	<p class="no-print">Loading...</p>
{:else if instance === null}
	<p class="no-print">No Birth Plan has been created for this Engagement yet.</p>
{:else}
	<h1>Birth Plan</h1>
	<button type="button" class="no-print" onclick={() => print()}>Print</button>
	<BirthPlanView fields={instance.fields} answers={instance.answers} />
{/if}

<style>
	@media print {
		.no-print {
			display: none;
		}
	}
</style>
