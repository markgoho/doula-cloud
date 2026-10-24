<!--
One version of the Privacy Policy, at /privacy/<date> (#1816). The same
page as /privacy for the version it names; it reads no clock, so it says
the same whenever it was built. Its own canonical address, and no entry
in sitemap.xml: /privacy is the page a crawler lists.
-->
<script lang="ts">
	import LegalPage from '#lib/components/LegalPage.svelte';
	import { PRIVACY, versionPath } from '#lib/legal.js';
	import { formatUpdated } from '#lib/practicePage.js';
	import { PRODUCT_NAME, SITE_ORIGIN } from '#lib/product.js';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();

	const date = $derived(formatUpdated(data.version.effective));
</script>

<svelte:head>
	<title>{PRIVACY.name}, the version of {date.text} | {PRODUCT_NAME}</title>
	<meta
		name="description"
		content="The version of the {PRODUCT_NAME} {PRIVACY.name} that takes effect on {date.text}."
	/>
	<link rel="canonical" href="{SITE_ORIGIN}{versionPath(PRIVACY, data.version)}" />
</svelte:head>

<LegalPage legalDocument={PRIVACY} version={data.version} linkTheVersionInForce />
