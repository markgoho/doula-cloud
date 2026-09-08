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

	const paymentsSettingsHref = 'https://example.test/practices/practice-1/settings/payments';
</script>

<stack-l space="var(--space-6)">
	<h1>Invoice section</h1>

	<section>
		<h2>Clients can pay</h2>
		<InvoiceSection
			{invoices}
			contractStatus="signed"
			clientsCanPay={true}
			hasClientEmail={true}
			isOwner={false}
			{paymentsSettingsHref}
			onCreate={async () => {}}
		/>
	</section>

	<section>
		<h2>No Invoices yet</h2>
		<InvoiceSection
			invoices={[]}
			contractStatus="signed"
			clientsCanPay={true}
			hasClientEmail={true}
			isOwner={false}
			{paymentsSettingsHref}
			onCreate={async () => {}}
		/>
	</section>

	<!-- #270: Clients cannot pay this Practice yet -- an Owner gets a link
	     to the Payments settings screen, anyone else gets the same Notice
	     with no link (a Practice may have more than one Owner, and this
	     screen does not fetch the Staff roster to name one). -->
	<section>
		<h2>Clients cannot pay, seen by the Owner</h2>
		<InvoiceSection
			invoices={[]}
			contractStatus="signed"
			clientsCanPay={false}
			hasClientEmail={true}
			isOwner={true}
			{paymentsSettingsHref}
			onCreate={async () => {}}
		/>
	</section>

	<section>
		<h2>Clients cannot pay, seen by anyone else</h2>
		<InvoiceSection
			invoices={[]}
			contractStatus="signed"
			clientsCanPay={false}
			hasClientEmail={true}
			isOwner={false}
			{paymentsSettingsHref}
			onCreate={async () => {}}
		/>
	</section>

	<section>
		<h2>Client has no email on file</h2>
		<InvoiceSection
			invoices={[]}
			contractStatus="signed"
			clientsCanPay={true}
			hasClientEmail={false}
			isOwner={false}
			{paymentsSettingsHref}
			onCreate={async () => {}}
		/>
	</section>

	<!-- #275: a Contract that cannot be billed hides Create Invoice and says
	     why, whether or not Clients can pay. -->
	<section>
		<h2>Contract not yet signed</h2>
		<InvoiceSection
			invoices={[]}
			contractStatus="sent"
			clientsCanPay={true}
			hasClientEmail={true}
			isOwner={false}
			{paymentsSettingsHref}
			onCreate={async () => {}}
		/>
	</section>

	<section>
		<h2>Contract voided</h2>
		<InvoiceSection
			invoices={[]}
			contractStatus="voided"
			clientsCanPay={true}
			hasClientEmail={true}
			isOwner={false}
			{paymentsSettingsHref}
			onCreate={async () => {}}
		/>
	</section>
</stack-l>
