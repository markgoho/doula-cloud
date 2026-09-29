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
	import { byHandPaymentNotice, INVOICES_HEADING } from '#lib/clientRegister.js';
	import BackLink from '#lib/components/molecules/BackLink.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';

	let invoice = $state<ClientInvoice | undefined>();
	let error = $state('');

	onMount(async () => {
		try {
			invoice = await loadClientInvoice(apiFetchWithSession, page.params.engagementId!, page.params.invoiceId!);
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load Invoice';
		}
	});
</script>

<container-l>
	<center-l max="var(--measure)" gutters="var(--page-gutter)">
		<stack-l space="var(--space-5)">
			<BackLink
				href={resolve('/portal/(authenticated)/engagements/[engagementId]/invoices', {
					engagementId: page.params.engagementId!
				})}
				label="Back to {INVOICES_HEADING}"
			/>

			{#if error}
				<PageTitle page={INVOICES_HEADING} serviceName={page.data.practiceName} />
				<Notice variant="error" message={error} />
			{:else if invoice === undefined}
				<PageTitle page={INVOICES_HEADING} serviceName={page.data.practiceName} />
				<Text text="Loading..." />
			{:else}
				<PageTitle page="Invoice {invoice.reference}" serviceName={page.data.practiceName} />
				<Heading level={1} text="Invoice {invoice.reference}" />
				<DescriptionList items={invoiceFacts(invoice)} />
				{#if invoice.status === 'open'}
					{#if invoice.billingMode === 'stripe'}
						<!--
							#1020 mounts the Payment Element and the pay action here. Empty
							on purpose: nothing a Client can read or press exists until
							that lands, and placeholder text would tell her something is
							coming that the product cannot promise.
						-->
						<div data-payment-element-mount></div>
					{:else}
						<Text text={byHandPaymentNotice(invoice.reference)} />
					{/if}
				{/if}
			{/if}
		</stack-l>
	</center-l>
</container-l>
