<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { apiFetch, apiFetchWithSession } from '#lib/api.js';
	import {
		practicePushSubscriptionsPath,
		unregisterPushSubscription
	} from '#lib/pushRegistration.js';
	import { signOutOfSession, type SignOutOutcome } from '#lib/signOut.js';
	import Link from '#lib/components/atoms/Link.svelte';
	import type { PracticeOption } from '#lib/components/molecules/PracticeSwitcher.svelte';
	import StaffTopBar, { type NavItem } from '#lib/components/organisms/StaffTopBar.svelte';

	let { children } = $props();

	type Membership = { practiceId: string; practiceName: string; roles: string[] };
	type StaffSession = { name: string; email: string; memberships: Membership[] };

	let session = $state<StaffSession | undefined>();
	let roles = $state<string[]>([]);

	const practiceId = $derived(page.params.practiceId);

	onMount(async () => {
		// Who am I: the name and email the avatar menu shows, and the
		// Memberships the Practice switcher is built from. Free -- this is
		// the same endpoint login already calls to decide where to land.
		const sessionResponse = await apiFetchWithSession('/api/staff/session');
		if (sessionResponse.ok) session = await sessionResponse.json();

		if (practiceId === undefined) return;

		// What I am *here*. Separate from the above because a role is held at
		// a Practice, not by a person: the same account is an Owner at one
		// agency and a contractor Doula at the next.
		const rolesResponse = await apiFetchWithSession(`/api/practices/${practiceId}/session`);
		if (!rolesResponse.ok) return;
		const body: { roles: string[] } = await rolesResponse.json();
		roles = body.roles;
	});

	/*
	 * The drawing (#431) shows an Owner's bar, with all its sections --
	 * seven since #265 added Invoices. A Doula's is four: `GET
	 * .../invoices`, `GET .../billing` and `GET .../staff` are all
	 * `ownerAndAdmin` on the BFF, so offering her those three would be a
	 * promise the endpoint refuses. Same rule #423 applied to the landing
	 * page's rail -- ask only for what the caller's role can be served.
	 */
	const isAdmin = $derived(roles.includes('owner') || roles.includes('admin'));

	const navItems = $derived.by((): NavItem[] => {
		if (practiceId === undefined) return [];
		const path = page.url.pathname;
		const overview = resolve('/practices/[practiceId]', { practiceId });
		const items = [
			{ label: 'Overview', href: overview, current: path === overview },
			{ label: 'Clients', href: resolve('/practices/[practiceId]/clients', { practiceId }) },
			...(isAdmin
				? [
						// Invoices is the Practice's money in, Billing its money
						// out (#265) -- two sections rather than one, because
						// "what have our Clients not paid us" and "how many
						// credits do we have left" are not the same question.
						// Both endpoints are `ownerAndAdmin`, so both sit behind
						// the same flag.
						{ label: 'Invoices', href: resolve('/practices/[practiceId]/invoices', { practiceId }) },
						{ label: 'Billing', href: resolve('/practices/[practiceId]/billing', { practiceId }) },
						{ label: 'Staff', href: resolve('/practices/[practiceId]/staff', { practiceId }) }
					]
				: []),
			{ label: 'Offers', href: resolve('/practices/[practiceId]/offers', { practiceId }) },
			{ label: 'Settings', href: resolve('/practices/[practiceId]/settings', { practiceId }) }
		];
		// Overview is an exact match and every other section is a prefix, so
		// a Client's own screen still marks Clients as the current section.
		return items.map((item) => ({
			...item,
			current: item.current ?? path.startsWith(item.href)
		}));
	});

	const practices = $derived.by((): PracticeOption[] =>
		(session?.memberships ?? []).map((membership) => ({
			...membership,
			href: resolve('/practices/[practiceId]', { practiceId: membership.practiceId })
		}))
	);

	function pushUnsubscribeURL(): string | undefined {
		return practiceId === undefined ? undefined : practicePushSubscriptionsPath(practiceId);
	}

	async function handleSignOut(): Promise<SignOutOutcome> {
		const outcome = await signOutOfSession({
			// Every authenticated Staff screen sits under
			// practices/[practiceId], so the push unregister endpoint gets its
			// scope straight off the route; a screen without one skips the
			// unregister rather than blocking sign-out (#152).
			unsubscribeURL: pushUnsubscribeURL(),
			// apiFetch, not apiFetchWithSession: that helper's 401 handling
			// would navigate to the login screen on a failure sign-out is
			// supposed to report in place, and an end-session request that has
			// already cleared the cookie must not read as an expired session.
			fetcher: apiFetch,
			unregisterPush: unregisterPushSubscription
		});
		if (outcome.ok) await goto(resolve('/(signed-out)/login'));
		return outcome;
	}
</script>

<Link href="#main" label="Skip to main content" variant="skip" />
<!--
	Rendered before the session lands, not after: the bar is a fixed 60px
	whatever it holds, so painting it immediately means the page below never
	moves. The parts that need an answer from the BFF -- the person's
	avatar, the Practice switcher, and the three admin-only nav items --
	arrive into a bar that is already there.
-->
<StaffTopBar
	{navItems}
	{practices}
	currentPracticeId={practiceId ?? ''}
	name={session?.name ?? ''}
	email={session?.email}
	accountHref={resolve('/account')}
	signOut={handleSignOut}
/>
<main id="main" tabindex="-1">
	{@render children()}
</main>
