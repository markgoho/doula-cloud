<script lang="ts">
	import Button from '$ui/atoms/Button.svelte';
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { sentAt } from '../feedback.ts';
	import { PagedList } from '../pagedList.svelte.ts';
	import { ACTION_LABELS, logDetail, readOperatorLog, subjectFact, type LogRow } from '../operatorLog.ts';
	import ErrorSummary from '$ui/molecules/ErrorSummary.svelte';
	import SummaryList from '$ui/molecules/SummaryList.svelte';

	let { onsignedout, fetcher = fetch }: { onsignedout: () => void; fetcher?: typeof fetch } = $props();

	const log = new PagedList<LogRow>((cursor) => readOperatorLog(cursor, fetcher), () => onsignedout());

	function facts(row: LogRow): { label: string; value: string }[] {
		const out = [
			{ label: 'Operator', value: row.operator },
			{ label: 'Time', value: sentAt(row.at) }
		];
		if (row.subject && row.subject.kind !== 'feedback') out.push(subjectFact(row.subject));
		const detail = logDetail(row);
		if (detail) out.push({ label: 'Details', value: detail });
		return out;
	}

	onMount(() => log.load());
</script>

<ErrorSummary errors={log.errors} />

{#if log.state.status === 'loaded'}
	{#if log.state.items.length === 0}
		<p>Nothing has been recorded yet.</p>
	{:else}
		<ul class="log" role="list">
			{#each log.state.items as row (row.id)}
				<li>
					<stack-l>
						<h2>{ACTION_LABELS[row.action] ?? row.action}</h2>
						<SummaryList rows={facts(row)} />
						{#if row.subject?.kind === 'feedback'}
							<!-- v8 ignore next: Svelte's compiled attribute-diffing branch for the href; a row's id never changes in place -->
							<a href={resolve('/feedback/[feedbackId]', { feedbackId: row.subject.id })}>Open the piece of Feedback</a>
						{/if}
					</stack-l>
				</li>
			{/each}
		</ul>
		{#if log.state.nextCursor}
			<Button label="Show more" loading={log.isLoadingMore} onClick={() => log.more()} />
		{/if}
	{/if}
{/if}

<style>
	ul {
		margin: 0;
		padding: 0;
		list-style: none;
	}

	.log > li {
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
