<script lang="ts">
	import { invalidate } from '$app/navigation';
	import { Streamed } from '#lib/streamedRead.svelte.js';
	import FailedRead from '../FailedRead.svelte';
	import OnCallScreen from './OnCallScreen.svelte';
	import { _ON_CALL_READ } from './+page.js';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();
	const roster = new Streamed(() => data.read);

	// A read that may work a second time is said inside the screen, beside
	// "Try again", so the bar keeps its narrowing. A refusal, a missing
	// Practice or a locked one would only fail again.
	const isRetryable = $derived(roster.failure === 'problem' || roster.failure === 'unavailable');

	async function retry() {
		await invalidate(_ON_CALL_READ);
		await data.read;
	}
</script>

{#if roster.failure && !isRetryable}
	<FailedRead kind={roster.failure} />
{:else}
	<OnCallScreen data={roster.value} isReading={roster.isPending} hasReadFailed={isRetryable} onRetry={retry} />
{/if}
