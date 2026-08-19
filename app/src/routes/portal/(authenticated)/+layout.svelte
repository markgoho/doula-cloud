<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { apiFetch } from '#lib/api.js';
	import { unregisterPushSubscription } from '#lib/pushRegistration.js';
	import { signOutOfSession, type SignOutOutcome } from '#lib/signOut.js';
	import SignOutButton from '#lib/components/molecules/SignOutButton.svelte';

	let { children } = $props();

	async function handleSignOut(): Promise<SignOutOutcome> {
		const outcome = await signOutOfSession({
			// Unconditional, unlike the Staff layout's: every authenticated
			// portal screen sits under engagements/[engagementId], so the
			// Engagement-scoped unregister endpoint always has its scope off
			// the route -- there is no scope-less screen to guard against.
			unsubscribeURL: `/api/portal/engagements/${page.params.engagementId}/push-subscriptions`,
			// apiFetch, not apiFetchWithSession: that helper's 401 handling
			// would navigate to the login screen on a failure sign-out is
			// supposed to report in place, and an end-session request that has
			// already cleared the cookie must not read as an expired session.
			fetcher: apiFetch,
			unregisterPush: unregisterPushSubscription
		});
		// The portal login screen, not the Staff one: a Client sent to
		// /login would be shown a door that is not theirs (#153).
		if (outcome.ok) await goto(resolve('/portal/login'));
		return outcome;
	}
</script>

<header>
	<SignOutButton signOut={handleSignOut} />
</header>

{@render children()}

<style>
	@layer components {
		header {
			display: flex;
			justify-content: flex-end;
			padding: var(--space-2) var(--space-4);
		}
	}
</style>
