<script lang="ts">
	import { issueLabel, pieceFacts, type FeedbackPiece } from '../feedback.ts';
	import SummaryList from './SummaryList.svelte';

	let { piece }: { piece: FeedbackPiece } = $props();

	const paragraphs = $derived(piece.text.split('\n').filter((line) => line.trim() !== ''));
</script>

<stack-l>
	<h2>What was written</h2>
	<div class="words">
		{#each paragraphs as paragraph, index (index)}
			<p>{paragraph}</p>
		{:else}
			<p>Nothing was written. This piece is its kind and its context only.</p>
		{/each}
	</div>
</stack-l>

<stack-l>
	<h2>About this piece</h2>
	<SummaryList rows={pieceFacts(piece)} />
</stack-l>

<stack-l>
	<h2>GitHub issue</h2>
	{#if piece.issue.url}
		<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- the issue is on GitHub, outside ops -->
		<p><a href={piece.issue.url}>{issueLabel(piece.issue)} in the feedback repository</a></p>
	{:else}
		<p>{issueLabel(piece.issue)}</p>
	{/if}
	{#if piece.issue.lastError}
		<p>{`The last attempt said: ${piece.issue.lastError}`}</p>
	{/if}
</stack-l>

<style>
	.words {
		max-inline-size: var(--measure);
		overflow-wrap: anywhere;
	}

	a {
		color: var(--color-primary);
	}

	a:focus-visible {
		outline: var(--focus-ring-width) solid var(--color-primary);
		outline-offset: var(--focus-ring-offset);
	}
</style>
