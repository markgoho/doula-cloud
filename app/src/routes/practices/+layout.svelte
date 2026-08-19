<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { apiBaseURL } from '#lib/api.js';
	import { unregisterPushSubscription } from '#lib/pushRegistration.js';
	import { signOutOfSession, type SignOutOutcome } from '#lib/signOut.js';
	import SignOutButton from '#lib/components/molecules/SignOutButton.svelte';

	let { children } = $props();

	async function handleSignOut(): Promise<SignOutOutcome> {
		const outcome = await signOutOfSession({
			// Every authenticated Staff screen sits under
			// practices/[practiceId], so the push unregister endpoint gets its
			// scope straight off the route; a screen without one skips the
			// unregister rather than blocking sign-out (#152).
			practiceId: page.params.practiceId,
			// A plain credentialed fetch, not apiFetchWithSession: that
			// helper's 401 handling would navigate to the login screen on a
			// failure sign-out is supposed to report in place, and an
			// end-session request that has already cleared the cookie must not
			// read as an expired session.
			fetcher: (path, init) => fetch(apiBaseURL() + path, { ...init, credentials: 'include' }),
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
