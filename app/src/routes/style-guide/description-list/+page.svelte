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
				},
				/*
				 * The longest realistic value, not a representative one
				 * (ADR-0025): a Practice pastes a referral link into a free-text
				 * field, and a URL has no break opportunity a browser will take.
				 * This exact value is the one #530 measured -- it puts the
				 * approval screen 93px past its own edge at 320px -- so the
				 * drag surface shows that break rather than a polite fixture.
				 */
				{
					label: 'Referral',
					value:
						'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake'
				}
			]}
		/>
	</section>
</stack-l>
