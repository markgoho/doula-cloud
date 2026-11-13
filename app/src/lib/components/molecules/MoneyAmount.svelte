<script lang="ts">
	/*
	 * One Invoice's money on a Staff list (DESIGN.md, "Money is said once"):
	 * its amount, then its state, a badge where the state asks something of
	 * the reader (amber owed, red overdue or written off) and a plain word
	 * otherwise. The Clients list and the Invoices list draw their Money
	 * cells with it, so the two cannot drift apart again (#2746); the state
	 * comes from `invoiceMoneyState`.
	 *
	 * `isTracked` is the table's form: the amount and the state in two
	 * tracks, the state's as wide as `--money-state-track`, so every amount
	 * in a right-aligned column ends on one edge whatever its state says.
	 * Without it, a record card's form, the state follows the amount and
	 * wraps under it where the line runs out. A screen reader hears one
	 * phrase, "$950.00, 12 days overdue".
	 */
	import Badge from '#lib/components/atoms/Badge.svelte';

	interface Properties {
		amount: string;
		state?: { word: string; variant?: 'warning' | 'error' };
		isTracked?: boolean;
	}

	let { amount, state, isTracked = false }: Properties = $props();
</script>

<span class="money" class:tracked={isTracked}
	><span class="amount">{amount}</span><span class="state"
		>{#if state?.word}<span class="visually-hidden">&#44;&#32;</span
			>{#if state.variant}<Badge label={state.word} variant={state.variant} />{:else}{state.word}{/if}{/if}</span
	></span
>

<style>
	@layer components {
		.money {
			display: inline-flex;
			flex-wrap: wrap;
			align-items: center;
			gap: var(--space-1);
		}

		/* The state's badge is a pill taller than a line of text by its own
		   block padding; this margin takes that padding back, so a row with a
		   badge is as tall as a row without one. */
		.tracked {
			display: inline-grid;
			grid-template-columns: auto var(--money-state-track);
			margin-block: calc(var(--space-1) / -2);
			white-space: nowrap;
		}

		.amount {
			white-space: nowrap;
		}

		/* A state with no badge is a plain word, quieter than its amount. */
		.state {
			color: var(--color-on-surface-variant);
			text-align: start;
		}
	}
</style>
