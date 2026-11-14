<script lang="ts">
	/*
	 * The Practice's whole activity ledger (#2006): every row the feed's
	 * reader may see, newest first, a page at a time. The Overview shows
	 * the newest few and links here, so the Overview stays short at any
	 * width and the record stays whole one press away -- CLAUDE.md's audit
	 * trail asks that "how did this come to be?" always has an answer.
	 *
	 * The same columns as every other ledger (`activityLedgerColumns`), and
	 * the same per-row gate on the BFF (#486): a Doula reads the rows about
	 * her own work, an Owner or Admin reads the Practice's.
	 */
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { activityLedgerColumns, loadPracticeActivityPage, type ActivityEntry } from '#lib/activityLedger.js';
	import { practiceTimeZone, practiceZoneNote } from '#lib/practiceZone.js';
	import { PaginatedList } from '#lib/paginatedList.svelte.js';
	import ListPage from '#lib/components/templates/ListPage.svelte';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import { PageRead } from '#lib/pageRead.svelte.js';

	const activity = new PaginatedList<ActivityEntry>({
		first: { items: [], hasMore: false },
		loadPage: (cursor) => loadPracticeActivityPage(apiFetchWithSession, page.params.practiceId!, cursor),
		failureMessage: 'Failed to load more activity'
	});
	// The first page's read; the pages after it are `activity`'s own.
	const firstPage = new PageRead();

	onMount(() =>
		firstPage.load(async () => {
			activity.reset(await loadPracticeActivityPage(apiFetchWithSession, page.params.practiceId!, ''));
		}, 'Failed to load activity')
	);
</script>

{#snippet intro()}
	<Text text="Who did what, and when, newest first." tone="variant" />
	{#if practiceZoneNote()}
		<Text text={practiceZoneNote()!} tone="muted" />
	{/if}
{/snippet}

{#snippet content()}
	<DataTable
		columns={activityLedgerColumns(practiceTimeZone())}
		rows={activity.items}
		hasMore={activity.hasMore}
		onLoadMore={() => activity.loadMore()}
		isLoadingMore={activity.isLoadingMore}
		loadMoreError={activity.loadMoreError}
		emptyMessage="Nothing has happened yet."
	/>
{/snippet}

<ListPage
	title="Activity"
	{intro}
	{content}
	read={firstPage.frame('Loading activity')}
/>
