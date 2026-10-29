<script lang="ts">
	/*
	 * The Practice-wide schedule (#263). Before it, "who is covering whom,
	 * and when" had no surface at all: a Staff member opened one
	 * Engagement page per Client and held the answer in her head.
	 *
	 * Composes existing components only, the same as the Practice-wide
	 * Contract list this follows (#273): `ListPage` for the frame,
	 * `DataTable` for the book, `FilterBar` with `LabeledField` +
	 * `TextInput`/`Select` for the narrowing (#2026). The one atom this
	 * screen changed is `Select`, which now takes `{value, label}` options
	 * as well as bare strings -- two Doulas at one agency can share a
	 * name, so the filter has to store an id.
	 *
	 * The narrowing is `FilterBar`'s real `<form method="get">`;
	 * `applyNarrowing` upgrades it to a client-side navigation rather than
	 * replacing it (Rule of Least Power).
	 */
	import { untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { formatScheduledVisit } from '#lib/dates.js';
	import { PaginatedList } from '#lib/paginatedList.svelte.js';
	import { isOwnerOrAdmin } from '#lib/roles.js';
	import {
		loadPracticeSchedule,
		scheduleFiltersFromParameters,
		scheduleHref,
		scheduleResultSummary,
		type ScheduledVisit
	} from '#lib/visitSchedule.js';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Select from '#lib/components/atoms/Select.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import FilterBar from '#lib/components/molecules/FilterBar.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import ListPage from '#lib/components/templates/ListPage.svelte';
	import type { PracticeSession } from '../+layout.js';
	import type { SchedulePageData } from './+page.js';

	/*
	 * `data` is the read once it lands (#2255). Until then the screen draws
	 * what the URL and the session already say -- the filter bar -- and a
	 * row skeleton where the rows will land.
	 */
	let { data }: { data?: SchedulePageData } = $props();

	const practiceId = $derived(page.params.practiceId!);
	const basePath = $derived(
		resolve('/practices/[practiceId]/schedule', { practiceId: page.params.practiceId! })
	);
	const filters = $derived(scheduleFiltersFromParameters(page.url.searchParams));
	// The picker's gate is the roster read's (`+page.ts`), so it holds its
	// place before the Doulas land.
	const isDoulaPickerOffered = $derived(
		isOwnerOrAdmin((page.data as { session: PracticeSession }).session)
	);

	/*
	 * Grows from the landed read's first page. `loadPage` reads the landed
	 * read's own filters, so "Load more" continues the narrowing its rows
	 * came from.
	 */
	const read = $derived(data);
	const schedule = new PaginatedList<ScheduledVisit>({
		first: untrack(() => read?.page) ?? { items: [], hasMore: false },
		loadPage: (cursor) =>
			loadPracticeSchedule(apiFetchWithSession, page.params.practiceId!, read?.filters ?? filters, cursor),
		failureMessage: 'Failed to load more scheduled visits'
	});

	let fromValue = $state(untrack(() => filters.from));
	let toValue = $state(untrack(() => filters.to));
	let staffIdValue = $state(untrack(() => filters.staffId ?? ''));

	/*
	 * A narrowing change is a navigation, so `load` runs again and hands
	 * this component a fresh first page. `PaginatedList` owns its own
	 * items, so without this it would keep the previous narrowing's rows.
	 * `reset` abandons anything in flight at the same time.
	 */
	$effect(() => {
		const landed = read;
		if (landed) untrack(() => schedule.reset(landed.page));
	});

	$effect(() => {
		const narrowing = filters;
		untrack(() => {
			fromValue = narrowing.from;
			toValue = narrowing.to;
			staffIdValue = narrowing.staffId ?? '';
		});
	});

	async function applyNarrowing(event: SubmitEvent) {
		event.preventDefault();
		// Before the navigation, so a "Load more" already in flight is
		// abandoned rather than merged into the new narrowing's list --
		// the Clients list's own ordering (#499).
		schedule.abandon();
		await goto(
			scheduleHref(basePath, { from: fromValue, to: toValue, staffId: staffIdValue || undefined })
		);
	}

	const summary = $derived(
		read ? scheduleResultSummary(schedule.items.length, schedule.hasMore) : 'Loading your schedule'
	);
	const doulaOptions = $derived([{ value: '', label: 'Every doula' }, ...(read?.doulas ?? [])]);

	/*
	 * Client first, because `DataTable` puts the row's link on its first
	 * column and only there: a link named "Sep 12, 2026, 10:30am" says
	 * nothing about where it goes, and every other Practice-wide list in
	 * the product (Contracts, Invoices, Clients) already puts the way in
	 * on the Client's name. It also keeps `When` out of that branch, which
	 * is what lets it stay a real `<time datetime>` carrying the exact
	 * instant underneath the words (ADR-0022) -- the link branch renders
	 * plain text and would drop it.
	 *
	 * The list's ORDER is still the Visit's instant, which is what #263
	 * asks for; that is the endpoint's `ORDER BY`, not a column position.
	 */
	const columns = $derived([
		{ label: 'Client', accessor: (visit: ScheduledVisit) => visit.clientName },
		{
			label: 'When',
			accessor: (visit: ScheduledVisit) => formatScheduledVisit(visit.scheduledAt),
			variant: 'meta' as const,
			datetimeAccessor: (visit: ScheduledVisit) => visit.scheduledAt
		},
		{ label: 'Doula', accessor: (visit: ScheduledVisit) => visit.staffName }
	]);

	function engagementHref(visit: ScheduledVisit): string {
		return resolve('/practices/[practiceId]/engagements/[engagementId]', {
			practiceId,
			engagementId: visit.engagementId
		});
	}
</script>

<ListPage title="Schedule">
	{#snippet intro()}
		<Text
			text="Every visit this practice has scheduled, soonest first, across all clients and doulas. Open one to reach the engagement it belongs to."
			tone="muted"
		/>
	{/snippet}

	{#snippet content()}
		<FilterBar
			action={basePath}
			onSubmit={applyNarrowing}
			legend="Narrow the schedule"
			status={summary}
			empty={read !== undefined && schedule.items.length === 0}
		>
			<LabeledField label="From">
				{#snippet children({ id, describedBy, invalid })}
					<TextInput
						{id}
						{describedBy}
						{invalid}
						type="date"
						name="from"
						value={fromValue}
						onInput={(value) => (fromValue = value)}
					/>
				{/snippet}
			</LabeledField>
			<LabeledField label="To">
				{#snippet children({ id, describedBy, invalid })}
					<TextInput
						{id}
						{describedBy}
						{invalid}
						type="date"
						name="to"
						value={toValue}
						onInput={(value) => (toValue = value)}
					/>
				{/snippet}
			</LabeledField>
			{#if isDoulaPickerOffered}
				<LabeledField label="Doula" width={20}>
					{#snippet children({ id, describedBy, invalid, width })}
						<Select
							{id}
							{describedBy}
							{invalid}
							{width}
							name="staffId"
							options={doulaOptions}
							disabled={!read}
							bind:value={staffIdValue}
						/>
					{/snippet}
				</LabeledField>
			{/if}
		</FilterBar>

		{#if read}
			<DataTable
				{columns}
				rows={schedule.items}
				rowHref={engagementHref}
				hasMore={schedule.hasMore}
				onLoadMore={() => schedule.loadMore()}
				isLoadingMore={schedule.isLoadingMore}
				emptyMessage="No visits are scheduled in this range. Widen the dates, or choose every doula, to see more."
			/>
		{:else}
			<Skeleton variant="row" lines={6} label="Loading your schedule" />
		{/if}

		{#if schedule.loadMoreError}
			<Notice message={schedule.loadMoreError} variant="error" />
		{/if}
	{/snippet}
</ListPage>
