<script lang="ts">
	// The app's stylesheet, whole: the same fonts, reset, tokens (both
	// themes), base and layout primitives, by import rather than by a copy
	// of the palette that has to be kept in step by hand -- which is what
	// the Hugo layouts carried, because Hugo had no asset pipeline.
	import '../../../app/src/lib/styles/app.css';
	import SiteFooter from '#lib/components/SiteFooter.svelte';

	let { children } = $props();
</script>

{@render children()}

<!-- Every page's one contentinfo landmark, with the links to the Terms of
     Service and the Privacy Policy (#1556). -->
<SiteFooter />
