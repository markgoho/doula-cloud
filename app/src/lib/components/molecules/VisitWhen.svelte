<script lang="ts">
	/*
	 * A Visit's day and time on a Staff list: "Fri Nov 13, 2:00pm", the form
	 * ADR-0022's 2026-10-24 amendment writes, with the exact instant in a
	 * `<time datetime>`. The Schedule and the Engagement page's Visits both
	 * draw their When cell with it, so the two say it the same way, comma
	 * included. A Visit with no instant yet says "Not yet scheduled" in plain
	 * text, as `formatScheduledVisit` does.
	 */
	import { formatScheduledVisit, scheduledVisitParts } from '#lib/dates.js';

	interface Properties {
		at?: string;
		/**
		The Practice's zone (ADR-0036): the day and the clock are read in it, never the reader's.
		*/
		timeZone: string;
	}

	let { at, timeZone }: Properties = $props();

	const parts = $derived(at ? scheduledVisitParts(at, timeZone) : undefined);
</script>

{#if parts}
	<!-- v8 ignore start: Svelte's compiled null-guard on these text nodes is unreachable -- the parts are always strings -->
	<time class="when" datetime={at}>{parts.day} {parts.clock}</time>
	<!-- v8 ignore stop -->
{:else}
	{formatScheduledVisit(at, timeZone)}
{/if}

<style>
	@layer components {
		.when {
			white-space: nowrap;
			font-variant-numeric: tabular-nums;
		}
	}
</style>
