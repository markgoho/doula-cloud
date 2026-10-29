<script lang="ts">
	import { Streamed } from '#lib/streamedRead.svelte.js';
	import FailedRead from '../FailedRead.svelte';
	import ScheduleScreen from './ScheduleScreen.svelte';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();
	const schedule = new Streamed(() => data.read);
</script>

{#if schedule.failure}
	<FailedRead kind={schedule.failure} />
{:else}
	<ScheduleScreen data={schedule.value} />
{/if}
