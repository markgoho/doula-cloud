<script lang="ts">
	import { Streamed } from '#lib/streamedRead.svelte.js';
	import ListPage from '#lib/components/templates/ListPage.svelte';
	import FailedRead from '../FailedRead.svelte';
	import OnCallScreen from './OnCallScreen.svelte';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();
	const roster = new Streamed(() => data.read);
</script>

{#snippet nothing()}{/snippet}

{#if roster.failure}
	<FailedRead kind={roster.failure} />
{:else if roster.value}
	<OnCallScreen data={roster.value} />
{:else}
	<ListPage title="On call" loading="Loading who is on call" content={nothing} />
{/if}
