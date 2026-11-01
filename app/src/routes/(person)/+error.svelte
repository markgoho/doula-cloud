<script lang="ts">
	/*
	 * The signed-in person's own boundary (#2040). `(person)/+layout.svelte`
	 * still renders above this -- it has no `load` of its own, so it cannot
	 * itself fail -- so a refusal here keeps the signed-in bar, the avatar
	 * and the way back to her Practices.
	 *
	 * Without it the root boundary catches a refusal: the signed-out bar
	 * and "Sign in" for a person who is signed in. The way out is her own
	 * account, which every signed-in person has.
	 */
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import ErrorPage from '#lib/components/templates/ErrorPage.svelte';
	import { errorKindForStatus } from '#lib/errorPage.js';
</script>

<ErrorPage
	kind={errorKindForStatus(page.status, page.error?.code)}
	wayOutHref={resolve('/(person)/account')}
	wayOutLabel="Go to your account"
/>
