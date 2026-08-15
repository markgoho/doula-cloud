<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '$lib/firebase';
	import { apiFetch } from '$lib/api';

	let practiceName = $state('');
	let error = $state('');

	onMount(async () => {
		const user = getFirebaseAuth().currentUser;
		if (!user) {
			await goto(resolve('/login'));
			return;
		}

		const idToken = await user.getIdToken();
		const response = await apiFetch(`/api/practices/${page.params.practiceId}/session`, idToken);
		if (!response.ok) {
			error = await response.text();
			return;
		}

		const body: { practiceName: string } = await response.json();
		practiceName = body.practiceName;
	});
</script>

{#if error}
	<p role="alert">{error}</p>
{:else if practiceName}
	<h1>Welcome to {practiceName}</h1>
{/if}
