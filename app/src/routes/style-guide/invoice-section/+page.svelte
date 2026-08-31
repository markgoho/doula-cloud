<script lang="ts">
	import InvoiceSection from '#lib/components/organisms/InvoiceSection.svelte';
	import type { Invoice } from '#lib/invoice.js';

	/*
	 * The longest realistic value, not a representative one (ADR-0025): an
	 * Invoice shows only an amount and two dates, so the longest realistic
	 * value is a four-figure birth package rather than a single visit.
	 */
	const invoices: Invoice[] = [
		{
			id: 'in_1',
			contractId: 'c_1',
			status: 'paid',
			amountCents: 425_000,
			currency: 'usd',
			createdAt: '2027-09-14T00:00:00Z',
			paidAt: '2027-09-28T00:00:00Z'
		},
		{
			id: 'in_2',
			contractId: 'c_1',
			status: 'open',
			amountCents: 425_000,
			currency: 'usd',
			createdAt: '2027-11-30T00:00:00Z'
		}
	];
</script>

<stack-l space="var(--space-6)">
	<h1>Invoice section</h1>

	<section>
		<h2>Stripe connected</h2>
		<InvoiceSection {invoices} onCreate={async () => {}} onConnect={async () => {}} />
	</section>

	<section>
		<h2>No Invoices yet</h2>
		<InvoiceSection invoices={[]} onCreate={async () => {}} onConnect={async () => {}} />
	</section>

	<section>
		<h2>Connect gate, seen by the Owner</h2>
		<InvoiceSection
			invoices={[]}
			connectGate={{ isOwner: true }}
			onCreate={async () => {}}
			onConnect={async () => {}}
		/>
	</section>

	<section>
		<h2>Connect gate, seen by anyone else</h2>
		<InvoiceSection
			invoices={[]}
			connectGate={{ isOwner: false }}
			onCreate={async () => {}}
			onConnect={async () => {}}
		/>
	</section>
</stack-l>
