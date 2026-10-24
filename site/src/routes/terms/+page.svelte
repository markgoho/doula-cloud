<!--
The Terms of Service (#1556): the version in force when the site was
built, and above its text each newer version that has not started yet
(#1816). The text of each version is its own component in
site/src/lib/legalTexts/, by date; site/src/lib/legalTexts.ts says how a
new version is added.

Indexed, unlike /pilot-terms: a Practice has to be able to find the terms
she agrees to, and a crawler is welcome to list them. So no robots meta,
a canonical address, and an entry in sitemap.xml.

The Refunds section is word for word the one on /pilot-terms, and the
spec beside this file fails when the two drift (ADR-0054).
-->
<script lang="ts">
	import LegalPage from '#lib/components/LegalPage.svelte';
	import { TERMS, versionInForce } from '#lib/legal.js';
	import { PRODUCT_NAME, SITE_ORIGIN } from '#lib/product.js';

	// The moment of the build: the site ships no JavaScript and has no
	// clock of its own (versionInForce says why that is enough).
	const version = versionInForce(TERMS, new Date());
</script>

<svelte:head>
	<title>{TERMS.name} | {PRODUCT_NAME}</title>
	<meta
		name="description"
		content="The agreement between a practice and Elephantine LLC, which operates {PRODUCT_NAME}: what a Credit costs, refunds, leaving, and how the terms change."
	/>
	<link rel="canonical" href="{SITE_ORIGIN}{TERMS.path}" />
</svelte:head>

<LegalPage legalDocument={TERMS} {version} />
