<script lang="ts">
	/*
	 * One Invoice, on a page of its own (#983, built on #1564). A Payment
	 * Element belongs to exactly one Invoice, so the payable surface is
	 * per-Invoice and needs somewhere to sit.
	 *
	 * The pay affordance itself is #1020's. This page draws what surrounds
	 * it: the Invoice's facts, a marked place for the Element on an open
	 * Stripe-rail Invoice, and on the by-hand rail the one sentence that
	 * stands where a button would be -- nothing to click, by construction.
	 */
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import { invoiceFacts, loadClientInvoice, type ClientInvoice } from '#lib/clientInvoice.js';
	import { loadClientPayment, type ClientPayment } from '#lib/clientPayment.js';
	import { formatMoney } from '#lib/money.js';
	import { byHandPaymentNotice, INVOICES_HEADING } from '#lib/clientRegister.js';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import DocumentPage from '#lib/components/templates/DocumentPage.svelte';
	import PayInvoice from './PayInvoice.svelte';

	let invoice = $state<ClientInvoice | undefined>();
	let error = $state('');
	// What the Payment Element mounts with, or the server's own sentence
	// for why this Invoice cannot be paid here. Read fresh on every visit:
	// the secret is a credential for one payment and is never kept.
	let payment = $state<ClientPayment | undefined>();
	let paymentError = $state('');

	onMount(async () => {
		try {
			invoice = await loadClientInvoice(apiFetchWithSession, page.params.engagementId!, page.params.invoiceId!);
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load Invoice';
			return;
		}
		// Only an open Stripe-rail Invoice can be paid here; asking for any
		// other would be refused, and the by-hand rail has nothing to ask.
		if (invoice.status === 'open' && invoice.billingMode === 'stripe') {
			try {
				payment = await loadClientPayment(apiFetchWithSession, page.params.engagementId!, page.params.invoiceId!);
			} catch (error_) {
				paymentError = error_ instanceof Error ? error_.message : 'We could not start this payment. Try again.';
			}
		}
	});
</script>

<!--
	No empty state: a missing Invoice is a load error, not a "none yet".
	The title is the Invoices heading until the Invoice arrives, so the
	<h1> and the tab title always name the screen.
-->
<DocumentPage
	title={invoice ? `Invoice ${invoice.reference}` : INVOICES_HEADING}
	serviceName={page.data.practiceName}
	backHref={resolve('/portal/(authenticated)/engagements/[engagementId]/invoices', {
		engagementId: page.params.engagementId!
	})}
	backLabel="Back to {INVOICES_HEADING}"
	loadError={error || undefined}
	loading={invoice === undefined ? 'Loading Invoice' : undefined}
>
	{#snippet content()}
		<!-- `content` renders only once the Template's own states are past,
		     so `invoice` is a loaded Invoice here. -->
		{#if invoice}
			<DescriptionList items={invoiceFacts(invoice)} />
			{#if invoice.status === 'open'}
				{#if invoice.billingMode === 'stripe'}
					<!--
						The Payment Element and the pay action (#1020). When the server
						refuses the secret -- the Practice cannot take a card yet, say --
						she reads its own sentence and nothing pay-like appears.
					-->
					{#if payment}
						<PayInvoice
							{payment}
							amount={formatMoney(invoice.amountCents, invoice.currency)}
							returnUrl={page.url.href}
						/>
					{:else if paymentError}
						<Notice variant="error" message={paymentError} />
					{/if}
				{:else}
					<Text text={byHandPaymentNotice(invoice.reference)} />
				{/if}
			{/if}
		{/if}
	{/snippet}
</DocumentPage>
