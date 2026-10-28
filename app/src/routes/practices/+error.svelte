<script lang="ts">
	/*
	 * The Staff chrome (`practices/+layout.svelte`) still renders above this
	 * -- it has no `load` of its own, so it cannot itself fail (#471). The
	 * way out is the Practice overview: `page.params.practiceId` survives a
	 * failed `load`, because routing matched before the load ran.
	 */
	import { page } from '$app/state';
	import { errorKindForStatus } from '#lib/errorPage.js';
	import FailedRead from './[practiceId]/FailedRead.svelte';
</script>

<FailedRead kind={errorKindForStatus(page.status, page.error?.code)} />
