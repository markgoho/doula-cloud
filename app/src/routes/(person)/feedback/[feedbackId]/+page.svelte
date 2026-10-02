<script lang="ts">
	/*
	 * One piece of Feedback, with every stored field (#1526). The link on
	 * the piece's private GitHub issue lands here, because the issue itself
	 * carries neither the words nor the sender (#1501 Q1).
	 *
	 * The BFF recorded this read before it answered `load`, so the page
	 * only prints. It writes no CSS of its own: the frame is
	 * `DocumentPage`'s, capped at a readable measure because the body is
	 * somebody's own words, and the facts are a `DescriptionList` -- GOV.UK's
	 * summary list (docs/design/govuk-alignment.md).
	 */
	import { resolve } from '$app/paths';
	import { formatSentAt, issueLabel, pieceFacts } from '#lib/founderFeedback.js';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import DocumentPage from '#lib/components/templates/DocumentPage.svelte';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();

	const piece = $derived(data.piece);

	// She typed line breaks; a paragraph each, so they survive.
	const paragraphs = $derived(piece.text.split('\n').filter((line) => line.trim() !== ''));
</script>

<DocumentPage
	title="Feedback sent {formatSentAt(piece.sentAt)}"
	backHref={resolve('/(person)/feedback')}
	backLabel="Back to Feedback"
>
	{#snippet content()}
		<stack-l space="var(--space-4)">
			<Heading level={2} variant="section" text="What was written" />
			{#each paragraphs as paragraph, index (index)}
				<Text text={paragraph} />
			{:else}
				<!-- Empty free text is allowed (#1523): a kind and nothing typed. -->
				<Text text="Nothing was written. This piece is its kind and its context only." tone="muted" />
			{/each}
		</stack-l>

		<stack-l space="var(--space-4)">
			<Heading level={2} variant="section" text="About this piece" />
			<DescriptionList items={pieceFacts(piece)} />
		</stack-l>

		<stack-l space="var(--space-4)">
			<Heading level={2} variant="section" text="GitHub issue" />
			{#if piece.issue.url}
				<Link href={piece.issue.url} label="{issueLabel(piece.issue)} in the feedback repository" />
			{:else}
				<Text text={issueLabel(piece.issue)} />
			{/if}
			{#if piece.issue.lastError}
				<Text text="The last attempt said: {piece.issue.lastError}" tone="muted" />
			{/if}
		</stack-l>
	{/snippet}
</DocumentPage>
