<script lang="ts">
	import { onMount } from 'svelte';
	import favicon from '#lib/assets/favicon.svg';
	import { registerLayoutPrimitives } from '#lib/primitives/index.js';
	import '#lib/styles/app.css';

	let { children } = $props();

	onMount(() => {
		registerLayoutPrimitives();
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

{@render children()}
