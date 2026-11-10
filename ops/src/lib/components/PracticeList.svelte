<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { PagedList } from '../pagedList.svelte.ts';
	import { practiceFacts, readPractices, type PracticeList, type PracticeRow } from '../practices.ts';
	import ErrorSummary from '$ui/molecules/ErrorSummary.svelte';
	import DataTable from '$ui/organisms/DataTable.svelte';

	let {
		list,
		onsignedout,
		fetcher = fetch
	}: { list: PracticeList; onsignedout: () => void; fetcher?: typeof fetch } = $props();

	const practices = new PagedList<PracticeRow>((cursor) => readPractices(list, cursor, fetcher), () => onsignedout());

	// The facts a Practice states, in the order practiceFacts gives them. Source is not a column:
	// twelve columns do not fit 1440px without a word breaking (#2615), and Source reads "Not
	// recorded yet" for every Practice until it is captured; it belongs on the Practice's page.
	const factColumns = [
		'Signed up',
		'Owners',
		'Staff',
		'Stripe Connect',
		'Credits held',
		'Engagements started',
		'Founding grant',
		'Deletion',
		'Source',
		'Open Feedback'
	].map((label, index) => ({
		label,
		accessor: (practice: PracticeRow) => practiceFacts(practice)[index]!.value,
		numeric: ['Staff', 'Credits held', 'Engagements started', 'Open Feedback'].includes(label)
	})).filter((column) => column.label !== 'Source');

	const columns = [{ label: 'Practice', accessor: (practice: PracticeRow) => practice.name }, ...factColumns];

	onMount(() => practices.load());
</script>

<ErrorSummary errors={practices.errors} />

{#snippet acts(practice: PracticeRow)}
	<!-- v8 ignore start: Svelte's compiled attribute-diffing branches for the two hrefs; a Practice's id never changes in place -->
	<ul class="acts" role="list">
		<li>
			<a href={resolve('/practices/[practiceId]/refund', { practiceId: practice.id })}>
				Refund Credits<span class="visually-hidden"> to {practice.name}</span>
			</a>
		</li>
		{#if !practice.foundingGrant}
			<li>
				<a href={resolve('/practices/[practiceId]/founding-grant', { practiceId: practice.id })}>
					Give a founding grant<span class="visually-hidden"> to {practice.name}</span>
				</a>
			</li>
		{/if}
	</ul>
	<!-- v8 ignore stop -->
{/snippet}

{#if practices.state.status === 'loaded'}
	<div class="practice-table">
		<DataTable
			{columns}
			rows={practices.state.items}
			rowActions={list === 'live' ? { label: 'Acts', content: acts } : undefined}
			hasMore={practices.state.nextCursor !== undefined}
			onLoadMore={() => practices.more()}
			isLoadingMore={practices.isLoadingMore}
			emptyMessage={list === 'deleted' ? 'No Practice has been deleted.' : 'No Practice has signed up yet.'}
		/>
	</div>
{/if}

<style>
	@layer components {
		.practice-table {
			container: practice-table / inline-size;
		}

		.practice-table > :global(*) {
			--data-table-table-display: none;
			--data-table-records-display: block;
		}

		@container practice-table (min-width: 82rem) {
			.practice-table > :global(*) {
				--data-table-table-display: table;
				--data-table-records-display: none;
			}
		}
	}

	/* Unlayered, so it outranks DataTable's layered `anywhere`: a cell here
	   breaks at a space or a hyphen and never inside a word. */
	.practice-table :global(:is(th, td)) {
		max-inline-size: none;
		overflow-wrap: normal;
	}

	.acts {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-2) var(--space-4);
		margin: 0;
		padding: 0;
		list-style: none;
	}

	a {
		color: var(--color-primary);
	}

	a:focus-visible {
		outline: var(--focus-ring-width) solid var(--color-primary);
		outline-offset: var(--focus-ring-offset);
	}
</style>
