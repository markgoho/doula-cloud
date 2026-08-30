<script lang="ts">
	/*
	 * The signed-out Staff chrome (`(signed-out)/+layout.svelte`) still
	 * renders above this, for the same reason `practices/+error.svelte`
	 * does (#471). Nobody here has a session yet, so the only way out is in.
	 */
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import ErrorPage from '#lib/components/templates/ErrorPage.svelte';
	import { errorKindForStatus } from '#lib/errorPage.js';
</script>

<ErrorPage
	kind={errorKindForStatus(page.status)}
	wayOutHref={resolve('/(signed-out)/login')}
	wayOutLabel="Log in"
/>
