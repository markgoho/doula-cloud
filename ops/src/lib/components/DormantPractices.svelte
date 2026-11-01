<script lang="ts">
	import Button from '$ui/atoms/Button.svelte';
	import { onMount } from 'svelte';
	import type { FormError } from '../authRefusal.ts';
	import { dormantFacts, readDormantPractices, type DormantPractice } from '../dormantPractices.ts';
	import ErrorSummary from '$ui/molecules/ErrorSummary.svelte';
	import SummaryList from '$ui/molecules/SummaryList.svelte';

	let { onsignedout, fetcher = fetch }: { onsignedout: () => void; fetcher?: typeof fetch } = $props();

	let practices = $state<DormantPractice[]>([]);
	let nextCursor = $state<string | undefined>();
	let hasLoaded = $state(false);
	let isLoading = $state(false);
	let errors = $state<FormError[]>([]);

	async function load(cursor?: string) {
		isLoading = true;
		try {
			const outcome = await readDormantPractices(cursor, fetcher);
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

	onMount(() => load());
</script>

<ErrorSummary {errors} />

{#if hasLoaded}
	{#if practices.length === 0}
		<p>No Practice holds Credits that nobody has used in two years.</p>
	{:else}
		<ul class="practices" role="list">
			{#each practices as practice (practice.id)}
				<li>
					<stack-l>
						<h2>{practice.name}</h2>
						<SummaryList rows={dormantFacts(practice)} />
					</stack-l>
				</li>
			{/each}
		</ul>
		{#if nextCursor}
			<Button label="Show more Practices" loading={isLoading} onClick={() => load(nextCursor)} />
		{/if}
	{/if}
{/if}

<style>
	ul {
		margin: 0;
		padding: 0;
		list-style: none;
	}

	.practices > li {
		padding-block: var(--space-4);
		border-block-start: var(--border-thin) solid var(--color-outline-variant);
	}
</style>
