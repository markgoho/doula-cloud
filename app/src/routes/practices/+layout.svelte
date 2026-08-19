<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { apiFetch } from '#lib/api.js';
	import {
		practicePushSubscriptionsPath,
		unregisterPushSubscription
	} from '#lib/pushRegistration.js';
	import { signOutOfSession, type SignOutOutcome } from '#lib/signOut.js';
	import SignOutButton from '#lib/components/molecules/SignOutButton.svelte';

	let { children } = $props();

	function pushUnsubscribeURL(): string | undefined {
		const practiceId = page.params.practiceId;
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
		if (outcome.ok) await goto(resolve('/login'));
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
