<script lang="ts">
	import { Streamed } from '#lib/streamedRead.svelte.js';
	import FailedRead from '../FailedRead.svelte';
	import InvoicesScreen from './InvoicesScreen.svelte';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();
	const invoices = new Streamed(() => data.read);
</script>

{#if invoices.failure}
	<FailedRead kind={invoices.failure} />
{:else}
	<InvoicesScreen data={invoices.value} />
{/if}
