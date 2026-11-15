<script lang="ts">
	import FilterBar from '#lib/components/molecules/FilterBar.svelte';
	import SentPeriodFields from '#lib/components/molecules/SentPeriodFields.svelte';
	import type { SentPeriodChoice } from '#lib/sentPeriod.js';

	let choice = $state<SentPeriodChoice>('');
	let from = $state('');
	let to = $state('');
	let chosenChoice = $state<SentPeriodChoice>('choose');
	let chosenFrom = $state('2026-10-03');
	let chosenTo = $state('2026-10-17');

	function stay(event: SubmitEvent) {
		event.preventDefault();
	}
</script>

<stack-l space="var(--space-6)">
	<h1>Sent period fields</h1>

	<p>The "Sent" select of a list that narrows to the day a row was sent. "Choose days" opens From and To.</p>

	<section>
		<h2>Default</h2>
		<FilterBar action="/style-guide/sent-period-fields" onSubmit={stay} legend="Choose which rows to show" status="Showing 12 rows.">
			<SentPeriodFields bind:choice bind:from bind:to />
		</FilterBar>
	</section>

	<section>
		<h2>Chosen days</h2>
		<FilterBar action="/style-guide/sent-period-fields" onSubmit={stay} legend="Choose which rows to show" status="Showing 12 rows.">
			<SentPeriodFields bind:choice={chosenChoice} bind:from={chosenFrom} bind:to={chosenTo} />
		</FilterBar>
	</section>
</stack-l>
