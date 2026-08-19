<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';

	type ClientEngagement = {
		clientId: string;
		name: string;
		email: string;
		engagementId: string;
		status: string;
	};

	let clients = $state<ClientEngagement[]>([]);
	let error = $state('');
	let isLoaded = $state(false);

	onMount(async () => {
		const response = await apiFetchWithSession(`/api/practices/${page.params.practiceId}/clients`);
		if (!response.ok) {
			error = await response.text();
			return;
		}

		clients = await response.json();
		isLoaded = true;
	});
</script>

<h1>Clients</h1>

<a href={resolve('/practices/[practiceId]/clients/new', { practiceId: page.params.practiceId! })}
	>Add a Client</a
>

{#if error}
	<p role="alert">{error}</p>
{:else if isLoaded}
	{#if clients.length === 0}
		<p>No Clients yet.</p>
	{:else}
		<ul>
			{#each clients as client (client.engagementId)}
				<li>
					<a
						href={resolve('/practices/[practiceId]/engagements/[engagementId]', {
							practiceId: page.params.practiceId!,
							engagementId: client.engagementId
						})}
					>
						{client.name}
					</a>
					— {client.status}
				</li>
			{/each}
		</ul>
	{/if}
{/if}
