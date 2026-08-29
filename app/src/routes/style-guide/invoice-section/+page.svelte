<script lang="ts">
	import InvoiceSection from '#lib/components/organisms/InvoiceSection.svelte';
	import type { Invoice } from '#lib/invoice.js';

	const invoices: Invoice[] = [
		{
			id: 'in_1',
			contractId: 'c_1',
			status: 'paid',
			amountCents: 120_000,
			currency: 'usd',
			createdAt: '2026-02-01T00:00:00Z',
			paidAt: '2026-02-08T00:00:00Z'
		},
		{
			id: 'in_2',
			contractId: 'c_1',
			status: 'open',
			amountCents: 120_000,
			currency: 'usd',
			createdAt: '2026-05-01T00:00:00Z'
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
