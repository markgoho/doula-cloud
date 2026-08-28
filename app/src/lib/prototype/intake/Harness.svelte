<script lang="ts">
	// PROTOTYPE (#372) -- throwaway. Decision 1 of 4: the shape. One dial only.
	// Everything else the ticket asks is pinned here and decided later, in its
	// own turn, so the shapes are judged rather than the permutations.
	import type { Snippet } from 'svelte';
	import type { ClientDraft, RequestDraft } from './fixtures.js';

	interface Properties {
		blurb: string;
		written:
			| { client?: ClientDraft; request?: RequestDraft; reused?: string; note: string }
			| undefined;
		children: Snippet;
	}

	let { blurb, written, children }: Properties = $props();
</script>

<div class="dials">
	<p class="blurb">{blurb}</p>
	<p class="pinned">
		Still to decide — <strong>where the save falls</strong> in the three pages, the
		<strong>voice of the two request actions</strong>, and the
		<strong>postpartum-only and returning-Client walks</strong>.
	</p>
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
		max-width: 62ch;
	}

	.blurb {
		margin: 0 0 0.5rem;
		font-weight: 600;
	}

	.pinned {
		margin: 0;
		color: #555;
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
