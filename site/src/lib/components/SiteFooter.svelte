<!--
@component
The foot of every page on the site, and its one `contentinfo` landmark: a
link to the Terms of Service and a link to the Privacy Policy (#1556), and
a link to /support, which Stripe reads (#390, #358).

It lives in the root layout rather than in ReadingPage because the home
page is not a ReadingPage, and each indexed page has to reach both
documents from where the reader is (CalOPPA asks for a conspicuous
posting). /pilot-terms carries it too; a link out of an unlisted page does
not list that page.

The links wrap onto a second line in a `cluster-l` when the column is too
narrow for both, so nothing here needs a media query (ADR-0024).
-->
<script lang="ts">
	import { PRIVACY, TERMS } from '#lib/legal.js';

	const links = [TERMS, PRIVACY, { name: 'Support', path: '/support' }];
</script>

<footer>
	<center-l>
		<cluster-l>
			{#each links as link (link.path)}
				<a href={link.path}>{link.name}</a>
			{/each}
		</cluster-l>
	</center-l>
</footer>

<style>
	footer {
		padding-block: var(--space-6) var(--space-8);
		padding-inline: var(--page-gutter);
		border-block-start: var(--border-thin) solid var(--color-outline-variant);
	}

	/* Centered, because the pages above do not share one column: a
	   ReadingPage is a measure wide and the teaser is a letter beside a
	   card, so a left edge that matched one would miss the other. */
	cluster-l {
		justify-content: center;
		column-gap: var(--space-6);
		color: var(--color-on-surface-muted);
		font-size: var(--text-meta-size);
		line-height: var(--text-meta-leading);
		letter-spacing: var(--text-meta-tracking);
	}

	a {
		color: var(--color-primary);
	}

	a:focus-visible {
		outline: var(--focus-ring-width) solid var(--color-primary);
		outline-offset: var(--focus-ring-offset);
	}
</style>
