<script lang="ts">
	import ContractTemplateEditor from '#lib/components/organisms/ContractTemplateEditor.svelte';

	/*
	 * The longest realistic value, not a representative one (ADR-0025): a
	 * Contract template is a Practice's own prose, so it runs to
	 * paragraphs, and a four-figure fee and a pasted link both land in it.
	 */
	let prose = $state(
		'This agreement is between {{practice_name}} and {{client_name}}.\n\nThe fee is {{price}}, payable before the estimated due date. The birth plan on file is at https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake.'
	);
</script>

<stack-l space="var(--space-6)">
	<h1>Contract template editor</h1>

	<section>
		<h2>Default</h2>
		<ContractTemplateEditor {prose} onProseChange={(next) => (prose = next)} />
	</section>
</stack-l>
