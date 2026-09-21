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
			paidAt: '2027-09-28T00:00:00Z',
			dueAt: '2027-10-14T00:00:00Z',
			reference: 'DC-0014',
			refundedCents: 0,
			billingMode: 'stripe'
		},
		{
			id: 'in_2',
			contractId: 'c_1',
			status: 'open',
			amountCents: 425_000,
			currency: 'usd',
			createdAt: '2027-11-30T00:00:00Z',
			dueAt: '2027-12-30T00:00:00Z',
			reference: 'INV-0002',
			refundedCents: 0,
			billingMode: 'by_hand'
		}
	];

	/*
	 * #1009: an Owner's view once money has gone back -- a partly refunded
	 * by-hand Invoice, whose row carries the returned figure beside "Paid"
	 * and still offers Return money but no longer Reverse payment, beside a
	 * card-paid one that offers Return money alone. The widest figure again:
	 * most of a four-figure package returned, to the cent.
	 */
	const refundedInvoices: Invoice[] = [
		{
			...invoices[0],
			id: 'in_3',
			reference: 'INV-0003',
			billingMode: 'by_hand',
			activePaymentId: 'pay_3',
			activePaymentKind: 'manual',
			refundedCents: 387_550
		},
		{ ...invoices[0], id: 'in_4', activePaymentId: 'pay_4', activePaymentKind: 'stripe' }
	];

	const paymentsSettingsHref = 'https://example.test/practices/practice-1/settings/payments';
	// #1280: any real IANA zone does -- this page's own todayIsoDate calls
	// don't depend on which one, only that a real name is there.
	const practiceTimezone = 'America/New_York';
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
			isOwnerOrAdmin={false}
			billingMode="stripe"
			{paymentsSettingsHref}
			{practiceTimezone}
			onCreate={async () => {}}
			onRecordPayment={async () => {}}
			onVoidInvoice={async () => {}}
			onWriteOffInvoice={async () => {}}
			onReversePayment={async () => {}}
			onRefundPayment={async () => {}}
		/>
	</section>

	<section>
		<h2>An Owner, after money was returned</h2>
		<InvoiceSection
			invoices={refundedInvoices}
			contractStatus="signed"
			clientsCanPay={true}
			hasClientEmail={true}
			isOwner={true}
			isOwnerOrAdmin={true}
			billingMode="stripe"
			{paymentsSettingsHref}
			{practiceTimezone}
			onCreate={async () => {}}
			onRecordPayment={async () => {}}
			onVoidInvoice={async () => {}}
			onWriteOffInvoice={async () => {}}
			onReversePayment={async () => {}}
			onRefundPayment={async () => {}}
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
			isOwnerOrAdmin={false}
			billingMode="stripe"
			{paymentsSettingsHref}
			{practiceTimezone}
			onCreate={async () => {}}
			onRecordPayment={async () => {}}
			onVoidInvoice={async () => {}}
			onWriteOffInvoice={async () => {}}
			onReversePayment={async () => {}}
			onRefundPayment={async () => {}}
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
			isOwnerOrAdmin={true}
			billingMode="stripe"
			{paymentsSettingsHref}
			{practiceTimezone}
			onCreate={async () => {}}
			onRecordPayment={async () => {}}
			onVoidInvoice={async () => {}}
			onWriteOffInvoice={async () => {}}
			onReversePayment={async () => {}}
			onRefundPayment={async () => {}}
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
			isOwnerOrAdmin={false}
			billingMode="stripe"
			{paymentsSettingsHref}
			{practiceTimezone}
			onCreate={async () => {}}
			onRecordPayment={async () => {}}
			onVoidInvoice={async () => {}}
			onWriteOffInvoice={async () => {}}
			onReversePayment={async () => {}}
			onRefundPayment={async () => {}}
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
			isOwnerOrAdmin={false}
			billingMode="stripe"
			{paymentsSettingsHref}
			{practiceTimezone}
			onCreate={async () => {}}
			onRecordPayment={async () => {}}
			onVoidInvoice={async () => {}}
			onWriteOffInvoice={async () => {}}
			onReversePayment={async () => {}}
			onRefundPayment={async () => {}}
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
			isOwnerOrAdmin={false}
			billingMode="stripe"
			{paymentsSettingsHref}
			{practiceTimezone}
			onCreate={async () => {}}
			onRecordPayment={async () => {}}
			onVoidInvoice={async () => {}}
			onWriteOffInvoice={async () => {}}
			onReversePayment={async () => {}}
			onRefundPayment={async () => {}}
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
			isOwnerOrAdmin={false}
			billingMode="stripe"
			{paymentsSettingsHref}
			{practiceTimezone}
			onCreate={async () => {}}
			onRecordPayment={async () => {}}
			onVoidInvoice={async () => {}}
			onWriteOffInvoice={async () => {}}
			onReversePayment={async () => {}}
			onRefundPayment={async () => {}}
		/>
	</section>

	<!-- #271: the Practice has never chosen a rail -- asked inline, on the
	     first Invoice raise, rather than assumed from an absent Stripe
	     Connect account. -->
	<section>
		<h2>Billing mode not yet chosen</h2>
		<InvoiceSection
			invoices={[]}
			contractStatus="signed"
			clientsCanPay={true}
			hasClientEmail={true}
			isOwner={false}
			isOwnerOrAdmin={false}
			billingMode={undefined}
			{paymentsSettingsHref}
			{practiceTimezone}
			onCreate={async () => {}}
			onRecordPayment={async () => {}}
			onVoidInvoice={async () => {}}
			onWriteOffInvoice={async () => {}}
			onReversePayment={async () => {}}
			onRefundPayment={async () => {}}
		/>
	</section>

	<!-- #271: Owner/Admin, an open by-hand Invoice -- Record payment, Void
	     and Write off all offered; a Stripe-backed open Invoice below
	     offers only Record payment. -->
	<section>
		<h2>Owner or Admin: recording a Payment, voiding, or writing off</h2>
		<InvoiceSection
			{invoices}
			contractStatus="signed"
			clientsCanPay={true}
			hasClientEmail={true}
			isOwner={true}
			isOwnerOrAdmin={true}
			billingMode="by_hand"
			{paymentsSettingsHref}
			{practiceTimezone}
			onCreate={async () => {}}
			onRecordPayment={async () => {}}
			onVoidInvoice={async () => {}}
			onWriteOffInvoice={async () => {}}
			onReversePayment={async () => {}}
			onRefundPayment={async () => {}}
		/>
	</section>
</stack-l>
