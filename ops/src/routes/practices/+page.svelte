<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import PracticeList from '#lib/components/PracticeList.svelte';

	async function signedOut() {
		await goto(resolve('/sign-in'));
	}
</script>

<svelte:head><title>Practices – DoulaCloud operations</title></svelte:head>

<h1>Practices</h1>
<p>Every Practice that can still act, newest signup first. A Practice whose Deletion finalized is on the Deleted list.</p>
<p><a href={resolve('/practices/deleted')}>Deleted Practices</a></p>
<PracticeList list="live" onsignedout={signedOut} />
