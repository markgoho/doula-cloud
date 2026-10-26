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
	import { PaginatedList } from '#lib/paginatedList.svelte.js';
	import ListPage from '#lib/components/templates/ListPage.svelte';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import Text from '#lib/components/atoms/Text.svelte';

	const activity = new PaginatedList<ActivityEntry>({
		first: { items: [], hasMore: false },
		loadPage: (cursor) => loadPracticeActivityPage(apiFetchWithSession, page.params.practiceId!, cursor),
		failureMessage: 'Failed to load more activity'
	});
	let isLoaded = $state(false);
	let loadError = $state('');

	onMount(async () => {
		try {
			activity.reset(await loadPracticeActivityPage(apiFetchWithSession, page.params.practiceId!, ''));
			isLoaded = true;
		} catch (error_) {
			loadError = error_ instanceof Error ? error_.message : 'Failed to load activity';
		}
	});
</script>

{#snippet intro()}
	<Text text="Who did what, and when, newest first." tone="variant" />
{/snippet}

{#snippet content()}
	<DataTable
		columns={activityLedgerColumns()}
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
	loading={isLoaded || loadError ? undefined : 'Loading activity'}
	loadError={loadError || undefined}
/>
