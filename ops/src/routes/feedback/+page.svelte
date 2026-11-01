<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import FeedbackList from '#lib/components/FeedbackList.svelte';

	async function signedOut() {
		await goto(resolve('/sign-in'));
	}
</script>

<svelte:head><title>Feedback – DoulaCloud operations</title></svelte:head>

<h1>Feedback</h1>
<p>Every piece of Feedback sent to DoulaCloud, newest first. Each time you open a piece, the read is recorded in the Operator log.</p>
<h2>Issue not opened</h2>
<FeedbackList list="unopened" onsignedout={signedOut} />
<h2>All Feedback</h2>
<FeedbackList list="all" onsignedout={signedOut} />
