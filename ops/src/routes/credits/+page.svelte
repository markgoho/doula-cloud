<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import MonthlyCredits from '#lib/components/MonthlyCredits.svelte';

	const month = $derived(page.url.searchParams.get('month') ?? undefined);

	async function signedOut() {
		await goto(resolve('/sign-in'));
	}
</script>

<svelte:head><title>Monthly Credits – DoulaCloud operations</title></svelte:head>

<h1>Monthly Credits</h1>
<p>Credits bought and spent by each Practice in a month that has ended. Months begin and end in New York time.</p>
{#key month}
	<MonthlyCredits {month} onsignedout={signedOut} />
{/key}
