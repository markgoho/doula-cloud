<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import type { FormError } from '../authRefusal.ts';
	import { monthName, readMonthlyCredits, type MonthlyCredits } from '../monthlyCredits.ts';
	import ErrorSummary from './ErrorSummary.svelte';

	let {
		month,
		onsignedout,
		fetcher = fetch
	}: { month: string | undefined; onsignedout: () => void; fetcher?: typeof fetch } = $props();

	let read = $state<MonthlyCredits | undefined>();
	let errors = $state<FormError[]>([]);

	onMount(async () => {
		const outcome = await readMonthlyCredits(month, fetcher);
		if (outcome.kind === 'signed-out') return onsignedout();
		if (outcome.kind === 'refused') {
			errors = outcome.errors;
			return;
		}
		/* v8 ignore next -- a read asks no fresh code, so 'step-up' never arrives */
		if (outcome.kind === 'ok') read = outcome.value;
	});
</script>

<ErrorSummary {errors} />

{#if errors.length > 0}
	<p><a href={resolve('/credits')}>Show the last month that has ended</a></p>
{/if}

{#if read}
	<!-- v8 ignore start: Svelte's compiled update branches for each value; a read is set once and never changes in place -->
	<form method="get" action={resolve('/credits')} class="picker">
		<label for="month">Month</label>
		<select id="month" name="month" value={read.month}>
			{#each read.months as option (option)}
				<option value={option}>{monthName(option)}</option>
			{/each}
		</select>
		<button class="ops-button" type="submit">Show month</button>
	</form>

	{#if read.practices.length === 0}
		<p>No Practice had signed up by the end of {monthName(read.month)}.</p>
	{:else}
		<div class="scroll">
			<table>
				<caption>Credits in {monthName(read.month)}</caption>
				<thead>
					<tr>
						<th scope="col">Practice</th>
						<th scope="col" class="number">Credits bought</th>
						<th scope="col" class="number">Credits spent</th>
					</tr>
				</thead>
				<tbody>
					{#each read.practices as practice (practice.id)}
						<tr>
							<th scope="row">{practice.name}</th>
							<td class="number">{practice.creditsBought}</td>
							<td class="number">{practice.creditsSpent}</td>
						</tr>
					{/each}
				</tbody>
				<tfoot>
					<tr>
						<th scope="row">Total</th>
						<td class="number">{read.totals.creditsBought}</td>
						<td class="number">{read.totals.creditsSpent}</td>
					</tr>
				</tfoot>
			</table>
		</div>
		<p>Practices who bought again: {read.totals.practicesBoughtAgain}</p>
	{/if}
	<!-- v8 ignore stop -->
{/if}

<style>
	.picker {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-2) var(--space-4);
	}

	label {
		flex-basis: 100%;
		font-weight: bold;
	}

	select {
		min-block-size: var(--control-height);
		padding-inline: var(--space-2);
		color: var(--color-on-surface);
		background-color: var(--color-surface);
		border: var(--border-thin) solid var(--color-outline);
		border-radius: var(--radius);
	}

	select:focus-visible {
		outline: var(--focus-ring-width) solid var(--color-primary);
		outline-offset: var(--focus-ring-offset);
	}

	.scroll {
		max-inline-size: 100%;
		overflow-x: auto;
	}

	table {
		inline-size: 100%;
		border-collapse: collapse;
	}

	caption {
		padding-block-end: var(--space-2);
		font-weight: bold;
		text-align: start;
	}

	th,
	td {
		padding: var(--space-2);
		text-align: start;
		border-block-end: var(--border-thin) solid var(--color-outline-variant);
	}

	tbody th {
		font-weight: normal;
		overflow-wrap: anywhere;
	}

	.number {
		text-align: end;
		font-variant-numeric: tabular-nums;
	}

	tfoot th,
	tfoot td {
		font-weight: bold;
		border-block-end: none;
	}

	a {
		color: var(--color-primary);
	}

	a:focus-visible {
		outline: var(--focus-ring-width) solid var(--color-primary);
		outline-offset: var(--focus-ring-offset);
	}
</style>
