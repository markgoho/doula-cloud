<script lang="ts">
	import DateRangeFields from '#lib/components/molecules/DateRangeFields.svelte';
	import FilterBar from '#lib/components/molecules/FilterBar.svelte';

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
		so a filter bar lays them out in its own row.
	</p>

	<FilterBar
		action="/style-guide/date-range-fields"
		onSubmit={stay}
		legend="Choose the days"
		status="Showing the days you chose."
	>
		<DateRangeFields bind:from bind:to />
	</FilterBar>
</stack-l>
