<script lang="ts">
	/*
	 * PROTOTYPE -- throwaway, wayfinder ticket #983. Three variants of the
	 * Client's money surface, switchable via `?variant=`, mounted inside a
	 * replica of the portal Engagement hub (`RecordDetail`, the Template
	 * that page really uses) so each one is judged against the chrome and
	 * the density it would actually sit in. `?state=` walks the six cases
	 * the ticket names.
	 *
	 * It lives under /style-guide because that is this repo's own way of
	 * seeing states render without an auth session and a live API -- and
	 * there is no money endpoint to drive the real portal page from yet.
	 * Nothing here is production code.
	 */
	import { goto } from '$app/navigation';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import RecordDetail from '#lib/components/templates/RecordDetail.svelte';
	import VariantA from './VariantA.svelte';
	import VariantB from './VariantB.svelte';
	import VariantC from './VariantC.svelte';
	import { states, type StateKey } from './fixture.js';

	const variants = [
		{ key: 'A', name: 'Section on the hub, two lists', component: VariantA },
		{ key: 'B', name: 'Own route, index then detail', component: VariantB },
		{ key: 'C', name: 'Statement: one list, in time order', component: VariantC }
	] as const;

	const initial = new URLSearchParams(globalThis.location?.search ?? '');
	let variantKey = $state(initial.get('variant') ?? 'A');
	let stateKey = $state<StateKey>((initial.get('state') as StateKey) ?? 'deposit-and-balance');

	const current = $derived(variants.find((each) => each.key === variantKey) ?? variants[0]);
	const currentState = $derived(states.find((each) => each.key === stateKey) ?? states[0]);

	$effect(() => {
		const parameters = new URLSearchParams({ variant: current.key, state: currentState.key });
		void goto(`?${parameters}`, { replaceState: true, noScroll: true, keepFocus: true });
	});

	function cycle(step: number) {
		const index = variants.findIndex((each) => each.key === variantKey);
		variantKey = variants[(index + step + variants.length) % variants.length].key;
	}

	function onKeydown(event: KeyboardEvent) {
		const target = event.target as HTMLElement | null;
		if (target && ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)) return;
		if (target?.isContentEditable) return;
		if (event.key === 'ArrowLeft') cycle(-1);
		if (event.key === 'ArrowRight') cycle(1);
	}

	const summaryItems = $derived([
		{ label: 'Status', value: currentState.engagementStatus === 'completed' ? 'Care complete' : 'Active' },
		{ label: 'Due date', value: 'December 12, 2027' }
	]);
</script>

<svelte:window onkeydown={onKeydown} />

{#snippet summary()}
	<DescriptionList items={summaryItems} />
{/snippet}

{#snippet money()}
	<current.component money={currentState} />
{/snippet}

{#snippet activity()}
	<Text
		text="(#486's activity ledger sits here on the real hub, behind a closed disclosure.)"
		tone="muted"
	/>
{/snippet}

<RecordDetail
	title="Your care"
	serviceName="Finger Lakes Birth Collective"
	{summary}
	sections={[
		{ heading: 'Invoices', content: money },
		{ heading: 'Everything that has happened', content: activity }
	]}
/>

<div class="switcher">
	<button type="button" onclick={() => cycle(-1)} aria-label="Previous variant">←</button>
	<span class="label">{current.key} — {current.name}</span>
	<button type="button" onclick={() => cycle(1)} aria-label="Next variant">→</button>

	<label class="state">
		<span>State</span>
		<select bind:value={stateKey}>
			{#each states as option (option.key)}
				<option value={option.key}>{option.name}</option>
			{/each}
		</select>
	</label>
</div>

<div class="banner">
	<Heading level={2} variant="card" text="PROTOTYPE — #983, throwaway" />
</div>

<style>
	.banner {
		position: fixed;
		inset-block-start: 0;
		inset-inline: 0;
		z-index: 20;
		padding: var(--space-1) var(--space-3);
		background: var(--color-warning);
		text-align: center;
	}

	.switcher {
		position: fixed;
		inset-block-end: var(--space-4);
		inset-inline: var(--space-3);
		z-index: 20;
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: center;
		gap: var(--space-2) var(--space-3);
		margin-inline: auto;
		inline-size: fit-content;
		max-inline-size: 100%;
		padding: var(--space-2) var(--space-3);
		border-radius: var(--radius-pill);
		background: var(--color-on-surface);
		color: var(--color-surface);
		box-shadow: var(--shadow-lg, 0 4px 16px rgb(0 0 0 / 30%));
	}

	.switcher button {
		border: 0;
		border-radius: var(--radius-pill);
		padding: var(--space-1) var(--space-3);
		background: none;
		color: inherit;
		font: inherit;
		cursor: pointer;
	}

	.label {
		font-size: var(--text-body-sm-size);
	}

	.state {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		font-size: var(--text-meta-size);
	}
</style>
