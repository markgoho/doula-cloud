<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';

	let name = $state('');
	let email = $state('');
	let error = $state('');
	let isSubmitting = $state(false);

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		isSubmitting = true;
		try {
			const response = await apiFetchWithSession(`/api/practices/${page.params.practiceId}/clients`, {
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
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to add Client';
		} finally {
			isSubmitting = false;
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
	<button type="submit" disabled={isSubmitting}>Add Client</button>
	{#if error}
		<p role="alert">{error}</p>
	{/if}
</form>
