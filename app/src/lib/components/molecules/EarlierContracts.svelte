<script lang="ts">
	/*
	 * The Contracts an Engagement's current one replaced, kept in its Contract
	 * section as history (#2958). Each is a voided Contract with its own
	 * `#contract-<id>` anchor, which is where the Contracts list's row for it
	 * opens: the day it was voided, who voided it, and the reason on the
	 * request an Owner or an Admin granted, where there was one.
	 */
	import Heading from '#lib/components/atoms/Heading.svelte';
	import SummaryList, { type SummaryListRow } from '#lib/components/molecules/SummaryList.svelte';
	import type { EarlierContract } from '#lib/contract.js';
	import { formatInstant } from '#lib/dates.js';

	interface Properties {
		earlier: readonly EarlierContract[];
		timeZone: string;
	}

	let { earlier, timeZone }: Properties = $props();

	function rowsOf(contract: EarlierContract): SummaryListRow[] {
		return [
			{
				label: 'Voided',
				value: `${formatInstant(contract.voidedAt, timeZone)}${contract.voidedBy ? ` by ${contract.voidedBy}` : ''}`
			},
			...(contract.reason ? [{ label: 'Reason', value: `${contract.requestedBy} asked: ${contract.reason}` }] : [])
		];
	}
</script>

{#if earlier.length > 0}
	<Heading level={3} text={earlier.length === 1 ? 'Earlier Contract' : 'Earlier Contracts'} />
	{#each earlier as contract (contract.id)}
		<!-- v8 ignore start: Svelte's compiled id-diffing branch, reachable only by re-rendering the same entry under another id, which keyed each blocks never do -->
		<div id="contract-{contract.id}">
			<SummaryList rows={rowsOf(contract)} />
		</div>
		<!-- v8 ignore stop -->
	{/each}
{/if}
