<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import DataTable from '#lib/components/organisms/DataTable.svelte';

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

	const columns = [
		{ label: 'Name', accessor: (client: ClientEngagement) => client.name },
		{ label: 'Status', accessor: (client: ClientEngagement) => client.status }
	];

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
	<DataTable
		{columns}
		rows={clients}
		rowHref={(client) =>
			resolve('/practices/[practiceId]/engagements/[engagementId]', {
				practiceId: page.params.practiceId!,
				engagementId: client.engagementId
			})}
		emptyMessage="No Clients yet."
	/>
{/if}
