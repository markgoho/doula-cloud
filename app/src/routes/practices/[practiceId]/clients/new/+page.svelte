<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiFetch } from '#lib/api.js';

	let name = $state('');
	let email = $state('');
	let error = $state('');
	let submitting = $state(false);

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		submitting = true;
		try {
			const user = getFirebaseAuth().currentUser;
			if (!user) {
				error = 'You must be logged in to add a Client';
				return;
			}
			const idToken = await user.getIdToken();

			const response = await apiFetch(`/api/practices/${page.params.practiceId}/clients`, idToken, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ name, email })
			});
			if (!response.ok) {
				error = await response.text();
				return;
			}

			const created: { engagementId: string } = await response.json();
			await goto(
				resolve('/practices/[practiceId]/engagements/[engagementId]', {
					practiceId: page.params.practiceId!,
					engagementId: created.engagementId
				})
			);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to add Client';
		} finally {
			submitting = false;
		}
	}
</script>

<h1>Add a Client</h1>

<form onsubmit={handleSubmit}>
	<label>
		Their name
		<input type="text" bind:value={name} required />
	</label>
	<label>
		Their email
		<input type="email" bind:value={email} required />
	</label>
	<button type="submit" disabled={submitting}>Add Client</button>
	{#if error}
		<p role="alert">{error}</p>
	{/if}
</form>
