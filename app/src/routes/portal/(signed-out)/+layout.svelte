<script lang="ts">
	import Link from '#lib/components/atoms/Link.svelte';
	import SignedOutTopBar from '#lib/components/organisms/SignedOutTopBar.svelte';

	/*
	 * The Client portal's own signed-out chrome: /portal/login and
	 * /portal/accept-invite. A second group rather than a shared one,
	 * because SvelteKit layouts nest rather than replace and
	 * `portal/(authenticated)` has to be able to render different chrome at
	 * the same depth (#431).
	 *
	 * The bar says `Doula Cloud` here and the Practice's name on every
	 * signed-in portal screen. That is not an inconsistency to fix: with no
	 * session the app does not yet know which Practice this person belongs
	 * to.
	 */
	let { children } = $props();
</script>

<Link href="#main" label="Skip to main content" variant="skip" />
<SignedOutTopBar />
<main id="main" tabindex="-1">
	{@render children()}
</main>
