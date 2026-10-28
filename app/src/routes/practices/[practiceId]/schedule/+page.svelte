<script lang="ts">
	import { Streamed } from '#lib/streamedRead.svelte.js';
	import ListPage from '#lib/components/templates/ListPage.svelte';
	import FailedRead from '../FailedRead.svelte';
	import ScheduleScreen from './ScheduleScreen.svelte';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();
	const schedule = new Streamed(() => data.read);
</script>

{#snippet nothing()}{/snippet}

{#if schedule.failure}
	<FailedRead kind={schedule.failure} />
{:else if schedule.value}
	<ScheduleScreen data={schedule.value} />
{:else}
	<ListPage title="Schedule" loading="Loading your schedule" content={nothing} />
{/if}
