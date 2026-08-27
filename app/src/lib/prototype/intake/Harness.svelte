<script lang="ts">
	// PROTOTYPE (#372) -- throwaway. The dials above every variant, and the
	// state box below it. Everything the variants disagree about is a switch
	// here, so nothing is decided by defaulting.
	import type { Snippet } from 'svelte';
	import {
		cases,
		demandLabels,
		type Case,
		type Demands,
		type ClientDraft,
		type RequestDraft
	} from './fixtures.js';

	interface Properties {
		blurb: string;
		activeCase: Case;
		demands: Demands;
		onCase: (value: Case) => void;
		onDemands: (value: Demands) => void;
		written: { client?: ClientDraft; request?: RequestDraft; reused?: string; note: string } | undefined;
		children: Snippet;
	}

	let { blurb, activeCase, demands, onCase, onDemands, written, children }: Properties = $props();

	const demandKeys = Object.keys(demandLabels) as Demands[];
</script>

<div class="dials">
	<p class="blurb">{blurb}</p>
	<div class="row">
		<span class="key">Case</span>
		{#each cases as item (item.key)}
			<button
				type="button"
				class:on={item.key === activeCase.key}
				onclick={() => onCase(item)}>{item.name}</button
			>
		{/each}
	</div>
	<div class="row">
		<span class="key">Form demands</span>
		{#each demandKeys as item (item)}
			<button type="button" class:on={item === demands} onclick={() => onDemands(item)}
				>{demandLabels[item]}</button
			>
		{/each}
	</div>
	<p class="case">{activeCase.blurb}</p>
</div>

<div class="stage">
	{@render children()}
</div>

{#if written}
	<div class="state">
		<strong>What was written</strong>
		<p>{written.note}</p>
		{#if written.reused}
			<p>Reused existing Client <code>{written.reused}</code> — no new <code>clients</code> row.</p>
		{/if}
		{#if written.client}
			<pre>clients: {JSON.stringify(written.client, undefined, 2)}</pre>
		{/if}
		{#if written.request}
			<pre>engagement_requests: {JSON.stringify(written.request, undefined, 2)}</pre>
		{/if}
		<p>
			engagements: <em>none</em> — an Engagement exists only after an Owner or Admin approves
			(#393). Credits spent: <strong>0</strong>.
		</p>
	</div>
{/if}

<style>
	.dials {
		border: 1px dashed #999;
		padding: 0.75rem;
		margin-block-end: 1.5rem;
		font-size: 0.8125rem;
	}

	.blurb {
		margin: 0 0 0.5rem;
		font-weight: 600;
	}

	.row {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.375rem;
		margin-block-end: 0.375rem;
	}

	.key {
		font: 600 0.6875rem/1 ui-monospace, monospace;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		min-width: 8rem;
	}

	.row button {
		border: 1px solid #999;
		background: transparent;
		border-radius: 999px;
		padding: 0.1875rem 0.625rem;
		font-size: 0.75rem;
		cursor: pointer;
	}

	.row button.on {
		background: #111;
		color: #fff;
		border-color: #111;
	}

	.case {
		margin: 0.5rem 0 0;
		font-style: italic;
		max-width: 68ch;
	}

	.stage {
		max-width: 46rem;
	}

	.state {
		margin-block: 2rem 5rem;
		padding: 1rem;
		background: #111;
		color: #eee;
		font-size: 0.8125rem;
		max-width: 46rem;
	}

	.state pre {
		white-space: pre-wrap;
		font-size: 0.75rem;
	}

	.state code {
		background: #333;
		padding: 0 0.25rem;
	}
</style>
