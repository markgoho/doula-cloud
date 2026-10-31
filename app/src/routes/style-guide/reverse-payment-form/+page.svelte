<script lang="ts">
	import ReversePaymentForm from '#lib/components/organisms/ReversePaymentForm.svelte';
</script>

<stack-l space="var(--space-6)">
	<h1>Reverse payment form</h1>

	<section>
		<h2>A Client's Payment recorded by mistake</h2>
		<p>A reason, then a check step. The original record stays.</p>
		<ReversePaymentForm
			label="Reverse this payment"
			heading="Reverse a payment of $4,250.00"
			amountCents={425_000}
			reasonMissingMessage="Enter a reason for reversing this payment"
			confirmLabel="Confirm and reverse"
			onConfirm={async () => {}}
			onCancel={() => {}}
		/>
	</section>

	<section>
		<h2>A Doula's fee recorded as paid by mistake</h2>
		<p>The same questions, worded as removing the record.</p>
		<ReversePaymentForm
			label="Remove this fee payment"
			heading="Remove the $4,250.00 paid to Alexandra Montgomery-Vasquez"
			amountCents={425_000}
			reasonMissingMessage="Enter a reason for removing this payment"
			confirmLabel="Confirm and remove"
			onConfirm={async () => {}}
			onCancel={() => {}}
		/>
	</section>
</stack-l>
