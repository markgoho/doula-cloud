<script lang="ts">
	import { Streamed } from '#lib/streamedRead.svelte.js';
	import ListPage from '#lib/components/templates/ListPage.svelte';
	import FailedRead from '../FailedRead.svelte';
	import CreditsScreen from './CreditsScreen.svelte';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();
	const balance = new Streamed(() => data.read);
</script>

{#snippet nothing()}{/snippet}

{#if balance.failure}
	<FailedRead kind={balance.failure} />
{:else if balance.value}
	<CreditsScreen data={{ ...data, ...balance.value }} />
{:else}
	<ListPage title="Credits" loading="Loading your Credits" content={nothing} />
{/if}
