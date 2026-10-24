<!--
@component
One version of the Terms of Service or the Privacy Policy, whole: its
title, any newer version and when it takes effect, the version's own text,
and the history of every version (#1556, #1816). /terms and /privacy
render the version in force at the build; /terms/<date> and
/privacy/<date> render the version they name.
-->
<script lang="ts">
	import type { LegalDocument, LegalVersion } from '#lib/legal.js';
	import { textOf } from '#lib/legalTexts.js';
	import NewVersionNotice from './NewVersionNotice.svelte';
	import ReadingPage from './ReadingPage.svelte';
	import VersionHistory from './VersionHistory.svelte';

	interface Properties {
		legalDocument: LegalDocument;
		version: LegalVersion;
	}

	let { legalDocument, version }: Properties = $props();

	const Text = $derived(textOf(legalDocument, version));
</script>

<ReadingPage title={legalDocument.name} updated={version.effective}>
	<NewVersionNotice {legalDocument} {version} />
	<Text effective={version.effective} />
	<VersionHistory {legalDocument} />
</ReadingPage>
