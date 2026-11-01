<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import DormantPractices from '#lib/components/DormantPractices.svelte';

	async function signedOut() {
		await goto(resolve('/sign-in'));
	}
</script>

<svelte:head><title>Dormant Practices – DoulaCloud operations</title></svelte:head>

<h1>Dormant Practices</h1>
<p>
	Every Practice that holds Credits and that nobody has used in two years or more, newest signup first: the list for the
	yearly balance notice. New York takes an unspent balance after three years.
</p>
<p>The Owner to write to is on the <a href={resolve('/practices')}>Practice list</a>.</p>
<DormantPractices onsignedout={signedOut} />
