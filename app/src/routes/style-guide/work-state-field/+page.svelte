<script lang="ts">
	import WorkStateField from '#lib/components/molecules/WorkStateField.svelte';

	let value = $state('');
</script>

<stack-l space="var(--space-6)">
	<h1>Work state field</h1>

	<section>
		<h2>Default</h2>
		<WorkStateField bind:value />
	</section>
</stack-l>
