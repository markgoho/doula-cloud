<script lang="ts">
	/*
	 * The signed-out Staff chrome (`(signed-out)/+layout.svelte`) still
	 * renders above this, for the same reason `practices/+error.svelte`
	 * does (#471). Most people here have no session yet, so the only way
	 * out is in.
	 *
	 * The three screens in this group that a live session can reach --
	 * `/`, `/no-practice` and `mfa/enroll` -- are the exception, and "Log
	 * in" is still the right way out for each: `/` and `/no-practice` have
	 * nowhere else to send a session that has chosen no Practice, and a
	 * person whose enrollment broke has to start the sign-in over anyway,
	 * since this app signs the JS SDK out at every exit from that screen
	 * (see its own #167 comment).
	 */
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import ErrorPage from '#lib/components/templates/ErrorPage.svelte';
	import { errorKindForStatus } from '#lib/errorPage.js';
</script>

<ErrorPage
	kind={errorKindForStatus(page.status, page.error?.code)}
	wayOutHref={resolve('/(signed-out)/login')}
	wayOutLabel="Log in"
/>
