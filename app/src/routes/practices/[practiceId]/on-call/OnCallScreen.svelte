<script lang="ts">
	/*
	 * Who is on call (#1093). The question an agency owner asks every
	 * evening -- who is on call tonight, and is anyone covering two
	 * births at once -- with a hole shown as a hole rather than hidden.
	 *
	 * Composes existing components only, the same way the Practice-wide
	 * schedule (#263) does: `ListPage` for the frame, `DataTable` for the
	 * book (its record view is what carries a dense roster at 320px, and
	 * its per-column snippets are what let one row say three things), and
	 * `FilterBar` with `LabeledField` + `TextInput` for the narrowing
	 * (#2026).
	 *
	 * The narrowing is `FilterBar`'s real `<form method="get">`;
	 * `applyRange` upgrades it to a client-side navigation rather than
	 * replacing it (Rule of Least Power).
	 *
	 * What a reader sees is the BFF's answer and nothing more: a
	 * contractor Doula's roster carries only births she is on, decided in
	 * the query, so this screen hides nothing and needs to hide nothing.
	 */
	import { untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '#lib/appState.svelte.js';
	import { formatCalendarDay, formatInstant } from '#lib/dates.js';
	import {
		describeCoverage,
		describeNoWindow,
		doublyBooked,
		rangeFromParameters,
		rosterHref,
		rosterSummary,
		type CoverageGap,
		type DayRange,
		type NoWindowRow,
		type RosterDoula,
		type RosterWindow
	} from '#lib/onCall.js';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import FilterBar from '#lib/components/molecules/FilterBar.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import ListPage from '#lib/components/templates/ListPage.svelte';
	import type { OnCallPageData } from './+page.js';

	/*
	 * `data` is the read once it lands (#2255). Until then the screen draws
	 * the days the URL names and a row skeleton where the births will land.
	 */
	let { data }: { data?: OnCallPageData } = $props();

	const practiceId = $derived(page.params.practiceId!);
	const basePath = $derived(resolve('/practices/[practiceId]/on-call', { practiceId }));
	const range = $derived(rangeFromParameters(page.url.searchParams));

	// The form's own values, seeded from the URL and put back in step
	// after every navigation -- a reader edits them before she submits.
	let fromValue = $state(untrack(() => range.from));
	let toValue = $state(untrack(() => range.to));
	$effect(() => {
		const days = range;
		untrack(() => {
			fromValue = days.from;
			toValue = days.to;
		});
	});

	async function applyRange(event: SubmitEvent) {
		event.preventDefault();
		await goto(rosterHref(basePath, { from: fromValue, to: toValue }));
	}

	const summary = $derived(data ? rosterSummary(data.roster) : 'Loading who is on call');
	const stretched = $derived(data ? doublyBooked(data.roster) : []);

	function dayRange(range: DayRange): string {
		return range.start === range.end
			? formatCalendarDay(range.start)
			: `${formatCalendarDay(range.start)} – ${formatCalendarDay(range.end)}`;
	}

	function gapWhen(gap: CoverageGap): string {
		return `${formatInstant(gap.startsAt)} – ${formatInstant(gap.endsAt)}`;
	}

	function engagementHref(row: { engagementId: string }): string {
		return resolve('/practices/[practiceId]/engagements/[engagementId]', {
			practiceId,
			engagementId: row.engagementId
		});
	}

	const columns = $derived([
		{ label: 'Client', accessor: (row: RosterWindow) => row.clientName },
		{
			label: 'On call',
			// "Nobody" as the cell says it: an empty accessor leaves the row
			// out of the record card (#2022), and nobody on call is the fact.
			accessor: (row: RosterWindow) => row.onCall.map((doula) => doula.name).join(', ') || 'Nobody',
			content: onCallCell
		},
		{
			label: 'Window',
			accessor: (row: RosterWindow) => dayRange(row.window),
			variant: 'meta' as const
		},
		{ label: 'Cover', accessor: describeCoverage, content: coverCell }
	]);

	/*
	 * The Doulas as a table (#2031): a roster is a list of rows, and a
	 * badge marks only the exception -- more than one birth in these
	 * days, or away. One birth and "Available" are what nearly everybody
	 * is, so they are plain text. `concurrentWindows` is absent where the
	 * reader may not hold it (a contractor reading about a colleague): the
	 * cell is empty and the record card leaves the row out (#2022).
	 *
	 * The values are a count and "Yes" under headers that say what they
	 * count, not "0 births" and "Available": walked at 320px, the longer
	 * words broke mid-word ("Avail-able") in a 288px table, and the short
	 * ones keep all three columns a table at every width. The header is
	 * "Births", not "Births in these days": the table shares out its
	 * spare width by each column's widest content, and the longer header
	 * took the room the names needed, splitting them mid-word at 320px.
	 * The days are the filter's, and the line above the table names them.
	 */
	function birthsText(doula: RosterDoula): string {
		return doula.concurrentWindows === undefined ? '' : String(doula.concurrentWindows);
	}

	const doulaColumns = [
		{ label: 'Doula', accessor: (doula: RosterDoula) => doula.name },
		{ label: 'Births', accessor: birthsText, content: birthsCell, numeric: true },
		{
			label: 'Available',
			accessor: (doula: RosterDoula) => (doula.available ? 'Yes' : 'Away'),
			content: availableCell
		}
	];

	const noWindowColumns = [
		{ label: 'Client', accessor: (row: NoWindowRow) => row.clientName },
		{ label: 'Why', accessor: (row: NoWindowRow) => describeNoWindow(row.reason), variant: 'muted' as const }
	];
</script>

{#snippet onCallCell(row: RosterWindow)}
	{#if row.onCall.length === 0}
		<Text text="Nobody" tone="muted" />
	{:else}
		<ul>
			{#each row.onCall as doula (doula.staffId)}
				<li>
					{doula.name}{#if doula.narrowed}<span class="span">{dayRange({ start: doula.from, end: doula.to })}</span>{/if}
				</li>
			{/each}
		</ul>
	{/if}
{/snippet}

{#snippet birthsCell(doula: RosterDoula)}
	{#if (doula.concurrentWindows ?? 0) > 1}
		<Badge label={birthsText(doula)} variant="warning" />
	{:else}
		{birthsText(doula)}
	{/if}
{/snippet}

{#snippet availableCell(doula: RosterDoula)}
	{#if doula.available}
		Yes
	{:else}
		<Badge label="Away" variant="neutral" />
	{/if}
{/snippet}

{#snippet coverCell(row: RosterWindow)}
	<div class="cover">
		<!-- A badge for the gap alone (#2031): "Covered" is the normal
		     case, and a badge on every row would spend the one a gap needs. -->
		{#if row.uncovered}
			<Badge label={describeCoverage(row)} variant="warning" />
		{:else}
			<Text text={describeCoverage(row)} />
		{/if}
		{#each row.unstaffedDays as hole (hole.start)}
			<Text text={`Nobody on call ${dayRange(hole)}`} tone="muted" />
		{/each}
		{#each row.gaps as gap (gap.id)}
			<Text
				text={gap.coveringStaffName
					? `${gap.staffName} away ${gapWhen(gap)} — ${gap.coveringStaffName} covering`
					: `${gap.staffName} away ${gapWhen(gap)} — nobody covering`}
				tone="muted"
			/>
		{/each}
	</div>
{/snippet}

<ListPage title="On call">
	{#snippet intro()}
		<Text
			text="Every birth on call in these days, who is on call for it, and where cover is still needed. Open one to reach the birth it belongs to."
			tone="muted"
		/>
	{/snippet}

	{#snippet content()}
		<FilterBar
			action={basePath}
			onSubmit={applyRange}
			legend="Choose the days"
			status={summary}
			empty={data !== undefined && data.roster.windows.length === 0}
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
		</FilterBar>

		{#if data}
			<!--
				hasMore is false and says something true: the roster is one whole
				answer for the days chosen, not a page of a longer list. What
				bounds it is the range itself, which the BFF caps at 92 days, and
				the Practice's live births inside it -- a cursor here would break
				the two things this screen exists to total, the holes and each
				Doula's concurrent windows, since both are counted over the whole
				answer rather than over whatever page arrived.
			-->
			<div class="roster-table">
				<DataTable
					{columns}
					rows={data.roster.windows}
					rowHref={engagementHref}
					hasMore={false}
					emptyMessage="No birth is on call in these days. Widen the dates to see more."
				/>
			</div>

			{#if data.roster.noWindow.length > 0}
				<section>
					<Heading level={2} text="No window yet" />
					<Text
						text="These births are active and have a doula, and the product cannot say when on call starts for them."
						tone="muted"
					/>
					<DataTable
						columns={noWindowColumns}
						rows={data.roster.noWindow}
						rowHref={engagementHref}
						hasMore={false}
						emptyMessage="None."
					/>
				</section>
			{/if}

			<section>
				<Heading level={2} text="Doulas" />
				{#if stretched.length > 0}
					<Text
						text={`${stretched.map((doula) => doula.name).join(', ')} ${stretched.length === 1 ? 'is' : 'are'} on more than one birth in these days.`}
					/>
				{/if}
				<div class="doula-table">
					<DataTable
						columns={doulaColumns}
						rows={data.roster.doulas}
						hasMore={false}
						emptyMessage="No doula is on this Practice's roster."
					/>
				</div>
			</section>
		{:else}
			<Skeleton variant="row" lines={6} label="Loading who is on call" />
		{/if}
	{/snippet}
</ListPage>

<style>
	@layer components {
		/* The on-call roster's content floor (#1945, #1981): below it the table's
		   cells wrap to more lines than at the old type floors, so it
		   shows its records -- the founder's answer on #1945. Measured on
		   this route's typical pilot screen, not its hostile one -- the
		   founder's answer of 2026-10-25 on #1981: a very long name or
		   address wraps one more line, and it does not change the layout.
		   Measured 2026-10-25 on the drag surface: its natural width fits
		   its column from 1353px on macOS/Chromium with no margin beyond it.
		   On the hostile fixture it was 137rem.
		   `DataTable.svelte`'s floor comment says why the floor is the
		   route's own. */
		.roster-table {
			container: roster-table / inline-size;
		}

		.roster-table > :global(*) {
			font-size: var(--text-body-size);
			--data-table-table-display: none;
			--data-table-records-display: block;
		}

		@container roster-table (min-width: 84.75rem) {
			.roster-table > :global(*) {
				--data-table-table-display: table;
				--data-table-records-display: none;
			}
		}

		section {
			display: flex;
			flex-direction: column;
			gap: var(--space-3);
		}

		ul {
			margin: 0;
			padding: 0;
			list-style: none;
			display: flex;
			flex-direction: column;
			gap: var(--space-1);
		}

		/* The Doulas table's floor (#2031, #2063): three short columns,
		   so it stays a table at every width. Fifteen record cards at
		   320px would be the same tall column of pills in another shape. */
		.doula-table > :global(*) {
			--data-table-table-display: table;
			--data-table-records-display: none;
		}

		/* `break-word` for the two short columns, not the cells' own
		   `anywhere` (#2031): `anywhere` lets a word's letters count as
		   break points when the table works out its column widths, so at
		   320px the table split "Available" in two. `break-word` sizes a
		   column to its longest word. The name column keeps `anywhere`, so
		   a long double-barreled name is what gives way when the room
		   runs out, and the table still fits its 288px. */
		.doula-table :global(:is(th, td):not(:first-child)) {
			overflow-wrap: break-word;
		}

		/* A narrowed doula's own days sit under her name rather than
		   beside it, so a long name and a long range never fight for one
		   line. */
		.span {
			display: block;
			color: var(--color-on-surface-variant);
			font-size: var(--text-small-size);
		}

		.cover {
			display: flex;
			flex-direction: column;
			gap: var(--space-1);
			align-items: start;
		}
	}
</style>
