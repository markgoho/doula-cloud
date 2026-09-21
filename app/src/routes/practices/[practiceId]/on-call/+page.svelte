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
	 * `LabeledField` + `TextInput` for the narrowing.
	 *
	 * The narrowing is a real `<form method="get">`: Enter submits it,
	 * the browser's own date controls do the entry, and the fields are
	 * named so the form reaches the right URL with no script running at
	 * all (Rule of Least Power). `onsubmit` upgrades that to a
	 * client-side navigation rather than replacing it.
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
		rosterHref,
		rosterSummary,
		type CoverageGap,
		type DayRange,
		type NoWindowRow,
		type RosterWindow
	} from '#lib/onCall.js';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import ListPage from '#lib/components/templates/ListPage.svelte';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();

	const practiceId = $derived(page.params.practiceId!);
	const basePath = $derived(resolve('/practices/[practiceId]/on-call', { practiceId }));

	// The form's own values, seeded from the URL and put back in step
	// after every navigation -- a reader edits them before she submits.
	let fromValue = $state(untrack(() => data.range.from));
	let toValue = $state(untrack(() => data.range.to));
	$effect(() => {
		const range = data.range;
		untrack(() => {
			fromValue = range.from;
			toValue = range.to;
		});
	});

	async function applyRange(event: SubmitEvent) {
		event.preventDefault();
		await goto(rosterHref(basePath, { from: fromValue, to: toValue }));
	}

	const summary = $derived(rosterSummary(data.roster));
	const stretched = $derived(doublyBooked(data.roster));

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
			accessor: (row: RosterWindow) => row.onCall.map((doula) => doula.name).join(', '),
			content: onCallCell
		},
		{
			label: 'Window',
			accessor: (row: RosterWindow) => dayRange(row.window),
			variant: 'meta' as const
		},
		{ label: 'Cover', accessor: describeCoverage, content: coverCell }
	]);

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

{#snippet coverCell(row: RosterWindow)}
	<div class="cover">
		<Badge label={describeCoverage(row)} variant={row.uncovered ? 'warning' : 'success'} />
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
		<!-- stacked-form:ignore: #1093 -- a `method="get"` narrowing control, not a run of fields. Its two dates sit side by side and wrap on their own, and `StackedForm` would both stack them and take away the GET submission this form is built on. -->
		<form method="get" action={basePath} onsubmit={applyRange}>
			<fieldset>
				<legend>Choose the days</legend>
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
					<div class="apply">
						<Button type="submit" label="Show" />
					</div>
				</div>
			</fieldset>
		</form>

		<p aria-live="polite" class="summary">{summary}</p>

		<!--
			hasMore is false and says something true: the roster is one whole
			answer for the days chosen, not a page of a longer list. What
			bounds it is the range itself, which the BFF caps at 92 days, and
			the Practice's live births inside it -- a cursor here would break
			the two things this screen exists to total, the holes and each
			Doula's concurrent windows, since both are counted over the whole
			answer rather than over whatever page arrived.
		-->
		<DataTable
			{columns}
			rows={data.roster.windows}
			rowHref={engagementHref}
			hasMore={false}
			emptyMessage="No birth is on call in these days. Widen the dates to see more."
		/>

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
			<ul class="doulas">
				{#each data.roster.doulas as doula (doula.staffId)}
					<li>
						<span class="name">{doula.name}</span>
						{#if doula.concurrentWindows !== undefined}
							<Badge
								label={doula.concurrentWindows === 1
									? '1 birth'
									: `${doula.concurrentWindows} births`}
								variant={doula.concurrentWindows > 1 ? 'warning' : 'neutral'}
							/>
						{/if}
						<Badge label={doula.available ? 'Available' : 'Away'} variant={doula.available ? 'success' : 'neutral'} />
					</li>
				{/each}
			</ul>
		</section>
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
		   comfortable width and takes its own row the moment the space
		   this form is given cannot hold two side by side. */
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

		.apply {
			flex: 0 0 auto;
		}

		.summary {
			margin: 0;
			color: var(--color-on-surface-variant);
			font-size: var(--text-small-size);
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

		/* The doula list wraps into as many columns as the space holds,
		   one per row at 320px, with no viewport query. */
		.doulas {
			flex-direction: row;
			flex-wrap: wrap;
			gap: var(--space-2) var(--space-4);
		}

		.doulas li {
			display: flex;
			align-items: center;
			gap: var(--space-2);
			flex: 1 1 14rem;
		}

		.name {
			font-weight: var(--font-weight-medium);
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
