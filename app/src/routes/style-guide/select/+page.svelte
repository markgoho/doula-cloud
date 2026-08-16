<script lang="ts">
	import Select from '#lib/components/atoms/Select.svelte';

	const options = ['Home', 'Hospital', 'Birth center'];

	let value = $state('');
</script>

<stack-l space="var(--space-6)">
	<h1>Select</h1>

	<section>
		<h2>Default</h2>
		<Select {options} bind:value placeholder="Choose a location" />
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
		<Select {options} value="Home" disabled />
	</section>
</stack-l>
