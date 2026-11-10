<script lang="ts">
	import Button from '$ui/atoms/Button.svelte';
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { readFeedback, sentAt, summaryFacts, type FeedbackList, type FeedbackSummary } from '../feedback.ts';
	import { PagedList } from '../pagedList.svelte.ts';
	import ErrorSummary from '$ui/molecules/ErrorSummary.svelte';
	import SummaryList from '$ui/molecules/SummaryList.svelte';

	let {
		list,
		onsignedout,
		fetcher = fetch
	}: { list: FeedbackList; onsignedout: () => void; fetcher?: typeof fetch } = $props();

	const pieces = new PagedList<FeedbackSummary>((cursor) => readFeedback(list, cursor, fetcher), () => onsignedout());

	onMount(() => pieces.load());
</script>

<ErrorSummary errors={pieces.errors} />

{#if pieces.state.status === 'loaded'}
	{#if pieces.state.items.length === 0}
		<p>{list === 'unopened' ? 'Every issue has opened, or is waiting to open.' : 'No Feedback has been sent yet.'}</p>
	{:else}
		<ul class="pieces" role="list">
			{#each pieces.state.items as piece (piece.id)}
				<li>
					<stack-l>
						<h3><a href={resolve('/feedback/[feedbackId]', { feedbackId: piece.id })}>{sentAt(piece.sentAt)}</a></h3>
						<SummaryList rows={summaryFacts(piece)} />
					</stack-l>
				</li>
			{/each}
		</ul>
		{#if pieces.state.nextCursor}
			<Button label="Show more Feedback" loading={pieces.isLoadingMore} onClick={() => pieces.more()} />
		{/if}
	{/if}
{/if}

<style>
	ul {
		margin: 0;
		padding: 0;
		list-style: none;
	}

	.pieces > li {
		padding-block: var(--space-4);
		border-block-start: var(--border-thin) solid var(--color-outline-variant);
	}

	a {
		color: var(--color-primary);
	}

	a:focus-visible {
		outline: var(--focus-ring-width) solid var(--color-primary);
		outline-offset: var(--focus-ring-offset);
	}
</style>
