<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import type { FormError } from '../authRefusal.ts';
	import { sentAt } from '../feedback.ts';
	import { ACTION_LABELS, logDetail, readOperatorLog, subjectFact, type LogRow } from '../operatorLog.ts';
	import ErrorSummary from './ErrorSummary.svelte';
	import SummaryList from './SummaryList.svelte';

	let { onsignedout, fetcher = fetch }: { onsignedout: () => void; fetcher?: typeof fetch } = $props();

	let rows = $state<LogRow[]>([]);
	let nextCursor = $state<string | undefined>();
	let hasLoaded = $state(false);
	let isLoading = $state(false);
	let errors = $state<FormError[]>([]);

	function facts(row: LogRow): { key: string; value: string }[] {
		const out = [
			{ key: 'Operator', value: row.operator },
			{ key: 'Time', value: sentAt(row.at) }
		];
		if (row.subject && row.subject.kind !== 'feedback') out.push(subjectFact(row.subject));
		const detail = logDetail(row);
		if (detail) out.push({ key: 'Details', value: detail });
		return out;
	}

	async function load(cursor?: string) {
		isLoading = true;
		try {
			const outcome = await readOperatorLog(cursor, fetcher);
			if (outcome.kind === 'signed-out') return onsignedout();
			if (outcome.kind === 'refused') {
				errors = outcome.errors;
				return;
			}
			/* v8 ignore next -- a read asks no fresh code, so 'step-up' never arrives */
			if (outcome.kind === 'ok') {
				errors = [];
				rows = [...rows, ...outcome.value.items];
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
	{#if rows.length === 0}
		<p>Nothing has been recorded yet.</p>
	{:else}
		<ul class="log" role="list">
			{#each rows as row (row.id)}
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
		{#if nextCursor}
			<button class="ops-button" type="button" disabled={isLoading} onclick={() => load(nextCursor)}>Show more</button>
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
