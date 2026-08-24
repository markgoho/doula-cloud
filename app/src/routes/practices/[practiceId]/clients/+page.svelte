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
		portalInviteStatus?: string;
	};

	let clients = $state<ClientEngagement[]>([]);
	let error = $state('');
	let isLoaded = $state(false);

	// #346: labels for portal_invite_outbox's states, plus "accepted"
	// (from client_portal_users.identity_uid) and the absent-key
	// fallback below for a Client never invited at all. "complained"
	// reads as informational -- the mail arrived, re-inviting will not
	// help -- unlike "bounced"/"dead_lettered", which do need one.
	const portalInviteStatusLabel: Record<string, string> = {
		pending: 'Invite pending',
		sent: 'Invite sent',
		bounced: 'Bounced — needs re-invite',
		dead_lettered: 'Dead-lettered — needs re-invite',
		complained: 'Marked as spam (no action needed)',
		accepted: 'Accepted'
	};

	function portalInviteStatusText(client: ClientEngagement): string {
		return (
			(client.portalInviteStatus && portalInviteStatusLabel[client.portalInviteStatus]) ??
			'Never invited'
		);
	}

	const columns = [
		{ label: 'Name', accessor: (client: ClientEngagement) => client.name },
		{ label: 'Status', accessor: (client: ClientEngagement) => client.status },
		{ label: 'Portal invite', accessor: portalInviteStatusText }
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
