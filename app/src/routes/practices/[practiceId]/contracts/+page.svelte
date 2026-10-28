<script lang="ts">
	import { Streamed } from '#lib/streamedRead.svelte.js';
	import ListPage from '#lib/components/templates/ListPage.svelte';
	import FailedRead from '../FailedRead.svelte';
	import ContractsScreen from './ContractsScreen.svelte';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();
	const contracts = new Streamed(() => data.read);
</script>

{#snippet nothing()}{/snippet}

{#if contracts.failure}
	<FailedRead kind={contracts.failure} />
{:else if contracts.value}
	<ContractsScreen data={contracts.value} />
{:else}
	<ListPage title="Contracts" loading="Loading your Contracts" content={nothing} />
{/if}
