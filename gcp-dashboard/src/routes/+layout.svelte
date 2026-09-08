<script lang="ts">
	import favicon from '#lib/assets/favicon.svg';

	let { children } = $props();
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

{@render children()}

<style>
	/* Declaring both schemes stops Chrome force-darkening a page that never
	   said which themes it supports -- its auto inversion tinted the cost
	   bars red. With this, the system colors this project styles with resolve
	   per theme instead. */
	:global(html) {
		color-scheme: light dark;
	}

	:global(body) {
		background: Canvas;
		color: CanvasText;
		font-family: system-ui, sans-serif;
		margin: 0;
	}
</style>
