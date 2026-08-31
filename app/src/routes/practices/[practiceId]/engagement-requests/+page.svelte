<script lang="ts">
	/*
	 * The inbox (#503, ADR-0017): where pending Engagement Requests gather.
	 *
	 * It exists because the alternative is the hunt -- an approver at a
	 * fourteen-doula agency opening Client records one at a time to find
	 * out who is waiting. ADR-0017: "a pending Request stops a Doula from
	 * doing any work at all", so a Request nobody can find is a Doula
	 * nobody knows is stopped.
	 *
	 * Oldest first, which is the endpoint's own order: this is a queue of
	 * decisions owed, not a feed, and the longest wait has cost the most.
	 * Decided Requests never appear here -- a decided Request is history,
	 * and its history belongs on the Client's record.
	 *
	 * The row links to the approval screen rather than to the Client: the
	 * decision is what this screen is for, and her record is one click
	 * further on from there.
	 *
	 * This screen writes no layout of its own. It composes PageTitle,
	 * DataTable and Skeleton, each of which owns how it behaves in the
	 * space it is given.
	 */
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import { formatCalendarDay, formatInstant } from '#lib/dates.js';
	import {
		kindLabel,
		loadPendingRequests,
		type PendingRequestItem
	} from '#lib/engagementRequest.js';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';

	let requests = $state<PendingRequestItem[]>([]);
	let error = $state('');
	let isLoaded = $state(false);
	let cursor = $state('');
	let isMoreAvailable = $state(false);

	const columns = [
		{ label: 'Client', accessor: (request: PendingRequestItem) => request.clientName },
		{ label: 'Kind of work', accessor: (request: PendingRequestItem) => kindLabel(request.kind) },
		{
			label: 'Due date',
			// "Not given" rather than an empty cell: a postpartum ask has no
			// due date, and a blank cell reads as a fact that failed to load.
			accessor: (request: PendingRequestItem) =>
				request.dueDate ? formatCalendarDay(request.dueDate) : 'Not given'
		},
		{ label: 'Asked by', accessor: (request: PendingRequestItem) => request.requestedByName },
		{ label: 'Asked on', accessor: (request: PendingRequestItem) => formatInstant(request.requestedAt) }
	];

	// DataTable links its first column only, so the Client's name is the
	// link -- and it goes to the decision, not to her record.
	function approvalHref(request: PendingRequestItem): string {
		return resolve('/practices/[practiceId]/engagement-requests/[requestId]', {
			practiceId: page.params.practiceId!,
			requestId: request.requestId
		});
	}

	$effect(() => {
		void loadFirstPage();
	});

	async function loadFirstPage() {
		try {
			const loaded = await loadPendingRequests(apiFetchWithSession, page.params.practiceId!);
			requests = loaded.items;
			cursor = loaded.nextCursor ?? '';
			isMoreAvailable = loaded.hasMore;
			isLoaded = true;
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load pending requests';
		}
	}

	async function handleLoadMore() {
		try {
			const loaded = await loadPendingRequests(apiFetchWithSession, page.params.practiceId!, cursor);
			requests = [...requests, ...loaded.items];
			cursor = loaded.nextCursor ?? '';
			isMoreAvailable = loaded.hasMore;
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load more pending requests';
		}
	}
</script>

<PageTitle page="Requests awaiting approval" />
<h1>Requests awaiting approval</h1>

{#if error}
	<p role="alert">{error}</p>
{:else if isLoaded}
	<DataTable
		{columns}
		rows={requests}
		rowHref={approvalHref}
		hasMore={isMoreAvailable}
		onLoadMore={handleLoadMore}
		emptyMessage="No requests are waiting for a decision."
	/>
{:else}
	<Skeleton variant="row" lines={8} label="Loading pending requests" />
{/if}
