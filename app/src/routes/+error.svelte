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
	import Link from '#lib/components/atoms/Link.svelte';
	import SignedOutTopBar from '#lib/components/organisms/SignedOutTopBar.svelte';
	import ErrorPage from '#lib/components/templates/ErrorPage.svelte';
	import { errorKindForStatus } from '#lib/errorPage.js';
</script>

<Link href="#main" label="Skip to main content" variant="skip" />
<SignedOutTopBar />
<main id="main" tabindex="-1">
	<ErrorPage
		kind={errorKindForStatus(page.status)}
		wayOutHref={resolve('/(signed-out)/login')}
		wayOutLabel="Log in"
	/>
</main>
