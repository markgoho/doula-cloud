<script lang="ts">
	import WorkStateField from '#lib/components/molecules/WorkStateField.svelte';

	/*
	 * The longest realistic value, not a representative one (ADR-0025): the
	 * options are the fifty states plus DC, so the longest a Doula can pick
	 * is "District of Columbia" -- what is selected here.
	 */
	let value = $state('District of Columbia');
</script>

<stack-l space="var(--space-6)">
	<h1>Work state field</h1>

	<section>
		<h2>Default</h2>
		<WorkStateField bind:value />
	</section>
</stack-l>
