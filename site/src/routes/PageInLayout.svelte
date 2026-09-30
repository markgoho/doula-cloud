<!--
A spec harness, not a route: SvelteKit reads only the `+` files in this
tree. A page spec renders the page component on its own, but the site
footer and its `contentinfo` landmark come from +layout.svelte (#1556), so
a spec that asserts what a reader of the whole page gets renders the page
inside the layout through this.
-->
<script lang="ts">
	import type { Component } from 'svelte';
	import Layout from './+layout.svelte';

	interface Properties {
		page: Component<Record<string, unknown>>;
		pageProperties?: Record<string, unknown>;
	}

	let { page: Page, pageProperties = {} }: Properties = $props();
</script>

<Layout>
	<Page {...pageProperties} />
</Layout>
