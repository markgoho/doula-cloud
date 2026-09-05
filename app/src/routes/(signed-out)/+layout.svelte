<script lang="ts">
	import Link from '#lib/components/atoms/Link.svelte';
	import SignedOutTopBar from '#lib/components/organisms/SignedOutTopBar.svelte';

	/*
	 * The chrome for archetype A: login, signup, accept-invite and the
	 * pre-account Offer read -- the screens a person meets before there is
	 * an account -- plus `/no-practice` (#745), which is the one screen
	 * here that needs a session and shows the same reduced bar because
	 * that session belongs to no Practice to put a name to.
	 *
	 * It is a route group rather than the root layout because SvelteKit
	 * layouts nest rather than replace -- a reduced bar in `routes/+layout`
	 * would render above the Staff bar too (#431).
	 *
	 * No gutters and no max-width here: those belong to the Template the
	 * page instantiates (ADR-0018).
	 */
	let { children } = $props();
</script>

<Link href="#main" label="Skip to main content" variant="skip" />
<SignedOutTopBar />
<main id="main" tabindex="-1">
	{@render children()}
</main>
