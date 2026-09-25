<!--
@component
One page of prose on the site: a titled column capped at a reading
measure, and a footer that says when the page last changed.

Every page on the site is this today -- /pilot-terms and each Practice
page at /p/<slug> -- which is what the Hugo site's two layouts each
wrote out in full.

The column is `center-l` and `stack-l` from app/'s own layout primitives
(ADR-0003), used in their default state, which is pure CSS: this site
ships no JavaScript, and a primitive left at its defaults needs none. No
media query anywhere: the measure is what makes it a column on a wide
screen and the gutter is what makes it fit a 320px one (ADR-0024).
-->
<script lang="ts">
	import type { Snippet } from 'svelte';
	import { formatUpdated } from '#lib/practicePage.js';

	interface Properties {
		// The page's one `h1`.
		title: string;
		// When the page last changed, RFC 3339 or a bare date.
		updated: string;
		// Printed after the date, inside the same footer line.
		footnote?: Snippet;
		// Extra attributes on `<main>`, such as the #443 probe marker.
		mainAttributes?: Record<string, string>;
		children: Snippet;
	}

	let { title, updated, footnote, mainAttributes = {}, children }: Properties = $props();

	const date = $derived(formatUpdated(updated));
</script>

<div class="page">
	<center-l>
		<stack-l class="sections">
			<main {...mainAttributes}>
				<stack-l class="sections">
					<h1>{title}</h1>
					{@render children()}
				</stack-l>
			</main>

			<footer>
				<p>
					Last updated <time datetime={date.datetime}>{date.text}</time>.
					{#if footnote}{@render footnote()}{/if}
				</p>
			</footer>
		</stack-l>
	</center-l>
</div>

<style>
	.page {
		padding-block: var(--space-8) var(--space-12);
		padding-inline: var(--page-gutter);
	}

	/* Between the page's sections: wider than the default stack gap, which
	   stays between the paragraphs inside one section. */
	.sections {
		gap: var(--space-8);
	}

	h1 {
		font-size: var(--text-display-size);
		font-weight: var(--text-display-weight);
		line-height: var(--text-display-leading);
		letter-spacing: var(--text-display-tracking);
	}

	footer {
		padding-block-start: var(--space-4);
		border-block-start: var(--border-thin) solid var(--color-outline-variant);
		color: var(--color-on-surface-muted);
		font-size: var(--text-meta-size);
		line-height: var(--text-meta-leading);
		letter-spacing: var(--text-meta-tracking);
	}

	.page :global(a) {
		color: var(--color-primary);
	}

	.page :global(a:focus-visible) {
		outline: var(--focus-ring-width) solid var(--color-primary);
		outline-offset: var(--focus-ring-offset);
	}
</style>
