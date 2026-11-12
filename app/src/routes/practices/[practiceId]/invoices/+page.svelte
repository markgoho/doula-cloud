<script lang="ts">
	import { invalidate } from '$app/navigation';
	import { Streamed } from '#lib/streamedRead.svelte.js';
	import FailedRead from '../FailedRead.svelte';
	import InvoicesScreen from './InvoicesScreen.svelte';
	import { _INVOICES_READ } from './+page.js';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();
	const invoices = new Streamed(() => data.read);

	// #2746 (T8): a read that may work a second time is said inside the
	// screen, beside "Try again", so the view and the typed search stay. A
	// refusal, a missing Practice or a locked one would only fail again.
	const isRetryable = $derived(invoices.failure === 'problem' || invoices.failure === 'unavailable');

	async function retry() {
		await invalidate(_INVOICES_READ);
		await data.read;
	}
</script>

{#if invoices.failure && !isRetryable}
	<FailedRead kind={invoices.failure} />
{:else}
	<InvoicesScreen data={invoices.value} hasReadFailed={isRetryable} onRetry={retry} />
{/if}
