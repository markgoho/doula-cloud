<script lang="ts">
	import { onMount } from 'svelte';
	import favicon from '#lib/assets/favicon.svg';
	import { registerLayoutPrimitives } from '#lib/primitives/index.js';
	import RouteAnnouncer from '#lib/components/RouteAnnouncer.svelte';
	import '#lib/styles/app.css';

	import { beforeNavigate } from '$app/navigation';
	import { updated } from '$app/state';
	import { skewReloadHref } from '#lib/versionSkew.js';

	let { children } = $props();

	beforeNavigate((navigation) => {
		const href = skewReloadHref(updated.current, navigation);
		if (href) location.assign(href);
	});

	onMount(() => {
		registerLayoutPrimitives();
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

{@render children()}

<RouteAnnouncer />
