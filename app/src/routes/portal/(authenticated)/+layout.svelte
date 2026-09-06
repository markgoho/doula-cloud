<script lang="ts">
	import { onMount } from 'svelte';
	import { goto, invalidateAll } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { apiFetch } from '#lib/api.js';
	import {
		portalNotificationPreferencePath,
		portalPushSubscriptionsPath,
		registerPushSubscriptionIfEnabled,
		unregisterPushSubscription
	} from '#lib/pushRegistration.js';
	import { apiBaseURL } from '#lib/api.js';
	import { signOutOfSession, type SignOutOutcome } from '#lib/signOut.js';
	import Link from '#lib/components/atoms/Link.svelte';
	import PortalTopBar from '#lib/components/organisms/PortalTopBar.svelte';
	import type { NavItem } from '#lib/components/organisms/StaffTopBar.svelte';

	let { children } = $props();

	// Plain credentialed fetch rather than apiFetchWithSession: that
	// helper's own 401 handling would sign the person out on a failure the
	// best-effort push-registration call below is supposed to swallow
	// silently.
	function credentialedFetch(path: string, init?: RequestInit) {
		return fetch(apiBaseURL() + path, { ...init, credentials: 'include' });
	}

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
		// registerPushSubscriptionIfEnabled (#303) consults the durable
		// preference first: a Client who has never visited the notification
		// settings screen, or who has turned push off there, must never see
		// the browser's own permission prompt fire on a bare page load --
		// only that screen's explicit "Turn on" action does that.
		//
		// Fire-and-forget -- see credentialedFetch's own doc comment above.
		void registerPushSubscriptionIfEnabled(
			portalNotificationPreferencePath(engagementId),
			portalPushSubscriptionsPath(engagementId),
			credentialedFetch
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
			},
			{
				label: 'Notifications',
				href: resolve('/portal/(authenticated)/engagements/[engagementId]/notifications', {
					engagementId
				})
			},
			// #619. In the bar rather than tucked behind an avatar menu:
			// this is the only screen a Client has for anything about her
			// own account, and a screen nobody can reach is a screen that
			// does not exist.
			{
				label: 'Sign-in address',
				href: resolve('/portal/(authenticated)/engagements/[engagementId]/sign-in-address', {
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
		if (outcome.ok) {
			// engagements/[engagementId]/+layout.ts's load result is keyed on
			// params alone (#487), so pressing Back to the exact URL it last
			// resolved for would otherwise reuse that stale, still-signed-in
			// data instead of re-checking the session -- SvelteKit skips a
			// rerun when nothing it tracks has changed, and signing out
			// changes nothing it tracks. `invalidateAll` marks it (and every
			// other active load) stale, so the next visit -- Back included --
			// re-fetches and hits the same 401 the page's own reads already
			// bounce on.
			await invalidateAll();
			await goto(resolve('/portal/(signed-out)/login'));
		}
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
