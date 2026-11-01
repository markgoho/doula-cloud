<script lang="ts">
	import Button from '$ui/atoms/Button.svelte';
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import type { FormError } from '../authRefusal.ts';
	import { readFeedback, sentAt, summaryFacts, type FeedbackList, type FeedbackSummary } from '../feedback.ts';
	import ErrorSummary from '$ui/molecules/ErrorSummary.svelte';
	import SummaryList from '$ui/molecules/SummaryList.svelte';

	let {
		list,
		onsignedout,
		fetcher = fetch
	}: { list: FeedbackList; onsignedout: () => void; fetcher?: typeof fetch } = $props();

	let pieces = $state<FeedbackSummary[]>([]);
	let nextCursor = $state<string | undefined>();
	let hasLoaded = $state(false);
	let isLoading = $state(false);
	let errors = $state<FormError[]>([]);

	async function load(cursor?: string) {
		isLoading = true;
		try {
			const outcome = await readFeedback(list, cursor, fetcher);
			if (outcome.kind === 'signed-out') return onsignedout();
			if (outcome.kind === 'refused') {
				errors = outcome.errors;
				return;
			}
			/* v8 ignore next -- a read asks no fresh code, so 'step-up' never arrives */
			if (outcome.kind === 'ok') {
				errors = [];
				pieces = [...pieces, ...outcome.value.items];
				nextCursor = outcome.value.hasMore ? outcome.value.nextCursor : undefined;
				hasLoaded = true;
			}
		} finally {
			isLoading = false;
		}
	}

	onMount(() => load());
</script>

<ErrorSummary {errors} />

{#if hasLoaded}
	{#if pieces.length === 0}
		<p>{list === 'unopened' ? 'Every issue has opened, or is waiting to open.' : 'No Feedback has been sent yet.'}</p>
	{:else}
		<ul class="pieces" role="list">
			{#each pieces as piece (piece.id)}
				<li>
					<stack-l>
						<h3><a href={resolve('/feedback/[feedbackId]', { feedbackId: piece.id })}>{sentAt(piece.sentAt)}</a></h3>
						<SummaryList rows={summaryFacts(piece)} />
					</stack-l>
				</li>
			{/each}
		</ul>
		{#if nextCursor}
			<Button label="Show more Feedback" loading={isLoading} onClick={() => load(nextCursor)} />
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
