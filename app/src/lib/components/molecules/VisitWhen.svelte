<script lang="ts">
	/*
	 * A Visit's day and time on a Staff list: "Fri Nov 13, 2:00pm", the form
	 * ADR-0022's 2026-10-24 amendment writes, with the exact instant in a
	 * `<time datetime>`. The Schedule and the Engagement page's Visits both
	 * draw their When cell with it, so the two say it the same way, comma
	 * included. A Visit with no instant yet says "Not yet scheduled" in plain
	 * text, as `formatScheduledVisit` does.
	 *
	 * `isTracked` is the table's form: the day holds a track so every weekday
	 * starts at one x (a row whose day carries a year is the one that runs
	 * longer), and the clock ends on one edge in tabular figures. The tracks
	 * are the longest a day and a clock of this year can be, a constant, so
	 * the cell is as wide with one Doula's Visits as with all of them. A
	 * card's form runs on, as one phrase.
	 */
	import { formatScheduledVisit, scheduledVisitParts } from '#lib/dates.js';

	interface Properties {
		at?: string;
		isTracked?: boolean;
	}

	let { at, isTracked = false }: Properties = $props();

	const parts = $derived(at ? scheduledVisitParts(at) : undefined);
</script>

{#if parts}
	<time class="when" class:tracked={isTracked} datetime={at}><span class="day">{parts.day}</span> <span class="clock">{parts.clock}</span></time>
{:else}
	{formatScheduledVisit(at)}
{/if}

<style>
	@layer components {
		.when {
			white-space: nowrap;
			font-variant-numeric: tabular-nums;
		}

		.tracked .day {
			display: inline-block;
			min-inline-size: var(--visit-when-day-track, 10.5ch);
		}

		.tracked .clock {
			display: inline-block;
			min-inline-size: var(--visit-when-clock-track, 7ch);
			text-align: end;
		}
	}
</style>
