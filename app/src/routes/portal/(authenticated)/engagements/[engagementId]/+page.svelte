<script lang="ts">
	/*
	 * The portal hub -- "Your care". What the Client's Engagement is, and
	 * the way to the two documents that belong to it.
	 *
	 * The message thread used to render here and now has its own route
	 * (#452): Messages is a destination in the portal's nav, and a nav item
	 * that scrolls to a section of another page is a nav item that lies.
	 * Push registration moved with it, up to `portal/(authenticated)/
	 * +layout.svelte`, so a Client who lands straight on her Contract is
	 * still registered for #61's alerts.
	 */
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import { formatCalendarDay } from '#lib/dates.js';
	import { PaginatedList } from '#lib/paginatedList.svelte.js';
	import { activityLedgerColumns, loadPortalActivityPage, type ActivityEntry } from '#lib/activityLedger.js';
	import Link from '#lib/components/atoms/Link.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import RecordDetail from '#lib/components/templates/RecordDetail.svelte';

	type Detail = {
		engagementId: string;
		practiceName: string;
		clientName: string;
		status: string;
		dueDate?: string;
	};

	let detail = $state<Detail | undefined>();
	let error = $state('');

	// #486 AC5: the same record-scoped ledger the staff Engagement page
	// gets, behind a closed disclosure -- the design brief's own placement
	// decision for the Client portal.
	const activity = new PaginatedList<ActivityEntry>({
		first: { items: [], hasMore: false },
		loadPage: (cursor) => loadPortalActivityPage(apiFetchWithSession, page.params.engagementId!, cursor),
		failureMessage: 'Failed to load more activity'
	});
	let activityError = $state('');

	onMount(async () => {
		const response = await apiFetchWithSession(
			`/api/portal/engagements/${page.params.engagementId}`
		);
		if (!response.ok) {
			error = await response.text();
			return;
		}

		detail = await response.json();

		try {
			activity.reset(await loadPortalActivityPage(apiFetchWithSession, page.params.engagementId!, ''));
		} catch (error_) {
			activityError = error_ instanceof Error ? error_.message : 'Failed to load activity';
		}
	});

	/** The summary row's own facts (#505). `dueDate` is left out of the
	 * array entirely, rather than shown with a placeholder, when null --
	 * ADR-0017's postpartum-only Engagement genuinely has none, and a
	 * "Due date" row reading "Not given" would tell a Client something is
	 * missing from her own record rather than that nothing was ever due. */
	function summaryItems(d: Detail): { label: string; value: string }[] {
		const items = [{ label: 'Status', value: d.status }];
		if (d.dueDate) {
			items.push({ label: 'Due date', value: formatCalendarDay(d.dueDate) });
		}
		return items;
	}
</script>

{#snippet summary()}
	<DescriptionList items={summaryItems(detail!)} />
{/snippet}

{#snippet actions()}
	<Link
		href={resolve('/portal/(authenticated)/engagements/[engagementId]/birth-plan', {
			engagementId: page.params.engagementId!
		})}
		label="Birth plan"
	/>
	<Link
		href={resolve('/portal/(authenticated)/engagements/[engagementId]/contract', {
			engagementId: page.params.engagementId!
		})}
		label="Contract"
	/>
{/snippet}

<!--
	#486 AC5: CONTEXT.md's own words for this to a Client -- "Everything
	that has happened" -- as the section heading. GOV.UK's Details guidance
	is that a summary names what it reveals rather than a bare "Show" with
	no subject, so the disclosure's own toggle text repeats "what has
	happened" instead of the heading's exact words. Behind a closed
	disclosure, per the design brief's #433 amendment for the Client
	portal.
-->
{#snippet activitySection()}
	{#if activityError}
		<Notice variant="error" message={activityError} />
	{/if}
	<DataTable
		disclosure="Show what has happened"
		columns={activityLedgerColumns()}
		rows={activity.items}
		hasMore={activity.hasMore}
		onLoadMore={() => activity.loadMore()}
		isLoadingMore={activity.isLoadingMore}
		loadMoreError={activity.loadMoreError}
		emptyMessage="Nothing has happened yet."
	/>
{/snippet}

<!--
	Archetype D, ADR-0018 -- the same Template the staff Engagement page
	uses, which is the point of putting both on it. #486 gives this page
	its first section: everything else here is the record's own summary
	and the way to its documents.
-->
<RecordDetail
	title={detail ? `Welcome to ${detail.practiceName}` : ''}
	serviceName={page.data.practiceName}
	{summary}
	{actions}
	sections={detail ? [{ heading: 'Everything that has happened', content: activitySection }] : []}
	loading={detail || error ? undefined : 'Loading your Engagement'}
	loadError={error || undefined}
/>
