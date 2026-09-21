<script lang="ts">
	import RefundPaymentForm from '#lib/components/organisms/RefundPaymentForm.svelte';
	import type { Invoice } from '#lib/invoice.js';

	/*
	 * The longest realistic value (ADR-0025): a four-figure birth package,
	 * part of it already returned, so the hint's remaining figure is as wide
	 * as the heading's.
	 */
	const byHand: Invoice = {
		id: 'in_1',
		contractId: 'c_1',
		status: 'paid',
		amountCents: 425_000,
		currency: 'usd',
		createdAt: '2027-09-14T00:00:00Z',
		paidAt: '2027-09-28T00:00:00Z',
		dueAt: '2027-10-14T00:00:00Z',
		reference: 'INV-0014',
		refundedCents: 37_450,
		billingMode: 'by_hand',
		activePaymentId: 'pay_1',
		activePaymentKind: 'manual'
	};

	const byCard: Invoice = { ...byHand, reference: 'DC-0014', billingMode: 'stripe', activePaymentKind: 'stripe' };
</script>

<stack-l space="var(--space-6)">
	<h1>Refund payment form</h1>

	<section>
		<h2>A Payment recorded by hand</h2>
		<p>Asks how the money went back.</p>
		<RefundPaymentForm invoice={byHand} onConfirm={async () => {}} onCancel={() => {}} />
	</section>

	<section>
		<h2>A card Payment Stripe collected</h2>
		<p>Stripe returns it, so there is no method to ask. The review step warns that Stripe keeps its fee.</p>
		<RefundPaymentForm invoice={byCard} onConfirm={async () => {}} onCancel={() => {}} />
	</section>
</stack-l>
