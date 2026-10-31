<script lang="ts">
	import RecordPaymentForm from '#lib/components/organisms/RecordPaymentForm.svelte';

	/*
	 * The longest realistic amount (ADR-0025): a four-figure birth package,
	 * as wide as a heading this form carries.
	 */
	const amountCents = 425_000;
</script>

<stack-l space="var(--space-6)">
	<h1>Record payment form</h1>

	<section>
		<h2>A Client's Payment against an Invoice</h2>
		<p>How the money arrived, and the day it was received.</p>
		<RecordPaymentForm
			label="Record a payment"
			heading="Record a payment of $4,250.00"
			{amountCents}
			dateLabel="Date received"
			dateMissingMessage="Enter the date the payment was received"
			practiceTimezone="America/New_York"
			onConfirm={async () => {}}
			onCancel={() => {}}
		/>
	</section>

	<section>
		<h2>A Doula's fee, paid by the Practice</h2>
		<p>The same questions, asking the day it was paid.</p>
		<RecordPaymentForm
			label="Record a fee paid"
			heading="Record paying Alexandra Montgomery-Vasquez $4,250.00"
			{amountCents}
			dateLabel="Date paid"
			dateMissingMessage="Enter the date the fee was paid"
			practiceTimezone="America/New_York"
			onConfirm={async () => {}}
			onCancel={() => {}}
		/>
	</section>
</stack-l>
