<script lang="ts">
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
</script>

<stack-l space="var(--space-6)">
	<h1>Description list</h1>

	<section>
		<h2>Default</h2>
		<DescriptionList
			items={[
				{ label: 'Status', value: 'Active' },
				{ label: 'Created', value: '1/1/2026' },
				{
					label: 'Address',
					value: '128 Meadowbrook Lane, Apartment 4B, Rochester, NY 14620'
				}
			]}
		/>
	</section>
</stack-l>
