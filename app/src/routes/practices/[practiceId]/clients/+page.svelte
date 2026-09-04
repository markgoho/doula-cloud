<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import { loadClients, type ClientListItem } from '#lib/client.js';
	import { PaginatedList } from '#lib/paginatedList.svelte.js';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import Checkbox from '#lib/components/atoms/Checkbox.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';
	import ListPage from '#lib/components/templates/ListPage.svelte';
	import type { PageProps as PageProperties } from './$types';

	// #539 (ADR-0017): +page.ts's load already decided, before this
	// component mounted, whether the caller is a contractor Doula with
	// neither the owner nor the admin role. `data` is optional only so
	// this component's own spec (which renders it directly, bypassing
	// SvelteKit's load cycle) keeps working without a fixture -- every
	// real navigation always supplies it.
	let { data }: { data?: PageProperties['data'] } = $props();
	const isContractor = $derived(data?.isContractor ?? false);

	// Starts empty: unlike Billing and Invoices this screen fetches its own
	// first page in the effect below rather than through a load, so every
	// page it holds arrives through this list.
	const clients = new PaginatedList<ClientListItem>({
		first: { items: [], hasMore: false },
		loadPage: (cursor) =>
			loadClients(apiFetchWithSession, page.params.practiceId!, {
				cursor,
				showAll: isShowingEveryone
			}),
		failureMessage: 'Failed to load more Clients'
	});
	let error = $state('');
	let isLoaded = $state(false);

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
		// abandoned rather than merged into the new filter's list. The rows
		// stay up until the new filter's first page arrives.
		clients.abandon();
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
	 * "Load more" is the only fetch that *merges* into what is on screen,
	 * so it is the only one a stale response can corrupt: a page-two
	 * request already in flight when the reader flips the toggle would
	 * append the old filter's Clients onto the new filter's list, and
	 * nothing afterwards would take them back out. `clients.reset` below
	 * abandons any such response -- the guard now lives in PaginatedList,
	 * where every paging list gets it rather than only this one.
	 *
	 * loadFirstPage needs no guard of its own. It *replaces* the list
	 * wholesale rather than appending, so two of them racing can only
	 * leave a whole, self-consistent result set from a filter one flip out
	 * of date -- which the effect's next run corrects -- never a list
	 * mixing both.
	 */
	async function loadFirstPage(isShowingEveryone: boolean) {
		isLoaded = false;
		error = '';
		try {
			clients.reset(
				await loadClients(apiFetchWithSession, page.params.practiceId!, {
					showAll: isShowingEveryone
				})
			);
			isLoaded = true;
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load Clients';
		}
	}

	function searchHref(): string {
		return resolve('/practices/[practiceId]/clients/search', {
			practiceId: page.params.practiceId!
		});
	}

	// #539 (ADR-0017): "No Clients yet." is silent about *why* to a
	// contractor Doula, who never gets to make one -- the same screen
	// tells her plainly that work reaches her as an Offer instead.
	const emptyMessage = $derived(
		isContractor ? 'Work reaches you as an Offer, so there are no Clients here yet.' : 'No Clients yet.'
	);
</script>

{#snippet actions()}
	<!--
		The search that fronts intake (#498, #539, ADR-0017): there is no
		top-level "Add a Client" action, so this link lands on the search
		screen, not on intake directly. A miss there hands the reader on to
		intake. Named for both errands it serves -- finding a returning
		Client is the one a contractor Doula's own attached-Clients list
		already covers, which is why she does not see this at all (#539).
	-->
	<Link href={searchHref()} label="Find or add a Client" />
{/snippet}

{#snippet content()}
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
		<Notice message={error} variant="error" />
	{:else if isLoaded}
		<DataTable
			{columns}
			rows={clients.items}
			rowHref={clientHref}
			hasMore={clients.hasMore}
			onLoadMore={() => clients.loadMore()}
			isLoadingMore={clients.isLoadingMore}
			loadMoreError={clients.loadMoreError}
			{emptyMessage}
		/>
		{#if isContractor && clients.items.length === 0}
			<!--
				#539, #501 (ADR-0017): hiding "Find or add a Client" above took
				away this door's only link for a contractor Doula. It only
				needs to come back here -- once she has an attached Client, the
				narrowed list above is already her route to it.
			-->
			<p><Link href={searchHref()} label="How to add Clients of your own" /></p>
		{/if}
	{:else}
		<Skeleton variant="row" lines={8} label="Loading Clients" />
	{/if}
{/snippet}

<ListPage title="Clients" actions={isContractor ? undefined : actions} {content} />
