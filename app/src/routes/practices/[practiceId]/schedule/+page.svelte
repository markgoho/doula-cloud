<script lang="ts">
	import { invalidate } from '$app/navigation';
	import { Streamed } from '#lib/streamedRead.svelte.js';
	import FailedRead from '../FailedRead.svelte';
	import ScheduleScreen from './ScheduleScreen.svelte';
	import { _forgetRoster, _SCHEDULE_READ } from './+page.js';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();
	const schedule = new Streamed(() => data.read);
	$effect(() => _forgetRoster);

	// A read that may work a second time is said inside the screen, beside
	// "Try again", so the bar keeps its narrowing. A refusal, a missing
	// Practice or a locked one would only fail again.
	const isRetryable = $derived(schedule.failure === 'problem' || schedule.failure === 'unavailable');

	async function retry() {
		await invalidate(_SCHEDULE_READ);
		await data.read;
	}
</script>

{#if schedule.failure && !isRetryable}
	<FailedRead kind={schedule.failure} />
{:else}
	<ScheduleScreen data={schedule.value} isReading={schedule.isPending} hasReadFailed={isRetryable} onRetry={retry} />
{/if}
