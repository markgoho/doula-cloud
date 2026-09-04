<script lang="ts">
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
</script>

<stack-l space="var(--space-6)">
	<h1>Description list</h1>

	<section>
		<h2>Default</h2>
		<DescriptionList
			items={[
				{ label: 'Status', value: 'Stripe onboarding incomplete' },
				{ label: 'Contract signed', value: '14 September 2027 at 09:42' },
				{ label: 'Client', value: 'Anne-Marie Ochieng-Whitfield' },
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
				},
				/*
				 * The label is free text too (#725), and this component is the
				 * one place that was polite about it: a Client Field Template's
				 * own field label is a question the Practice types, so the same
				 * URL belongs on both sides of a row rather than only on the
				 * value. #720's shape rule -- a row set realizes every state a
				 * field renders differently -- is why this is a fourth row and
				 * not a rewrite of the one above.
				 */
				{
					label:
						'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
					value: 'Epidural, as early as it can be given'
				}
			]}
		/>
	</section>
</stack-l>
