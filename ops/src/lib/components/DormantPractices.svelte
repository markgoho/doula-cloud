<script lang="ts">
	import Button from '$ui/atoms/Button.svelte';
	import { onMount } from 'svelte';
	import { dormantFacts, readDormantPractices, type DormantPractice } from '../dormantPractices.ts';
	import { PagedList } from '../pagedList.svelte.ts';
	import ErrorSummary from '$ui/molecules/ErrorSummary.svelte';
	import SummaryList from '$ui/molecules/SummaryList.svelte';

	let { onsignedout, fetcher = fetch }: { onsignedout: () => void; fetcher?: typeof fetch } = $props();

	const practices = new PagedList<DormantPractice>((cursor) => readDormantPractices(cursor, fetcher), () => onsignedout());

	onMount(() => practices.load());
</script>

<ErrorSummary errors={practices.errors} />

{#if practices.state.status === 'loaded'}
	{#if practices.state.items.length === 0}
		<p>No Practice holds Credits that nobody has used in two years.</p>
	{:else}
		<ul class="practices" role="list">
			{#each practices.state.items as practice (practice.id)}
				<li>
					<stack-l>
						<h2>{practice.name}</h2>
						<SummaryList rows={dormantFacts(practice)} />
					</stack-l>
				</li>
			{/each}
		</ul>
		{#if practices.state.nextCursor}
			<Button label="Show more Practices" loading={practices.isLoadingMore} onClick={() => practices.more()} />
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
