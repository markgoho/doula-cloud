<script lang="ts">
	/*
	 * The catch-all: a URL that matches no route at all (#471). SvelteKit
	 * uses the root `+error.svelte` for this case (see "Routing#+error" in
	 * the SvelteKit docs) with no layout above it to supply chrome -- a
	 * route that never matched has no `[practiceId]`/`[engagementId]` to
	 * read either. This is the one boundary that renders its own bar rather
	 * than inheriting one, and it renders the signed-out bar regardless of
	 * session: recovering the correct staff/portal chrome here would mean
	 * duplicating each layout's own session fetch for a URL nobody actually
	 * typed to reach a real page.
	 */
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import SignedOutShell from './SignedOutShell.svelte';
	import ErrorPage from '#lib/components/templates/ErrorPage.svelte';
	import { errorKindForStatus } from '#lib/errorPage.js';

	/*
	 * No `portal/+error.svelte` exists, so a Portal 404 or 500 renders this
	 * boundary with no Portal layout above it. The address alone says it is
	 * a Portal screen, and every Portal screen carries the Portal's footer
	 * (#1558). `/portal` itself or below it, never a sibling like `/portalx`.
	 */
	const isPortalPath = $derived(
		page.url.pathname === '/portal' || page.url.pathname.startsWith('/portal/')
	);
</script>

<SignedOutShell isPortalFooterShown={isPortalPath}>
	<ErrorPage
		kind={errorKindForStatus(page.status, page.error?.code)}
		wayOutHref={resolve('/(signed-out)/login')}
		wayOutLabel="Sign in"
	/>
</SignedOutShell>
