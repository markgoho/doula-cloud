<script lang="ts">
	import Link from '#lib/components/atoms/Link.svelte';
	import { page } from '#lib/appState.svelte.js';
	import PortalFooter from '#lib/components/organisms/PortalFooter.svelte';
	import SignedOutTopBar from '#lib/components/organisms/SignedOutTopBar.svelte';
	import type { RootLanding } from './+page.js';

	/*
	 * The chrome for archetype A: login, signup, accept-invite and the
	 * pre-account Offer read -- the screens a person meets before there is
	 * an account -- plus the three screens here that a session can reach
	 * and that still show the same reduced bar, because that session
	 * belongs to no Practice to put a name to: `/no-practice` (#745), the
	 * wayfinding root `/` (#678), whose Staff and portal pickers are a
	 * session that has not chosen a Practice or an Engagement yet, and
	 * TOTP enrollment at `mfa/enroll` (#1114). See `+page.svelte` for why
	 * `/` takes this shell rather than a group of its own.
	 *
	 * `mfa/enroll` is the newest of the three and the least obvious, so
	 * the reason is worth stating: the person on it holds a live Staff
	 * session that has not cleared MFA, carrying a `returnTo` to the
	 * Practice-scoped page that refused her. Until she enrolls, every
	 * destination `StaffTopBar` offers is one she cannot reach, and there
	 * is no chosen Practice for that bar to name -- which is the same
	 * position `/no-practice` and `/` are in, and so the same reduced bar
	 * is the true one. A `mfa/+layout.svelte` of its own was the
	 * alternative and was rejected: it would be a third hand-written copy
	 * of skip link, bar and `<main>` for a single route, when the group
	 * that already means exactly this was one directory move away.
	 *
	 * It is a route group rather than the root layout because SvelteKit
	 * layouts nest rather than replace -- a reduced bar in `routes/+layout`
	 * would render above the Staff bar too (#431).
	 *
	 * No gutters and no max-width here: those belong to the Template the
	 * page instantiates (ADR-0018).
	 */
	let { children } = $props();

	/*
	 * `/`'s portal picker is a Portal screen: a signed-in Client choosing an
	 * Engagement, or told she has none. So it carries the Portal's own
	 * footer, in the same place as every other Portal screen (#1558). No
	 * other screen in this group is read by a Client, and `/` only knows
	 * which population is reading once its load resolves. Narrowed through
	 * `RootLanding`, so a renamed variant there fails the type check here.
	 */
	function isPortalPicker(
		data: Record<string, unknown>
	): data is Extract<RootLanding, { type: 'portal-picker' }> {
		return data.type === ('portal-picker' satisfies RootLanding['type']);
	}

	const isPortalScreen = $derived(isPortalPicker(page.data));
</script>

<Link href="#main" label="Skip to main content" variant="skip" />
<SignedOutTopBar />
<main id="main" tabindex="-1">
	{@render children()}
</main>
{#if isPortalScreen}
	<PortalFooter />
{/if}
