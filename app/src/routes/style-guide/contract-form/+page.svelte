<script lang="ts">
	import ContractForm from '#lib/components/molecules/ContractForm.svelte';

	let values = $state<Record<string, string>>({ '{{practice_name}}': 'Hudson Valley Doulas' });
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
			values={{ '{{practice_name}}': 'Hudson Valley Doulas', '{{client_name}}': 'Sarah Beck' }}
			readOnly
			onValueChange={() => {}}
		/>
	</section>
</stack-l>
