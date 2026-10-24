<!--
@component
At the top of a version of the Terms of Service or the Privacy Policy:
each version after it, with the date it takes effect, what changes, and a
link to its text (#1816). During the 30 days before a material version
starts, this is how /terms and /privacy show the coming version beside the
one in force.

It reads no clock. Each sentence says when a version takes effect, which
is true whenever the page was built, so it renders the same on a version's
own page long after the date.

GOV.UK's notification banner in its important-information form: a
section named by its heading, on the page when the reader arrives, so no
live region and no focus. Nothing when there is no later version.
-->
<script lang="ts">
	import { type LegalDocument, type LegalVersion, laterVersions, versionPath } from '#lib/legal.js';
	import { formatUpdated } from '#lib/practicePage.js';

	interface Properties {
		legalDocument: LegalDocument;
		version: LegalVersion;
	}

	let { legalDocument, version }: Properties = $props();

	const later = $derived(laterVersions(legalDocument, version));
</script>

{#each later as next (next.effective)}
	{@const date = formatUpdated(next.effective)}
	{@const headingId = `new-version-${next.effective}`}
	<section aria-labelledby={headingId}><!-- spelling:ignore: aria-labelledby is an ARIA attribute name -->
		<stack-l>
			<h2 id={headingId}>A new version takes effect on <time datetime={date.datetime}>{date.text}</time></h2>
			<p>{`${next.material ? 'A material change.' : 'Not a material change.'} ${next.change}`}</p>
			<!-- One string, as in VersionHistory: a bare expression beside
			     text compiles a fallback no version can reach. -->
			<p><a href={versionPath(legalDocument, next)}>{`Read the version that takes effect on ${date.text}`}</a></p>
		</stack-l>
	</section>
{/each}

<style>
	section {
		padding: var(--space-4);
		border: var(--border-thin) solid var(--color-info);
		border-radius: var(--radius);
		background-color: color-mix(in oklab, var(--color-info) 12%, var(--color-surface));
	}

	h2 {
		font-size: var(--text-heading-size);
		font-weight: var(--text-heading-weight);
		line-height: var(--text-heading-leading);
		letter-spacing: var(--text-heading-tracking);
	}
</style>
