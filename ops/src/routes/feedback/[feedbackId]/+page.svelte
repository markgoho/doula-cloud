<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import type { FormError } from '#lib/authRefusal.js';
	import { readFeedbackPiece, sentAt, type FeedbackPiece as Piece } from '#lib/feedback.js';
	import ErrorSummary from '$ui/molecules/ErrorSummary.svelte';
	import FeedbackPiece from '#lib/components/FeedbackPiece.svelte';

	const feedbackId = $derived(page.params.feedbackId ?? '');

	let piece = $state<Piece | undefined>();
	let errors = $state<FormError[]>([]);

	onMount(async () => {
		const outcome = await readFeedbackPiece(feedbackId);
		if (outcome.kind === 'signed-out') return goto(resolve('/sign-in'));
		if (outcome.kind === 'refused') errors = outcome.errors;
		else if (outcome.kind === 'ok') piece = outcome.value;
	});
</script>

<svelte:head><title>Feedback – DoulaCloud operations</title></svelte:head>

<p><a href={resolve('/feedback')}>Back to Feedback</a></p>
{#if piece}
	<h1>Feedback sent {sentAt(piece.sentAt)}</h1>
	<FeedbackPiece {piece} />
{:else if errors.length > 0}
	<h1>Feedback</h1>
	<ErrorSummary {errors} />
{/if}
