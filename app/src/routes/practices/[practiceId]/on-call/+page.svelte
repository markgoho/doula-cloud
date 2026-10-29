<script lang="ts">
	import { Streamed } from '#lib/streamedRead.svelte.js';
	import FailedRead from '../FailedRead.svelte';
	import OnCallScreen from './OnCallScreen.svelte';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();
	const roster = new Streamed(() => data.read);
</script>

{#if roster.failure}
	<FailedRead kind={roster.failure} />
{:else}
	<OnCallScreen data={roster.value} />
{/if}
