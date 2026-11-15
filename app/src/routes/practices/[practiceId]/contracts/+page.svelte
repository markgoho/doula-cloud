<script lang="ts">
	import { invalidate } from '$app/navigation';
	import { CONTRACTS_READ, type VoidRequestsPage } from '#lib/contract.js';
	import type { LabeledValue } from '#lib/roles.js';
	import { Streamed } from '#lib/streamedRead.svelte.js';
	import FailedRead from '../FailedRead.svelte';
	import ContractsScreen from './ContractsScreen.svelte';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();
	const contracts = new Streamed(() => data.read);
	const queue = new Streamed<VoidRequestsPage | undefined>(
		() => data.voidRequests ?? Promise.resolve({ isLoaded: true, value: undefined })
	);

	const isRetryable = $derived(contracts.failure === 'problem' || contracts.failure === 'unavailable');

	let doulas = $state.raw<LabeledValue[]>([]);
	let isRosterLanded = $state(false);
	$effect(() => {
		const pending = data.doulas;
		let isCurrent = true;
		void pending.then((options) => {
			if (!isCurrent) return;
			doulas = options;
			isRosterLanded = true;
		});
		return () => {
			isCurrent = false;
		};
	});

	async function retry() {
		await invalidate(CONTRACTS_READ);
		await data.read;
	}
</script>

{#if contracts.failure && !isRetryable}
	<FailedRead kind={contracts.failure} />
{:else}
	<ContractsScreen
		data={contracts.value}
		isReading={contracts.isPending}
		hasReadFailed={isRetryable}
		onRetry={retry}
		isQueueExpected={data.voidRequests !== undefined}
		queue={queue.value}
		hasQueueFailed={queue.failure !== undefined}
		{doulas}
		{isRosterLanded}
	/>
{/if}
