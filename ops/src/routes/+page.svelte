<script lang="ts">
	import { page } from '$app/state';
	import type { Operator } from '#lib/session.js';

	const operator = $derived(page.data.operator as Operator);
</script>

<svelte:head><title>DoulaCloud operations</title></svelte:head>

{#if operator}
	<h1>DoulaCloud operations</h1>
	<p>You are signed in as {operator.name} ({operator.email}).</p>
{/if}
