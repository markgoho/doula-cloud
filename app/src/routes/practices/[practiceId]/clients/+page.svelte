<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import { loadClients, type ClientListItem } from '#lib/client.js';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';

	let clients = $state<ClientListItem[]>([]);
	let error = $state('');
	let isLoaded = $state(false);
	let cursor = $state('');
	let isMoreAvailable = $state(false);

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

	function portalInviteStatusText(client: ClientListItem): string {
		return (
			(client.portalInviteStatus && portalInviteStatusLabel[client.portalInviteStatus]) ??
			'Never invited'
		);
	}

	const columns = [
		{ label: 'Name', accessor: (client: ClientListItem) => client.name },
		{ label: 'Email', accessor: (client: ClientListItem) => client.email },
		{ label: 'Portal invite', accessor: portalInviteStatusText }
	];

	// The Client detail hub (#494) -- DataTable links only its first
	// column, per rowHref's own contract.
	function clientHref(client: ClientListItem): string {
		return resolve('/practices/[practiceId]/clients/[clientId]', {
			practiceId: page.params.practiceId!,
			clientId: client.clientId
		});
	}

	onMount(async () => {
		try {
			const loaded = await loadClients(apiFetchWithSession, page.params.practiceId!);
			clients = loaded.items;
			cursor = loaded.nextCursor ?? '';
			isMoreAvailable = loaded.hasMore;
			isLoaded = true;
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load Clients';
		}
	});

	async function handleLoadMore() {
		try {
			const loaded = await loadClients(apiFetchWithSession, page.params.practiceId!, cursor);
			clients = [...clients, ...loaded.items];
			cursor = loaded.nextCursor ?? '';
			isMoreAvailable = loaded.hasMore;
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load more Clients';
		}
	}
</script>

<PageTitle page="Clients" />
<h1>Clients</h1>

<Link
	href={resolve('/practices/[practiceId]/clients/new', { practiceId: page.params.practiceId! })}
	label="Add a Client"
/>

{#if error}
	<p role="alert">{error}</p>
{:else if isLoaded}
	<DataTable
		{columns}
		rows={clients}
		rowHref={clientHref}
		hasMore={isMoreAvailable}
		onLoadMore={handleLoadMore}
		emptyMessage="No Clients yet."
	/>
{:else}
	<Skeleton variant="row" lines={8} label="Loading Clients" />
{/if}
