<script lang="ts">
	import ContractView from '#lib/components/molecules/ContractView.svelte';
</script>

<stack-l space="var(--space-6)">
	<h1>Contract view</h1>

	<section>
		<h2>Merge fields filled in</h2>
		<ContractView
			prose={'This agreement is between {{practice_name}} and {{client_name}}.\n\nThe fee is {{price}}.'}
			values={{ practice_name: 'Hudson Valley Doulas', client_name: 'Sarah Beck', price: '$2,400' }}
		/>
	</section>
</stack-l>
