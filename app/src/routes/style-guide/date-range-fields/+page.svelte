<script module lang="ts">
	import type { DemoVariant } from '../drag-surface/dragSurface.js';

	interface Properties {
		hasProblem?: boolean;
	}

	export const variants: readonly DemoVariant<Properties>[] = [
		{ name: 'Date range fields, with a hint and a refused To day', props: { hasProblem: true } }
	];
</script>

<script lang="ts">
	import DateRangeFields from '#lib/components/molecules/DateRangeFields.svelte';
	import FilterBar from '#lib/components/molecules/FilterBar.svelte';

	let { hasProblem = false }: Properties = $props();

	let from = $state('2026-10-26');
	let to = $state('2026-11-25');

	function stay(event: SubmitEvent) {
		event.preventDefault();
	}
</script>

<stack-l space="var(--space-6)">
	<h1>Date range fields</h1>

	<p>
		The From and To dates a list narrows by, as two labeled date boxes. They are drawn as siblings
		so a filter bar lays them out in its own row. A screen that refuses a range gives them
		<code>toError</code> (and <code>fromError</code>), and one that says whether the last day is part
		of the range gives them <code>hint</code>, drawn once above both boxes.
	</p>

	<FilterBar
		action="/style-guide/date-range-fields"
		onSubmit={stay}
		legend="Choose the days"
		status="Showing the days you chose."
	>
		<DateRangeFields
			bind:from
			bind:to
			hint={hasProblem ? 'Both days are included.' : undefined}
			toError={hasProblem ? 'Choose a To day that is the same as the From day or later.' : undefined}
		/>
	</FilterBar>
</stack-l>
