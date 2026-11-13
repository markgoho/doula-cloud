<script lang="ts">
	/*
	 * A Visit's Doula on a Staff list: her name, and where a booked Visit
	 * still stands under a Doula who was taken off its Engagement (#2596) the
	 * warning badge, the one row that needs someone to act. The Schedule and
	 * the Engagement page's Visits both draw their Doula cell with it, so the
	 * two say it the same way. A screen reader hears one phrase, "Priya
	 * Raman, No longer on this work".
	 *
	 * `isTracked` is the table's form, on one line: the badge's pill is
	 * taller than a line of text by its own block padding, and a margin takes
	 * that padding back, so a row with a badge is as tall as a row without
	 * one (`MoneyAmount` does the same). A record card's form wraps, and its
	 * box holds the pill.
	 */
	import Badge from '#lib/components/atoms/Badge.svelte';
	import { NO_LONGER_ON_THIS_ENGAGEMENT } from '#lib/engagementDetail.js';

	interface Properties {
		name: string;
		isOffEngagement: boolean;
		isTracked?: boolean;
	}

	let { name, isOffEngagement, isTracked = false }: Properties = $props();
</script>

<span class="doula" class:tracked={isTracked}
	><span class="name">{name}</span
	>{#if isOffEngagement}<span class="visually-hidden">&#44;&#32;</span><Badge label={NO_LONGER_ON_THIS_ENGAGEMENT} variant="warning" />{/if}</span
>

<style>
	@layer components {
		.doula {
			display: inline-flex;
			flex-wrap: wrap;
			align-items: center;
			gap: var(--space-1) var(--space-2);
		}

		.tracked {
			flex-wrap: nowrap;
			white-space: nowrap;
			margin-block: calc(var(--space-1) / -2);
		}

		.name {
			overflow-wrap: anywhere;
		}

		.tracked .name {
			overflow-wrap: normal;
		}
	}
</style>
