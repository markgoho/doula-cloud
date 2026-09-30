<!--
@component
The last section of the Terms of Service and of the Privacy Policy: every
version of the document, newest first, each with the date it took effect,
whether the change was material, and one line of what changed (#1556).
A reader sees what moved without comparing two texts by eye, and #1548
and #1549 read the same facts from the BFF's copy of this history.
-->
<script lang="ts">
	import type { LegalDocument } from '#lib/legal.js';
	import { formatUpdated } from '#lib/practicePage.js';
	import PageSection from './PageSection.svelte';

	interface Properties {
		legalDocument: LegalDocument;
	}

	let { legalDocument }: Properties = $props();

	const newestFirst = $derived(legalDocument.versions.toReversed());
</script>

<PageSection heading="Version history">
	<ol>
		{#each newestFirst as version (version.effective)}
			{@const date = formatUpdated(version.effective)}
			<li>
				<time datetime={date.datetime}>{date.text}</time>.
				<!-- One string, not two expressions: Svelte compiles each bare
				     expression with a `?? ''` fallback that no version can reach,
				     which the coverage gate reads as an untested branch. -->
				{`${version.material ? 'A material change.' : 'Not a material change.'} ${version.change}`}
			</li>
		{/each}
	</ol>
</PageSection>

<style>
	ol {
		padding-inline-start: var(--space-6);
	}
</style>
