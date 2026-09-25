<script lang="ts">
	// The app's stylesheet, whole: the same fonts, reset, tokens (both
	// themes), base and layout primitives, by import rather than by a copy
	// of the palette that has to be kept in step by hand -- which is what
	// the Hugo layouts carried, because Hugo had no asset pipeline.
	import '../../../app/src/lib/styles/app.css';

	let { children } = $props();
</script>

{@render children()}
