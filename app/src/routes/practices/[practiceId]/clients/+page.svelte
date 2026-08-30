<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import { loadClients, type ClientListItem } from '#lib/client.js';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import Checkbox from '#lib/components/atoms/Checkbox.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';

	let clients = $state<ClientListItem[]>([]);
	let error = $state('');
	let isLoaded = $state(false);
	let cursor = $state('');
	let isMoreAvailable = $state(false);

	// The default filter is "Clients with work" (ADR-0017); `?all=true`
	// switches to everyone -- including a Client whose only Request was
	// refused. A URL param, not local state: shareable, survives a
	// reload, and works with the back button, all for free from
	// SvelteKit's own navigation (#499).
	let isShowingEveryone = $derived(page.url.searchParams.get('all') === 'true');

	function clientsPath(isShowingEveryone: boolean): string {
		const base = resolve('/practices/[practiceId]/clients', {
			practiceId: page.params.practiceId!
		});
		return isShowingEveryone ? `${base}?all=true` : base;
	}

	// eslint-disable-next-line unicorn/consistent-boolean-name -- mirrors the native HTMLInputElement `checked` property Checkbox's onChange wraps 1:1 (same exception Checkbox.svelte itself carries)
	async function handleToggleAll(checked: boolean) {
		// Before the navigation, so a "Load more" already in flight is
		// abandoned rather than merged into the new filter's list.
		filterToken += 1;
		await goto(clientsPath(checked));
	}

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

	// ADR-0017: a pending Engagement Request shows on its Client's row.
	// She can hold one of each kind at once, hence the join rather than a
	// single value.
	function pendingRequestText(client: ClientListItem): string {
		const kinds = client.pendingRequestKinds ?? [];
		if (kinds.length === 0) return '';
		const labels = kinds.map((kind) => (kind === 'birth' ? 'Birth' : 'Postpartum'));
		return `${labels.join(' & ')} request pending`;
	}

	const columns = [
		{ label: 'Name', accessor: (client: ClientListItem) => client.name },
		{ label: 'Email', accessor: (client: ClientListItem) => client.email },
		{ label: 'Pending request', accessor: pendingRequestText },
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

	// Reloads page one against the current filter -- on first mount, on a
	// toggle click, and on a browser back/forward that lands on a
	// different `all` value, since `isShowingEveryone` is derived from
	// `page.url` and every one of those re-runs this effect.
	$effect(() => {
		void loadFirstPage(isShowingEveryone);
	});

	/*
	 * Bumped the moment the filter changes. "Load more" is the only fetch
	 * that *merges* into what is on screen, so it is the only one a stale
	 * response can corrupt: a page-two request already in flight when the
	 * reader flips the toggle would append the old filter's Clients onto
	 * the new filter's list, and nothing afterwards would take them back
	 * out. It therefore carries the counter's value from when it started
	 * and drops its own response if the counter has moved.
	 *
	 * loadFirstPage needs no such guard. It *replaces* `clients` wholesale
	 * rather than appending, so two of them racing can only leave a whole,
	 * self-consistent result set from a filter one flip out of date --
	 * which the effect's next run corrects -- never a list mixing both.
	 */
	let filterToken = 0;

	async function loadFirstPage(isShowingEveryone: boolean) {
		isLoaded = false;
		error = '';
		try {
			const loaded = await loadClients(apiFetchWithSession, page.params.practiceId!, {
				showAll: isShowingEveryone
			});
			clients = loaded.items;
			cursor = loaded.nextCursor ?? '';
			isMoreAvailable = loaded.hasMore;
			isLoaded = true;
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load Clients';
		}
	}

	async function handleLoadMore() {
		const token = filterToken;
		try {
			const loaded = await loadClients(apiFetchWithSession, page.params.practiceId!, {
				cursor,
				showAll: isShowingEveryone
			});
			if (token !== filterToken) return;
			clients = [...clients, ...loaded.items];
			cursor = loaded.nextCursor ?? '';
			isMoreAvailable = loaded.hasMore;
		} catch (error_) {
			if (token !== filterToken) return;
			error = error_ instanceof Error ? error_.message : 'Failed to load more Clients';
		}
	}
</script>

<PageTitle page="Clients" />
<h1>Clients</h1>

<!--
	The search that fronts intake (#498, ADR-0017): there is no top-level
	"Add a Client" action, so this button lands on the search screen, not
	on intake directly. A miss there hands the reader on to intake.
-->
<Link
	href={resolve('/practices/[practiceId]/clients/search', { practiceId: page.params.practiceId! })}
	label="Add a Client"
/>

<!--
	The default view is "Clients with work" (ADR-0017); this switches to
	everyone, including a Client whose only Request was refused -- nothing
	new is built for her, she is simply no longer filtered out.
-->
<label>
	<Checkbox variant="toggle" checked={isShowingEveryone} onChange={handleToggleAll} />
	See everyone
</label>

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
