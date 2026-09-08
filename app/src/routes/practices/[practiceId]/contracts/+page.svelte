<script lang="ts">
	/*
	 * The Practice-wide "Contracts awaiting signature" list (#273). Before
	 * it, finding an unsigned Contract meant opening every Engagement at
	 * the Practice in turn -- the BFF roll-up (#426) has existed since
	 * before this screen did, and this page is its first caller.
	 *
	 * Composes existing components only, the same as the Invoice list this
	 * follows (#265): the book is a `DataTable`, which already adapts to
	 * the space it is given.
	 */
	import { untrack } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import {
		awaitingContractStatusLabel,
		loadPracticeAwaitingContracts,
		loadPracticeAwaitingVoidRequests,
		type AwaitingContract,
		type AwaitingVoidRequest
	} from '#lib/contract.js';
	import { formatActivityTimestamp } from '#lib/dates.js';
	import { PaginatedList } from '#lib/paginatedList.svelte.js';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();

	/*
	 * Takes the load's first page and grows from there, the same reasoning
	 * as the Invoice list's own PaginatedList: the list owns its own
	 * cursor, in-flight guard and error text, so re-deriving from `data`
	 * would drop every page appended since.
	 */
	const contracts = new PaginatedList({
		first: untrack(() => data.contracts),
		loadPage: (cursor) =>
			loadPracticeAwaitingContracts(apiFetchWithSession, page.params.practiceId!, cursor),
		failureMessage: 'Failed to load more contracts'
	});

	const columns = [
		{ label: 'Client', accessor: (contract: AwaitingContract) => contract.clientName },
		{
			label: 'Status',
			accessor: (contract: AwaitingContract) => awaitingContractStatusLabel(contract.status)
		},
		{
			label: 'Waiting since',
			accessor: (contract: AwaitingContract) => formatActivityTimestamp(contract.createdAt),
			variant: 'meta' as const,
			datetimeAccessor: (contract: AwaitingContract) => contract.createdAt
		}
	];

	function engagementHref(contract: AwaitingContract): string {
		return resolve('/practices/[practiceId]/engagements/[engagementId]', {
			practiceId: page.params.practiceId!,
			engagementId: contract.engagementId
		});
	}

	// #971's own roll-up, beside the one above: "sees the void requests
	// waiting on them" is a second work list on the same screen, not a
	// second address -- the same reasoning that keeps this whole page
	// singular rather than one per roll-up.
	const voidRequests = new PaginatedList({
		first: untrack(() => data.voidRequests),
		loadPage: (cursor) =>
			loadPracticeAwaitingVoidRequests(apiFetchWithSession, page.params.practiceId!, cursor),
		failureMessage: 'Failed to load more void requests'
	});

	const voidRequestColumns = [
		{ label: 'Client', accessor: (request: AwaitingVoidRequest) => request.clientName },
		{ label: 'Asked by', accessor: (request: AwaitingVoidRequest) => request.requestedByName },
		{ label: 'Reason', accessor: (request: AwaitingVoidRequest) => request.reason },
		{
			label: 'Waiting since',
			accessor: (request: AwaitingVoidRequest) => formatActivityTimestamp(request.createdAt),
			variant: 'meta' as const,
			datetimeAccessor: (request: AwaitingVoidRequest) => request.createdAt
		}
	];

	function voidRequestEngagementHref(request: AwaitingVoidRequest): string {
		return resolve('/practices/[practiceId]/engagements/[engagementId]', {
			practiceId: page.params.practiceId!,
			engagementId: request.engagementId
		});
	}
</script>

<PageTitle page="Contracts" />
<Heading level={1} text="Contracts" />
<Text
	text="Every Contract this practice is still waiting on, oldest first. Open one to reach the engagement it belongs to and send, void or invoice it there."
	tone="muted"
/>

<DataTable
	{columns}
	rows={contracts.items}
	rowHref={engagementHref}
	hasMore={contracts.hasMore}
	onLoadMore={() => contracts.loadMore()}
	isLoadingMore={contracts.isLoadingMore}
	emptyMessage="Nothing is waiting. Every contract has been signed or voided."
/>

{#if contracts.loadMoreError}
	<Notice message={contracts.loadMoreError} variant="error" />
{/if}

<Heading level={2} text="Void requests" />
<Text
	text="Every void a Doula has asked for and nobody has decided yet, oldest first. Open one to reach the engagement it belongs to and void or decline it there."
	tone="muted"
/>

<DataTable
	columns={voidRequestColumns}
	rows={voidRequests.items}
	rowHref={voidRequestEngagementHref}
	hasMore={voidRequests.hasMore}
	onLoadMore={() => voidRequests.loadMore()}
	isLoadingMore={voidRequests.isLoadingMore}
	emptyMessage="No void request is waiting on you."
/>

{#if voidRequests.loadMoreError}
	<Notice message={voidRequests.loadMoreError} variant="error" />
{/if}
