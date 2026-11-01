<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import PracticeList from '#lib/components/PracticeList.svelte';

	async function signedOut() {
		await goto(resolve('/sign-in'));
	}
</script>

<svelte:head><title>Deleted Practices – DoulaCloud operations</title></svelte:head>

<h1>Deleted Practices</h1>
<p>Each Practice whose Deletion finalized, newest signup first. Its Credit ledger and Stripe Connect records are kept.</p>
<p><a href={resolve('/practices')}>Practices</a></p>
<PracticeList list="deleted" onsignedout={signedOut} />
