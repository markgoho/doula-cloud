<script lang="ts">
	import '../../../app/src/lib/styles/app.css';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import ServiceNavigation from '#lib/components/ServiceNavigation.svelte';
	import type { Operator } from '#lib/session.js';

	let { children } = $props();

	const operator = $derived(page.data.operator as Operator | undefined);
</script>

<ServiceNavigation {operator} pathname={page.url.pathname} onsignedout={() => goto(resolve('/signed-out'))} />
<main id="main">
	<stack-l>
		{@render children()}
	</stack-l>
</main>

<style>
	main {
		padding: var(--space-6) var(--space-4);
	}

	main :global(p) {
		max-inline-size: var(--measure);
	}

	@layer base {
		:global(a) {
			color: var(--color-primary);
		}

		:global(a:hover) {
			color: var(--color-primary-hover);
		}
	}
</style>
