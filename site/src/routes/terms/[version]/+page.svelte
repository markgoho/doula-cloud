<!--
One version of the Terms of Service, at /terms/<date> (#1816). The same
page as /terms for the version it names; it reads no clock, so it says
the same whenever it was built. Its own canonical address, and no entry
in sitemap.xml: /terms is the page a crawler lists.
-->
<script lang="ts">
	import LegalPage from '#lib/components/LegalPage.svelte';
	import { TERMS, versionPath } from '#lib/legal.js';
	import { formatUpdated } from '#lib/practicePage.js';
	import { PRODUCT_NAME, SITE_ORIGIN } from '#lib/product.js';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();

	const date = $derived(formatUpdated(data.version.effective));
</script>

<svelte:head>
	<title>{TERMS.name}, the version of {date.text} | {PRODUCT_NAME}</title>
	<meta
		name="description"
		content="The version of the {PRODUCT_NAME} {TERMS.name} that takes effect on {date.text}."
	/>
	<link rel="canonical" href="{SITE_ORIGIN}{versionPath(TERMS, data.version)}" />
</svelte:head>

<LegalPage legalDocument={TERMS} version={data.version} linkTheVersionInForce />
