<script lang="ts">
	/*
	 * The Engagement's header facts (#2755), one line under the Client's name:
	 * the care phase, the due day with how far along, and who the Doulas are.
	 * A properties bar, not a list of controls: it holds no link (rubric 6a),
	 * and each item keeps its label and does not break inside itself, so the
	 * line wraps only between items.
	 */
	import { formatCalendarDay } from '#lib/dates.js';
	import Badge from '#lib/components/atoms/Badge.svelte';

	interface Properties {
		carePhase: string;
		/**
		 * A calendar day, absent with no due date or once an outcome is recorded.
		 */
		dueDay?: string;
		weeks?: number;
		/**
		 * "Doula" or "Doulas"; absent for a completed Engagement nobody is on.
		 */
		doulaLabel?: string;
		doulas?: string;
		/**
		 * Nobody holds the birth tonight: a warning, drawn after the line.
		 */
		isNobodyOnCallTonight?: boolean;
	}

	let { carePhase, dueDay, weeks, doulaLabel, doulas, isNobodyOnCallTonight = false }: Properties = $props();
</script>

<div class="line">
<dl>
	<div>
		<dt>Care phase</dt>
		<dd>{carePhase}</dd>
	</div>
	{#if dueDay}
		<div>
			<dt>Due</dt>
			<dd><time datetime={dueDay}>{formatCalendarDay(dueDay)}</time>{weeks === undefined ? '' : `, ${weeks} weeks`}</dd>
		</div>
	{/if}
	{#if doulaLabel && doulas}
		<div>
			<dt>{doulaLabel}</dt>
			<dd>{doulas}</dd>
		</div>
	{/if}
</dl>
{#if isNobodyOnCallTonight}<Badge label="Nobody on call tonight" variant="warning" />{/if}
</div>

<style>
	.line {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		column-gap: var(--space-4);
		row-gap: var(--space-1);
	}

	dl {
		display: flex;
		flex-wrap: wrap;
		column-gap: var(--space-4);
		row-gap: var(--space-1);
		margin: 0;
		font-size: var(--text-body-sm-size);
	}

	dl > div {
		display: flex;
		gap: var(--space-1);
		min-inline-size: 0;
	}

	dt {
		color: var(--color-on-surface-muted);
		white-space: nowrap;
	}

	dd {
		margin: 0;
		color: var(--color-on-surface);
	}
</style>
