<!--
@component
One headed section of a ReadingPage: an `h2` and the paragraphs under it,
spaced by a default `stack-l` so the heading sits closer to its own text
than to the section before it.
-->
<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Properties {
		heading: string;
		children: Snippet;
	}

	let { heading, children }: Properties = $props();
</script>

<section>
	<stack-l>
		<h2>{heading}</h2>
		{@render children()}
	</stack-l>
</section>

<style>
	h2 {
		font-size: var(--text-heading-size);
		font-weight: var(--text-heading-weight);
		line-height: var(--text-heading-leading);
		letter-spacing: var(--text-heading-tracking);
	}
</style>
