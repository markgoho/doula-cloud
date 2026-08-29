<script lang="ts">
	import ContractTemplateEditor from '#lib/components/organisms/ContractTemplateEditor.svelte';

	let prose = $state('This agreement is between {{practice_name}} and {{client_name}}.');
</script>

<stack-l space="var(--space-6)">
	<h1>Contract template editor</h1>

	<section>
		<h2>Default</h2>
		<ContractTemplateEditor {prose} onProseChange={(next) => (prose = next)} />
	</section>
</stack-l>
