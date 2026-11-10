<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { CheckRead } from '#lib/checkRead.svelte.js';
	import { readFeedbackPiece, sentAt, type FeedbackPiece as Piece } from '#lib/feedback.js';
	import ErrorSummary from '$ui/molecules/ErrorSummary.svelte';
	import FeedbackPiece from '#lib/components/FeedbackPiece.svelte';

	const feedbackId = $derived(page.params.feedbackId ?? '');

	const read = new CheckRead<Piece>(() => goto(resolve('/sign-in')));

	onMount(() => read.load(() => readFeedbackPiece(feedbackId)));
</script>

<svelte:head><title>Feedback – DoulaCloud operations</title></svelte:head>

<p><a href={resolve('/feedback')}>Back to Feedback</a></p>
{#if read.state.status === 'loaded'}
	<h1>Feedback sent {sentAt(read.state.value.sentAt)}</h1>
	<FeedbackPiece piece={read.state.value} />
{:else if read.state.status === 'failed'}
	<h1>Feedback</h1>
	<ErrorSummary errors={read.errors} />
{/if}
