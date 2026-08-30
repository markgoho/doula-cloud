<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { apiFetch } from '#lib/api.js';
	import {
		portalPushSubscriptionsPath,
		registerPushSubscription,
		unregisterPushSubscription
	} from '#lib/pushRegistration.js';
	import { apiBaseURL } from '#lib/api.js';
	import { signOutOfSession, type SignOutOutcome } from '#lib/signOut.js';
	import Link from '#lib/components/atoms/Link.svelte';
	import PortalTopBar from '#lib/components/organisms/PortalTopBar.svelte';
	import type { NavItem } from '#lib/components/organisms/StaffTopBar.svelte';

	let { children } = $props();

	/*
	 * The Practice's name is the portal's identity (#431), so the bar used
	 * to wait on an onMount fetch to draw it -- the same flash #487 exists
	 * to remove from the tab title. `engagements/[engagementId]/+layout.ts`
	 * now loads it before first paint, so it is read here instead.
	 */
	const detail = $derived(page.data as { practiceName?: string; clientName?: string });

	const engagementId = $derived(page.params.engagementId!);

	onMount(() => {
		// Push registration lives here rather than on the hub page (#61's
		// "once per device after login"). It used to sit on the hub, which
		// stopped being the only authenticated portal screen the moment
		// Messages became its own route -- a person who lands straight on
		// her Contract should still be registered.
		//
		// Fire-and-forget, and a plain credentialed fetch rather than
		// apiFetchWithSession: that helper's own 401 handling would sign the
		// person out on a failure this call is supposed to swallow silently.
		void registerPushSubscription(portalPushSubscriptionsPath(engagementId), (path, init) =>
			fetch(apiBaseURL() + path, { ...init, credentials: 'include' })
		);
	});

	const navItems = $derived.by((): NavItem[] => {
		const path = page.url.pathname;
		const hub = resolve('/portal/(authenticated)/engagements/[engagementId]', { engagementId });
		const items = [
			{ label: 'Your care', href: hub, current: path === hub },
			{
				label: 'Messages',
				href: resolve('/portal/(authenticated)/engagements/[engagementId]/messages', {
					engagementId
				})
			},
			{
				label: 'Birth plan',
				href: resolve('/portal/(authenticated)/engagements/[engagementId]/birth-plan', {
					engagementId
				})
			},
			{
				label: 'Contract',
				href: resolve('/portal/(authenticated)/engagements/[engagementId]/contract', {
					engagementId
				})
			}
		];
		return items.map((item) => ({ ...item, current: item.current ?? path.startsWith(item.href) }));
	});

	async function handleSignOut(): Promise<SignOutOutcome> {
		const outcome = await signOutOfSession({
			// Unconditional, unlike the Staff layout's: every authenticated
			// portal screen sits under engagements/[engagementId], so the
			// Engagement-scoped unregister endpoint always has its scope off
			// the route -- there is no scope-less screen to guard against.
			unsubscribeURL: portalPushSubscriptionsPath(engagementId),
			// apiFetch, not apiFetchWithSession: that helper's 401 handling
			// would navigate to the login screen on a failure sign-out is
			// supposed to report in place, and an end-session request that has
			// already cleared the cookie must not read as an expired session.
			fetcher: apiFetch,
			unregisterPush: unregisterPushSubscription
		});
		// The portal login screen, not the Staff one: a Client sent to
		// /login would be shown a door that is not theirs (#153).
		if (outcome.ok) await goto(resolve('/portal/(signed-out)/login'));
		return outcome;
	}
</script>

<Link href="#main" label="Skip to main content" variant="skip" />
<PortalTopBar
	practiceName={detail?.practiceName ?? ''}
	{navItems}
	name={detail?.clientName ?? ''}
	signOut={handleSignOut}
/>
<main id="main" tabindex="-1">
	{@render children()}
</main>
