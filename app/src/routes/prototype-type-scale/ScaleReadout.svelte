<script lang="ts">
	/*
	 * PROTOTYPE (#1926) -- the current candidate's eight steps in px: the
	 * floor, what each column renders it at, the ceiling, and the ratio to
	 * the step below, so a scale can be read as numbers beside the screens.
	 */
	import { formatPx, sizeAt, STEPS, type Scale } from './scales.js';

	interface Properties {
		scale: Scale;
		narrowWidth: number;
		wideWidth: number;
	}

	let { scale, narrowWidth, wideWidth }: Properties = $props();

	const rows = $derived(
		STEPS.map((step, index) => {
			const floor = scale.floors[index]!;
			const below = index > 0 ? scale.floors[index - 1]! : undefined;
			return {
				...step,
				floor,
				ratio: below === undefined ? '' : (floor / below).toFixed(3),
				narrow: sizeAt(floor, narrowWidth),
				wide: sizeAt(floor, wideWidth),
				ceiling: floor * 1.2
			};
		}).toReversed()
	);
</script>

<div class="readout">
	<table>
		<caption>{scale.name}. {scale.note}</caption>
		<thead>
			<tr>
				<th scope="col">Step</th>
				<th scope="col">Used for</th>
				<th scope="col" class="number">Floor, 320px</th>
				<th scope="col" class="number">Narrow column</th>
				<th scope="col" class="number">Wide column</th>
				<th scope="col" class="number">Ceiling, 1920px</th>
				<th scope="col" class="number">Ratio to the step below</th>
			</tr>
		</thead>
		<tbody>
			{#each rows as row (row.token)}
				<tr class:body={row.token === 'body'}>
					<th scope="row">{row.token}</th>
					<td>{row.use}</td>
					<td class="number">{formatPx(row.floor)}px</td>
					<td class="number">{formatPx(Number(row.narrow.toFixed(1)))}px</td>
					<td class="number">{formatPx(Number(row.wide.toFixed(1)))}px</td>
					<td class="number">{formatPx(Number(row.ceiling.toFixed(1)))}px</td>
					<td class="number">{row.ratio}</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>

<style>
	.readout {
		overflow-x: auto;
	}

	table {
		border-collapse: collapse;
		font-size: var(--text-label-size);
		font-variant-numeric: tabular-nums;
	}

	caption {
		padding-block-end: var(--space-2);
		font-size: var(--text-body-sm-size);
		font-weight: var(--font-weight-semibold);
		text-align: start;
	}

	th,
	td {
		padding: var(--space-1) var(--space-3);
		text-align: start;
		border-block-end: var(--border-thin) solid var(--color-outline-variant);
	}

	.number {
		text-align: end;
	}

	.body {
		background-color: var(--color-surface-container);
	}
</style>
