<script lang="ts">
	import Button from '$ui/atoms/Button.svelte';
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import type { FormError } from '../authRefusal.ts';
	import { practiceFacts, readPractices, type PracticeList, type PracticeRow } from '../practices.ts';
	import ErrorSummary from '$ui/molecules/ErrorSummary.svelte';
	import SummaryList from '$ui/molecules/SummaryList.svelte';

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

	onMount(() => load());
</script>

<ErrorSummary {errors} />

{#if hasLoaded}
	{#if practices.length === 0}
		<p>{list === 'deleted' ? 'No Practice has been deleted.' : 'No Practice has signed up yet.'}</p>
	{:else}
		<ul class="practices" role="list">
			{#each practices as practice (practice.id)}
				<li>
					<stack-l>
						<h2>{practice.name}</h2>
						<SummaryList rows={practiceFacts(practice)} />
						{#if list === 'live'}
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
						{/if}
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

	.acts {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-2) var(--space-4);
	}

	a {
		color: var(--color-primary);
	}

	a:focus-visible {
		outline: var(--focus-ring-width) solid var(--color-primary);
		outline-offset: var(--focus-ring-offset);
	}
</style>
