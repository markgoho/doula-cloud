<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import OperatorLog from '#lib/components/OperatorLog.svelte';

	async function signedOut() {
		await goto(resolve('/sign-in'));
	}
</script>

<svelte:head><title>Operator log – DoulaCloud operations</title></svelte:head>

<h1>Operator log</h1>
<p>Every act, sign-in, sign-out, refused code and recorded read, newest first.</p>
<OperatorLog onsignedout={signedOut} />
