<script lang="ts">
	/*
	 * The signed-in person's own boundary (#2040). `(person)/+layout.svelte`
	 * still renders above this -- it has no `load` of its own, so it cannot
	 * itself fail -- so a refusal here keeps the signed-in bar, the avatar
	 * and the way back to her Practices.
	 *
	 * The case that asked for it: a signed-in Owner who is not the founder
	 * opens `/feedback`. The founder read page refuses her with a 404 on
	 * purpose (`founderRead.ts`), so its existence is not revealed, and
	 * before this boundary the root one caught it: the signed-out bar and
	 * "Sign in" for a person who is signed in. The 404 stays; only the way
	 * out changes, to her own account, which every signed-in person has.
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
