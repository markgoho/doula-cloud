<script lang="ts">
	/*
	 * When a coverage gap runs: "Sat Nov 14, 6:00pm to Sun Nov 15, 9:00am",
	 * each end in a `<time datetime>` that keeps the exact instant. The On
	 * call roster and the Engagement's gap list both draw it, so the two say
	 * it the same way; the words are `formatGapTimes`'s, ADR-0022's Visit
	 * form. Each end holds on one line, so a line wraps between the ends and
	 * never inside one.
	 */
	import { gapTimes } from '#lib/coverageGap.js';

	interface Properties {
		startsAt: string;
		endsAt: string;
		/**
		The Practice's zone (ADR-0036): both ends are read in it, never the reader's.
		*/
		timeZone: string;
		/**
		The instant "this year" is read from: a day carries its year only when it is not this one.
		*/
		now: Date;
	}

	let { startsAt, endsAt, timeZone, now }: Properties = $props();

	const times = $derived(gapTimes(startsAt, endsAt, timeZone, now));
</script>

<time class="end" datetime={startsAt}>{times.start}</time> to <time class="end" datetime={endsAt}>{times.end}</time>

<style>
	@layer components {
		.end {
			white-space: nowrap;
			font-variant-numeric: tabular-nums;
		}
	}
</style>
