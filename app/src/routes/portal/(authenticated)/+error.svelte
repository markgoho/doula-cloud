<script lang="ts">
	/*
	 * The Portal chrome (`portal/(authenticated)/+layout.svelte`) still
	 * renders above this, for the same reason `practices/+error.svelte`
	 * does (#471). The way out is the Client's own hub.
	 */
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import ErrorPage from '#lib/components/templates/ErrorPage.svelte';
	import { errorKindForStatus } from '#lib/errorPage.js';

	const engagementId = $derived(page.params.engagementId!);
</script>

<ErrorPage
	kind={errorKindForStatus(page.status)}
	wayOutHref={resolve('/portal/(authenticated)/engagements/[engagementId]', { engagementId })}
	wayOutLabel="Go to your care"
/>
