<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import System from '#lib/components/System.svelte';

	async function signedOut() {
		await goto(resolve('/sign-in'));
	}
</script>

<svelte:head><title>System – DoulaCloud operations</title></svelte:head>

<h1>System</h1>
<p>What the service is doing. Times are New York time.</p>
<System onsignedout={signedOut} />
