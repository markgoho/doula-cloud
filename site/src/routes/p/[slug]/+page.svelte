<!--
A Practice's public page (#441), at doula.cloud/p/<slug>/.

Every connected account has to declare a website to Stripe, and #421
walked what happens to a Practice who has none: she completes hosted
onboarding, submits, and returns with charges_enabled false and nothing on
screen saying why. This page is what a Practice without a website of her
own declares instead.

It carries what Stripe's written standard asks for, established
first-party on #382: the business name, a description of the services, a
support contact, and a refund or cancellation position. Nothing is
optional -- scripts/sync-practice-pages.ts refuses to write a page missing
any of it, so there is no `if` around a required block.

Every value she typed is printed with `{...}`, never `{@html}`, so Svelte
escapes it for the context it lands in. That is the whole defense against a
cancellation policy with an angle bracket in it, and the spec beside this
file is what holds it in place.
-->
<script lang="ts">
	import PageSection from '#lib/components/PageSection.svelte';
	import ReadingPage from '#lib/components/ReadingPage.svelte';
	import { metaDescription } from '#lib/practicePage.js';
	import { PRODUCT_NAME, SITE_ORIGIN } from '#lib/product.js';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();
	const page = $derived(data.page);
</script>

<svelte:head>
	<title>{page.name}</title>
	<!-- Indexed and reachable: this is the Practice's public website, and a
	     hidden one would fail the job it exists to do. -->
	<meta name="description" content={metaDescription(page.serviceDescription)} />
	<link rel="canonical" href="{SITE_ORIGIN}/p/{page.slug}/" />
</svelte:head>

<!-- data-practice-page is what #443's probe looks for after a deploy. A
     status code alone would not do: a host that serves its own not-found
     page with a 200 would pass, and Stripe would go on holding a URL that
     shows a Client nothing. The same string is sitebuild.PageMarker in
     api/internal/sitebuild/http.go; it appears on this page and nowhere
     else on the site. -->
<ReadingPage
	title={page.name}
	updated={page.publishedAt}
	mainAttributes={{ 'data-practice-page': '' }}
>
	<PageSection heading="What we offer">
		<p class="prose">{page.serviceDescription}</p>
	</PageSection>

	<PageSection heading="Cancellations and refunds">
		<p class="prose">{page.cancellationPolicy}</p>
	</PageSection>

	<PageSection heading="Contact us">
		<!-- An address element, because that is what it is, and a mailto so
		     the contact is one tap on the phone this is most often read on. -->
		<address>
			{page.supportName}<br />
			<a href="mailto:{page.supportEmail}">{page.supportEmail}</a>
		</address>
	</PageSection>

	<section class="privacy" aria-label="Privacy">
		<!-- Stripe never asks for this -- #421 found no privacy field in the
		     hosted flow. It is here for the Client, who is about to pay a
		     business she found through a page on someone else's domain. One
		     shared block rather than a field on the form: it describes what
		     the product does with the data, which is the same for every
		     Practice, and a Practice asked to write her own would be asked
		     to write ours. -->
		<p>
			This page is published by {PRODUCT_NAME} on behalf of the practice named above.
			{PRODUCT_NAME} provides the software the practice uses to run its business, and stores the
			practice's records on its behalf. If you are a client of this practice, the practice decides
			what it records about you and how long it keeps it; write to the practice at the address above
			to ask what it holds or to ask it to correct something. {PRODUCT_NAME} does not sell any of it,
			and does not use it to advertise to you. Payments are processed by Stripe, which handles your
			card details directly; {PRODUCT_NAME} never sees or stores a card number.
		</p>
	</section>

	{#snippet footnote()}
		Published with <a href="{SITE_ORIGIN}/">{PRODUCT_NAME}</a>.
	{/snippet}
</ReadingPage>

<style>
	/* A Practice types one block of prose per box and may well put line
	   breaks in it. Preserving them costs nothing, and losing them turns a
	   three-clause cancellation policy into a wall. */
	.prose {
		white-space: pre-line;
	}

	address {
		font-style: normal;
	}

	.privacy {
		padding: var(--space-4) var(--space-5);
		border-radius: var(--radius);
		background-color: var(--color-surface-container);
		color: var(--color-on-surface-muted);
		font-size: var(--text-body-sm-size);
		line-height: var(--text-body-sm-leading);
	}
</style>
