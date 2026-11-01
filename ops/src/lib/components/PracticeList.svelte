<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import type { FormError } from '../authRefusal.ts';
	import { practiceFacts, readPractices, type PracticeList, type PracticeRow } from '../practices.ts';
	import ErrorSummary from '$ui/molecules/ErrorSummary.svelte';
	import DataTable from '$ui/organisms/DataTable.svelte';

	let {
		list,
		onsignedout,
		fetcher = fetch
	}: { list: PracticeList; onsignedout: () => void; fetcher?: typeof fetch } = $props();

	let practices = $state<PracticeRow[]>([]);
	let nextCursor = $state<string | undefined>();
	let hasLoaded = $state(false);
	let isLoading = $state(false);
	let errors = $state<FormError[]>([]);

	async function load(cursor?: string) {
		isLoading = true;
		try {
			const outcome = await readPractices(list, cursor, fetcher);
			if (outcome.kind === 'signed-out') return onsignedout();
			if (outcome.kind === 'refused') {
				errors = outcome.errors;
				return;
			}
			/* v8 ignore next -- a read asks no fresh code, so 'step-up' never arrives */
			if (outcome.kind === 'ok') {
				errors = [];
				practices = [...practices, ...outcome.value.items];
				nextCursor = outcome.value.hasMore ? outcome.value.nextCursor : undefined;
				hasLoaded = true;
			}
		} finally {
			isLoading = false;
		}
	}

	// The facts a Practice states, in the order practiceFacts gives them.
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
	}));

	const columns = [{ label: 'Practice', accessor: (practice: PracticeRow) => practice.name }, ...factColumns];

	onMount(() => load());
</script>

<ErrorSummary {errors} />

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

{#if hasLoaded}
	<DataTable
		{columns}
		rows={practices}
		rowActions={list === 'live' ? { label: 'Acts', content: acts as never } : undefined}
		hasMore={nextCursor !== undefined}
		onLoadMore={() => load(nextCursor)}
		isLoadingMore={isLoading}
		emptyMessage={list === 'deleted' ? 'No Practice has been deleted.' : 'No Practice has signed up yet.'}
	/>
{/if}

<style>
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
