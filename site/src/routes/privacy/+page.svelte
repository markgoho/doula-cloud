<!--
The Privacy Policy (#1556): the version in force when the site was built,
and above its text each newer version that has not started yet (#1816).
The text of each version is its own component in
site/src/lib/legalTexts/, by date; site/src/lib/legalTexts.ts says how a
new version is added. A sentence about a company, a cookie or how long
something is kept is written only once it has been checked in
docs/copy/privacy-policy-page.md.

Indexed and conspicuously posted: CalOPPA asks for the second, and the
site footer on every page links here.
-->
<script lang="ts">
	import LegalPage from '#lib/components/LegalPage.svelte';
	import { PRIVACY, versionInForce } from '#lib/legal.js';
	import { PRODUCT_NAME, SITE_ORIGIN } from '#lib/product.js';

	// The moment of the build: the site ships no JavaScript and has no
	// clock of its own (versionInForce says why that is enough).
	const version = versionInForce(PRIVACY, new Date());
</script>

<svelte:head>
	<title>{PRIVACY.name} | {PRODUCT_NAME}</title>
	<meta
		name="description"
		content="What {PRODUCT_NAME} holds about a practice's owners, its staff, its clients and a visitor to this site, who else receives it, how long it is kept, and how to have it erased."
	/>
	<link rel="canonical" href="{SITE_ORIGIN}{PRIVACY.path}" />
</svelte:head>

<LegalPage legalDocument={PRIVACY} {version} />
