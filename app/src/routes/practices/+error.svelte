<script lang="ts">
	/*
	 * The Staff chrome (`practices/+layout.svelte`) still renders above this
	 * -- it has no `load` of its own, so it cannot itself fail (#471). The
	 * way out is the Practice overview: `page.params.practiceId` survives a
	 * failed `load`, because routing matched before the load ran.
	 */
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import ErrorPage from '#lib/components/templates/ErrorPage.svelte';
	import { errorKindForStatus } from '#lib/errorPage.js';

	const practiceId = $derived(page.params.practiceId!);
</script>

<ErrorPage
	kind={errorKindForStatus(page.status)}
	wayOutHref={resolve('/practices/[practiceId]', { practiceId })}
	wayOutLabel="Go to your Practice overview"
/>
