<!--
@component
A short note from the founder, in the teaser's voice and look: the mark,
one `h1`, a few lines, and the teaser letter's signature. It is the page a
person lands on after the waitlist form and after confirming her email
(#1799), in place of Buttondown's own pages.

The column is `center-l` and `stack-l` at their defaults, which is pure
CSS: the measure makes it a column on a wide screen and the gutter makes
it fit a 320px one, with no media query (ADR-0024).
-->
<script lang="ts">
	import type { Snippet } from 'svelte';
	import { PRODUCT_NAME } from '#lib/product.js';
	import SiteMark from './SiteMark.svelte';

	interface Properties {
		// The page's one `h1`.
		title: string;
		children: Snippet;
	}

	let { title, children }: Properties = $props();
</script>

<main class="page">
	<center-l>
		<stack-l>
			<SiteMark />
			<h1>{title}</h1>
			{@render children()}
			<p class="signature">Mark<br /><span>{`Building ${PRODUCT_NAME}`}</span></p>
		</stack-l>
	</center-l>
</main>

<style>
	.page {
		padding-block: var(--space-10) var(--space-12);
		padding-inline: var(--page-gutter);
	}

	h1 {
		font-size: var(--text-display-size);
		font-weight: var(--text-display-weight);
		line-height: var(--text-display-leading);
		letter-spacing: var(--text-display-tracking);
	}

	.signature {
		font-weight: var(--text-label-weight);
	}

	.signature span {
		color: var(--color-on-surface-variant);
		font-weight: normal;
	}
</style>
