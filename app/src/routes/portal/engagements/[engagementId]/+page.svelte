<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '$lib/firebase';
	import { apiFetch } from '$lib/api';

	type Detail = {
		engagementId: string;
		practiceName: string;
		status: string;
		createdAt: string;
	};

	let detail = $state<Detail | null>(null);
	let error = $state('');

	onMount(async () => {
		const user = getFirebaseAuth().currentUser;
		if (!user) {
			await goto(resolve('/portal/login'));
			return;
		}

		const idToken = await user.getIdToken();
		const response = await apiFetch(`/api/portal/engagements/${page.params.engagementId}`, idToken);
		if (!response.ok) {
			error = await response.text();
			return;
		}

		detail = await response.json();
	});
</script>

{#if error}
	<p role="alert">{error}</p>
{:else if detail}
	<h1>Welcome to {detail.practiceName}</h1>
	<dl>
		<dt>Status</dt>
		<dd>{detail.status}</dd>
		<dt>Created</dt>
		<dd>{new Date(detail.createdAt).toLocaleDateString()}</dd>
	</dl>
{/if}
