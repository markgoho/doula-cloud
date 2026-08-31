<script lang="ts">
	import ContractView from '#lib/components/molecules/ContractView.svelte';
</script>

<stack-l space="var(--space-6)">
	<h1>Contract view</h1>

	<section>
		<h2>Merge fields filled in</h2>
		<!--
			The longest realistic value, not a representative one (ADR-0025): a
			Contract is where a Practice's registered name, a four-figure fee and
			a pasted link all land in the same paragraph, and a URL is the one
			value a browser will not break (#521).
		-->
		<ContractView
			prose={'This agreement is between {{practice_name}} and {{client_name}}.\n\nThe fee is {{price}}, payable before the estimated due date. The birth plan on file is at {{plan_url}}.'}
			values={{
				practice_name: 'Highland Midwifery & Birth Support Collective of Western New York',
				client_name: 'Anne-Marie Ochieng-Whitfield',
				price: '$4,250.00',
				plan_url:
					'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake'
			}}
		/>
	</section>
</stack-l>
