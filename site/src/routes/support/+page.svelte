<!--
/support (#390, #358): what DoulaCloud sells, who to contact, and the
refund position. Copy, verbatim, and why each sentence is there:
docs/copy/support-page.md.

Stripe reads this page to activate the platform account and re-checks it
for as long as the account is live (#419), so it carries a description of
the service, a support contact and a refund position for good. Indexable,
and with no canonical tag, as the copy document sets it. Reachable with no
password, no cookie and no JavaScript.
-->
<script lang="ts">
	import PageSection from '#lib/components/PageSection.svelte';
	import ReadingPage from '#lib/components/ReadingPage.svelte';
	import { PRODUCT_NAME } from '#lib/product.js';

	const title = 'Support and billing';
</script>

<svelte:head>
	<title>{title} | {PRODUCT_NAME}</title>
	<meta
		name="description"
		content="What {PRODUCT_NAME} is, what it sells, how refunds work, and how to reach a person."
	/>
</svelte:head>

<ReadingPage {title} updated="2026-09-29">
	<PageSection heading="What {PRODUCT_NAME} is">
		<p>
			{PRODUCT_NAME} is practice-management software for doulas and doula agencies. A practice uses it to
			keep its client records, plan and schedule visits, message its clients, send and sign its
			contracts, and invoice its own clients for its own services.
		</p>
		<p>
			{PRODUCT_NAME} is not a payment service. When a practice invoices a client, the practice is the
			merchant: it holds its own agreement with Stripe, the money is paid into its own account, and it
			is responsible for the care it provides and for what it charges. {PRODUCT_NAME} never receives or
			holds a practice's money.
		</p>
		<p>
			{PRODUCT_NAME} is in a private pilot with a small number of practices, ahead of a public launch in
			January 2027.
		</p>
	</PageSection>

	<PageSection heading="What we sell">
		<p>
			Practices buy <strong>Credits</strong>. One Credit covers one client engagement — a single client
			relationship, from intake through the end of care.
		</p>
		<p>
			Credits do not expire. There is no subscription and no recurring charge: a practice buys Credits
			when it wants them, and is charged nothing in between.
		</p>
	</PageSection>

	<PageSection heading="Refunds and cancellation">
		<p>
			There is nothing to cancel. {PRODUCT_NAME} bills no recurring fee, so a practice that stops buying
			Credits is charged nothing further, and may close its account at any time.
		</p>
		<p>
			Unspent Credits that a practice has purchased can be refunded within three years of the date they
			were bought, at the price paid for them and together with any sales tax charged on them, to the
			original payment method. Credits given free of charge are not refundable. A Credit already used to
			start an engagement has been spent, and is not refundable.
		</p>
		<p>
			Credits themselves do not expire, and a practice can spend them whenever it likes. It is only the
			right to a cash refund that runs out.
		</p>
		<p>To ask for a refund, email us. We do not need a reason.</p>
	</PageSection>

	<PageSection heading="If you think a charge is wrong">
		<p>
			Email us before disputing the charge with your bank. If we have charged you in error we will
			refund it. We would rather fix it than argue about it.
		</p>
	</PageSection>

	<PageSection heading="Contact us">
		<p>
			Email <strong>hello@doula.cloud</strong>. That address reaches a person, and it is also where
			privacy and data requests go.
		</p>
	</PageSection>
</ReadingPage>
