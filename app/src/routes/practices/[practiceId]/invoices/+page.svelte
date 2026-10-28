<script lang="ts">
	import { Streamed } from '#lib/streamedRead.svelte.js';
	import ListPage from '#lib/components/templates/ListPage.svelte';
	import FailedRead from '../FailedRead.svelte';
	import InvoicesScreen from './InvoicesScreen.svelte';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();
	const invoices = new Streamed(() => data.read);
</script>

{#snippet nothing()}{/snippet}

{#if invoices.failure}
	<FailedRead kind={invoices.failure} />
{:else if invoices.value}
	<InvoicesScreen data={invoices.value} />
{:else}
	<ListPage title="Invoices" loading="Loading your invoices" content={nothing} />
{/if}
