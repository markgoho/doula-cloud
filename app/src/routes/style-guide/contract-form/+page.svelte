<script lang="ts">
	import ContractForm from '#lib/components/molecules/ContractForm.svelte';

	/*
	 * The longest realistic value, not a representative one (ADR-0025): a
	 * Practice's registered name is long, a Client's name is hyphenated and
	 * double-barrelled, and a birth package is a four-figure amount.
	 */
	let values = $state<Record<string, string>>({
		'{{practice_name}}': 'Highland Midwifery & Birth Support Collective of Western New York'
	});
</script>

<stack-l space="var(--space-6)">
	<h1>Contract form</h1>

	<section>
		<h2>Editable</h2>
		<ContractForm
			mergeFields={['{{practice_name}}', '{{client_name}}', '{{price}}']}
			{values}
			readOnly={false}
			onValueChange={(key, value) => (values[key] = value)}
		/>
	</section>

	<section>
		<h2>Read-only, once the Contract has been sent</h2>
		<ContractForm
			mergeFields={['{{practice_name}}', '{{client_name}}']}
			values={{
				'{{practice_name}}': 'Highland Midwifery & Birth Support Collective of Western New York',
				'{{client_name}}': 'Anne-Marie Ochieng-Whitfield'
			}}
			readOnly
			onValueChange={() => {}}
		/>
	</section>
</stack-l>
