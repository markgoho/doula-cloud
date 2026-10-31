<script lang="ts">
	// app/'s stylesheet, whole, by relative import as site/ takes it: one
	// set of tokens for the product (ADR-0062).
	import '../../../app/src/lib/styles/app.css';

	let { children } = $props();
</script>

{@render children()}
