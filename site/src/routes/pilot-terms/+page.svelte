<!--
The pilot terms (#444). Copy, and why each line says what it says:
docs/copy/pilot-terms-page.md.

Unlisted, not secret. Stripe's rule for a promotion is placement -- the
terms are visible where a practice agrees to take part -- not publicity,
and the pilot is private. So: noindex here, no link from anywhere on the
site, and left out of sitemap.xml as well. A noindexed page listed in the
sitemap is a mixed signal that publishes the URL anyway, which is the one
thing being avoided.
-->
<script lang="ts">
	import PageSection from '#lib/components/PageSection.svelte';
	import ReadingPage from '#lib/components/ReadingPage.svelte';
	import { PRODUCT_NAME, SITE_ORIGIN } from '#lib/product.js';

	const title = 'Pilot terms';
</script>

<svelte:head>
	<title>{title} | {PRODUCT_NAME}</title>
	<meta name="robots" content="noindex, nofollow" />
	<link rel="canonical" href="{SITE_ORIGIN}/pilot-terms/" />
</svelte:head>

<ReadingPage {title} updated="2026-08-29">
	<stack-l>
		<p>
			{PRODUCT_NAME} is in a private pilot with a small number of practices, from October 2026 until
			it opens to everyone in January 2027.
		</p>
		<p>
			This page says what a practice is offered for taking part, and what it is charged. It applies
			to every practice invited into the pilot.
		</p>
	</stack-l>

	<PageSection heading="What a pilot practice is given">
		<p>
			A practice that joins the pilot is granted <strong>three free Credits for each person on its
				staff</strong>. A solo doula is granted three. A practice of fourteen doulas is granted
			forty-two.
		</p>
		<p>
			The grant is counted once, at the moment the practice joins. Adding a doula later grants
			nothing further, and the grant is not repeated.
		</p>
		<p>
			Granted Credits do not expire. They run out by being used, not by a date, and any left unused
			stay in the account.
		</p>
	</PageSection>

	<PageSection heading="What a Credit is, and what it costs">
		<p>
			One Credit covers one engagement — a single client relationship centered on one baby, from
			intake through the end of care. Birth and postpartum engagements cost the same.
		</p>
		<p>
			A Credit costs <strong>$20.00</strong>, plus sales tax where it applies. That is the ordinary
			price of a Credit, not a pilot price, and it is what a practice pays for every Credit beyond its
			grant.
		</p>
		<p>
			There is no subscription and no recurring charge. A practice buys Credits when it wants them and
			is charged nothing in between.
		</p>
	</PageSection>

	<PageSection heading="What happens when the pilot ends">
		<p>Nothing changes when {PRODUCT_NAME} opens to everyone in January 2027.</p>
		<ul>
			<li>A Credit still costs $20.00.</li>
			<li>Unused Credits stay, granted and purchased alike.</li>
			<li>The account carries on. There is nothing to renew, and nothing to cancel.</li>
		</ul>
		<p>
			The founding grant is the one thing the pilot offers that January does not, and it is granted
			once, when a practice joins.
		</p>
	</PageSection>

	<PageSection heading="Refunds">
		<p>Credits given free of charge are not refundable.</p>
		<p>
			Purchased Credits that have not been spent can be refunded within three years of the date they
			were bought, at the price paid for them and together with any sales tax charged on them, to the
			original payment method. A Credit already used to start an engagement has been spent, and is not
			refundable. To ask for a refund, email us. We do not need a reason.
		</p>
	</PageSection>

	<PageSection heading="The pilot does not change who the merchant is">
		<p>
			{PRODUCT_NAME} is not a payment service. When a practice invoices a client, the practice is the
			merchant: it holds its own agreement with Stripe, the money is paid into its own account, and it
			is responsible for the care it provides and for what it charges. {PRODUCT_NAME} never receives or
			holds a practice's money.
		</p>
	</PageSection>

	<PageSection heading="Contact us">
		<p>Email <strong>hello@doula.cloud</strong>. That address reaches a person.</p>
	</PageSection>
</ReadingPage>

<style>
	ul {
		padding-inline-start: var(--space-6);
	}
</style>
