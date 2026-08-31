<script lang="ts">
	import Select from '#lib/components/atoms/Select.svelte';

	/*
	 * The longest realistic value, not a representative one (ADR-0025): a
	 * Select's options are an enum, so the longest realistic option is the
	 * longest member a Practice can actually pick -- a named birth center
	 * rather than the word "Hospital".
	 */
	const options = [
		'Home',
		'Rochester General Hospital Birthing Center',
		'Highland Midwifery & Birth Support Collective'
	];

	let value = $state('');
</script>

<stack-l space="var(--space-6)">
	<h1>Select</h1>

	<section>
		<h2>Default</h2>
		<Select {options} bind:value placeholder="Choose where this birth is planned to happen" />
	</section>

	<section>
		<h2>Focus</h2>
		<p>Tab to any control on this page to see its focus outline.</p>
	</section>

	<section>
		<h2>Invalid</h2>
		<Select {options} value="" invalid />
	</section>

	<section>
		<h2>Disabled</h2>
		<Select {options} value="Rochester General Hospital Birthing Center" disabled />
	</section>
</stack-l>
