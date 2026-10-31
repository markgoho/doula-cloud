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
	 *
	 * A payment on its way (#2225) -- a bank debit, for days -- stands
	 * where the Payment Element would be: there is nothing more for her to
	 * pay, and the server refuses a second secret for it. A method that
	 * left the page (Klarna, Cash App Pay) comes back here with Stripe's
	 * query on the address. That query is taken off at once, so a reload or
	 * a shared link never replays it, and its client secret is handed to
	 * PayInvoice to read the real outcome from Stripe.
	 */
	import { onMount } from 'svelte';
	import { replaceState } from '$app/navigation';
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import { invoiceFacts, loadClientInvoice, type ClientInvoice } from '#lib/clientInvoice.js';
	import { loadClientPayment, type ClientPaymentRead } from '#lib/clientPayment.js';
	import { formatMoney } from '#lib/money.js';
	import {
		byHandPaymentNotice,
		cardPaymentsNotSetUpNotice,
		INVOICES_HEADING,
		PAYMENT_ON_ITS_WAY_MESSAGE
	} from '#lib/clientRegister.js';
	import SummaryList from '#lib/components/molecules/SummaryList.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import DocumentPage from '#lib/components/templates/DocumentPage.svelte';
	import PayInvoice from './PayInvoice.svelte';

	let invoice = $state<ClientInvoice | undefined>();
	let error = $state('');
	// What the Payment Element mounts with, or the Practice's setup in the
	// way (#2039), or the server's own sentence for why this Invoice cannot
	// be paid here. Read fresh on every visit: the secret is a credential
	// for one payment and is never kept.
	let payment = $state<ClientPaymentRead | undefined>();
	let paymentError = $state('');
	// Stripe's return from a method that left the page, read once and
	// taken off the address.
	let returnedClientSecret = $state<string | undefined>();
	// The address Stripe brings her back to: this page, with no query.
	let returnUrl = $state('');

	/**
	Stripe's three parameters on a return (its `return_url` docs).
	*/
	const RETURN_PARAMETERS = ['payment_intent', 'payment_intent_client_secret', 'redirect_status'];

	onMount(async () => {
		const url = new URL(page.url.href);
		returnedClientSecret = url.searchParams.get('payment_intent_client_secret') ?? undefined;
		if (RETURN_PARAMETERS.some((name) => url.searchParams.has(name))) {
			for (const name of RETURN_PARAMETERS) url.searchParams.delete(name);
			replaceState(url, {});
		}
		returnUrl = url.href;
		try {
			invoice = await loadClientInvoice(apiFetchWithSession, page.params.engagementId!, page.params.invoiceId!);
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load invoice';
			return;
		}
		// Only an open Stripe-rail Invoice can be paid here; asking for any
		// other would be refused, and the by-hand rail has nothing to ask.
		if (invoice.status === 'open' && invoice.billingMode === 'stripe' && !invoice.paymentProcessing) {
			try {
				payment = await loadClientPayment(apiFetchWithSession, page.params.engagementId!, page.params.invoiceId!);
			} catch (error_) {
				paymentError = error_ instanceof Error ? error_.message : 'We could not start this payment. Try again.';
				await rereadAfterRefusal();
			}
		}
	});

	/** A refusal can mean the Invoice moved between the two reads: on her
	 * way back from Klarna, Stripe's `invoice.paid` often lands in that
	 * moment (seen on #2225's walk). Read it once more, and if it is no
	 * longer waiting for a payment, show where it stands now in place of
	 * the refusal. */
	async function rereadAfterRefusal() {
		try {
			const now = await loadClientInvoice(apiFetchWithSession, page.params.engagementId!, page.params.invoiceId!);
			if (now.status !== 'open' || now.paymentProcessing) {
				invoice = now;
				paymentError = '';
			}
		} catch {
			// The refusal already on screen stands.
		}
	}

	/** Stripe says her payment is on its way: the page says so where the
	 * Element was, as it will once the webhook has recorded it. */
	function markProcessing() {
		if (invoice) invoice = { ...invoice, paymentProcessing: true };
	}
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
	loading={invoice === undefined ? 'Loading invoice' : undefined}
>
	{#snippet content()}
		<!-- `content` renders only once the Template's own states are past,
		     so `invoice` is a loaded Invoice here. -->
		{#if invoice}
			<SummaryList rows={invoiceFacts(invoice)} />
			{#if invoice.status === 'open'}
				{#if invoice.billingMode === 'stripe' && invoice.paymentProcessing}
					<Notice variant="status" message={PAYMENT_ON_ITS_WAY_MESSAGE} />
				{:else if invoice.billingMode === 'stripe'}
					<!--
						The Payment Element and the pay action (#1020). A Practice that
						cannot take a card yet is a fact about the Practice, stated
						with the way to pay another way (#2039, #2002's rule): info,
						never an error. Any other refusal is the server's own sentence.
						Nothing pay-like appears for either.
					-->
					{#if payment?.kind === 'ready'}
						<PayInvoice
							payment={payment.payment}
							amount={formatMoney(invoice.amountCents, invoice.currency)}
							{returnUrl}
							{returnedClientSecret}
							onProcessing={markProcessing}
						/>
					{:else if payment?.kind === 'notSetUp'}
						<Notice variant="info" message={cardPaymentsNotSetUpNotice(page.data.practiceName)} />
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
