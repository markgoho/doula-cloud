<script lang="ts">
	/*
	 * The Practice-wide schedule (#263). Before it, "who is covering whom,
	 * and when" had no surface at all: a Staff member opened one
	 * Engagement page per Client and held the answer in her head.
	 *
	 * Composes existing components only, the same as the Practice-wide
	 * Contract list this follows (#273): `ListPage` for the frame,
	 * `DataTable` for the book, `LabeledField` + `TextInput`/`Select` for
	 * the narrowing. The one atom this screen changed is `Select`, which
	 * now takes `{value, label}` options as well as bare strings -- two
	 * Doulas at one agency can share a name, so the filter has to store an
	 * id.
	 *
	 * The narrowing is a real `<form method="get">`: Enter submits it, the
	 * browser's own controls do the date entry, and the fields are named
	 * so the form still reaches the right URL with no script running at
	 * all. `onsubmit` upgrades that to a client-side navigation rather
	 * than replacing it (Rule of Least Power).
	 */
	import { untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { formatScheduledVisit } from '#lib/dates.js';
	import { PaginatedList } from '#lib/paginatedList.svelte.js';
	import {
		loadPracticeSchedule,
		scheduleHref,
		scheduleResultSummary,
		type ScheduledVisit
	} from '#lib/visitSchedule.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Select from '#lib/components/atoms/Select.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import ListPage from '#lib/components/templates/ListPage.svelte';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();

	const practiceId = $derived(page.params.practiceId!);
	const basePath = $derived(
		resolve('/practices/[practiceId]/schedule', { practiceId: page.params.practiceId! })
	);

	/*
	 * Takes the load's first page and grows from there. `loadPage` reads
	 * `data.filters` at call time rather than closing over the values it
	 * was built with, so "Load more" always continues the narrowing
	 * currently on screen.
	 */
	const schedule = new PaginatedList<ScheduledVisit>({
		first: untrack(() => data.page),
		loadPage: (cursor) =>
			loadPracticeSchedule(apiFetchWithSession, page.params.practiceId!, data.filters, cursor),
		failureMessage: 'Failed to load more scheduled visits'
	});

	// The form's own values, seeded from the URL. Separate from
	// `data.filters` because a reader edits them before she submits; the
	// effect below is what puts them back in step after a navigation.
	let fromValue = $state(untrack(() => data.filters.from));
	let toValue = $state(untrack(() => data.filters.to));
	let staffIdValue = $state(untrack(() => data.filters.staffId ?? ''));

	/*
	 * A narrowing change is a navigation, so `load` runs again and hands
	 * this component a fresh first page. Without this the list would keep
	 * the previous narrowing's rows forever -- `PaginatedList` is
	 * constructed once and owns its own items from then on. `reset`
	 * abandons anything in flight at the same time, so a "Load more"
	 * already running against the old filter can never merge into the new
	 * one.
	 */
	$effect(() => {
		const first = data.page;
		const filters = data.filters;
		untrack(() => {
			schedule.reset(first);
			fromValue = filters.from;
			toValue = filters.to;
			staffIdValue = filters.staffId ?? '';
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

	const summary = $derived(scheduleResultSummary(schedule.items.length, schedule.hasMore));

	/*
	 * Client first, because `DataTable` puts the row's link on its first
	 * column and only there: a link named "12 Sep 2026, 10:30am" says
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
		<form method="get" action={basePath} onsubmit={applyNarrowing}>
			<fieldset>
				<legend>Narrow the schedule</legend>
				<div class="controls">
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
					{#if data.doulas.length > 0}
						<LabeledField label="Doula">
							{#snippet children({ id, describedBy, invalid })}
								<Select
									{id}
									{describedBy}
									{invalid}
									name="staffId"
									options={[{ value: '', label: 'Every doula' }, ...data.doulas]}
									bind:value={staffIdValue}
								/>
							{/snippet}
						</LabeledField>
					{/if}
					<div class="apply">
						<Button type="submit" label="Apply" />
					</div>
				</div>
			</fieldset>
		</form>

		<p aria-live="polite" class="summary">{summary}</p>

		<DataTable
			{columns}
			rows={schedule.items}
			rowHref={engagementHref}
			hasMore={schedule.hasMore}
			onLoadMore={() => schedule.loadMore()}
			isLoadingMore={schedule.isLoadingMore}
			emptyMessage="No visits are scheduled in this range. Widen the dates, or choose every doula, to see more."
		/>

		{#if schedule.loadMoreError}
			<Notice message={schedule.loadMoreError} variant="error" />
		{/if}
	{/snippet}
</ListPage>

<style>
	@layer components {
		fieldset {
			margin: 0;
			padding: 0;
			border: 0;
		}

		legend {
			padding: 0;
			font-size: var(--text-body-size);
			font-weight: var(--font-weight-bold);
			color: var(--color-on-surface);
		}

		/* Intrinsic, not a breakpoint (ADR-0024): each control asks for a
		   comfortable width and wraps to its own row the moment the space
		   this form is given cannot hold two side by side. At 320px that is
		   one control per row; in a wide column all four sit together. No
		   viewport media query, and the same behaviour whether the form is
		   the page or a panel inside something else. */
		.controls {
			display: flex;
			flex-wrap: wrap;
			gap: var(--space-4);
			align-items: end;
			margin-block-start: var(--space-3);
		}

		.controls > :global(*) {
			flex: 1 1 12rem;
		}

		/* The button asks for no growth: it is as wide as its own words,
		   and drops onto its own row only when nothing else fits beside
		   it. */
		.apply {
			flex: 0 0 auto;
		}

		.summary {
			margin: 0;
			color: var(--color-on-surface-variant);
			font-size: var(--text-small-size);
		}
	}
</style>
