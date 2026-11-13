<script lang="ts">
	import MoneyAmount from '#lib/components/molecules/MoneyAmount.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';

	const states = [
		{ amount: '$1,500.00', state: { word: 'owed', variant: 'warning' as const } },
		{ amount: '$950.00', state: { word: '120 days overdue', variant: 'error' as const } },
		{ amount: '$2,400.00', state: { word: 'written off', variant: 'error' as const } },
		{ amount: '$475.00', state: { word: 'paid' } },
		{ amount: '$1,500.00', state: { word: '$300.00 refunded' } },
		{ amount: '$9,999.00', state: { word: 'payment on its way' } },
		{ amount: '$500.00', state: { word: 'void' } }
	];
</script>

<stack-l space="var(--space-6)">
	<Heading level={2} variant="section" text="Money amount" />
	<Text
		text="One Invoice's money on a Staff list: the amount, then its state. Amber owed and red overdue or written off are badges; every other state is a plain word. The Clients list and the Invoices list both draw their Money cell with it."
		tone="variant"
	/>

	<stack-l space="var(--space-4)">
		<h2>In a table: two tracks, so the amounts end on one edge</h2>
		<div class="column">
			{#each states as row, index (index)}
				<div class="cell"><MoneyAmount amount={row.amount} state={row.state} isTracked /></div>
			{/each}
		</div>
	</stack-l>

	<stack-l space="var(--space-4)">
		<h2>On a card: the state follows the amount</h2>
		<stack-l space="var(--space-2)">
			{#each states as row, index (index)}
				<div><MoneyAmount amount={row.amount} state={row.state} /></div>
			{/each}
		</stack-l>
	</stack-l>
</stack-l>

<style>
	@layer components {
		/* A numeric column: the amounts end on its right edge. */
		.column {
			display: flex;
			flex-direction: column;
			align-items: end;
			gap: var(--space-2);
		}

		/* A table cell's room around one line, which the badge's pill takes back into. */
		.cell {
			padding-block: var(--space-1);
		}
	}
</style>
