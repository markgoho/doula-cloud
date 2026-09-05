<script lang="ts">
	/*
	 * Where a signed-in identity that belongs to neither population lands
	 * (#745).
	 *
	 * Two different people arrive here and they are in the same state:
	 * someone whose signup half-landed -- the Identity Platform account
	 * created, `POST /api/staff/signup` refused -- and a Staff member whose
	 * last Membership was removed. Both hold a live session that resolves
	 * to no Practice, and before this screen both were shown
	 * `no matching staff account`, which names an internal lookup and
	 * offers no way forward. ADR-0021's rule for a screen that reports a
	 * failure is that it says what happened and what to do next, so this
	 * one names the state and gives the two real routes out of it: start a
	 * Practice, or wait to be invited to one.
	 *
	 * It composes EntryPage, Link and SignOutButton and writes no CSS of
	 * its own -- the CLAUDE.md block on new components while #518 is open
	 * exempts exactly that.
	 */
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { apiFetch } from '#lib/api.js';
	import { unregisterPushSubscription } from '#lib/pushRegistration.js';
	import { signOutOfSession, type SignOutOutcome } from '#lib/signOut.js';
	import { decideLanding, type SessionInfo } from '#lib/landing.js';
	import Link from '#lib/components/atoms/Link.svelte';
	import SignOutButton from '#lib/components/molecules/SignOutButton.svelte';
	import EntryPage from '#lib/components/templates/EntryPage.svelte';

	/*
	 * This screen makes a claim about the reader's account, so it checks
	 * the claim rather than trusting whoever sent her here. It reads the
	 * staff session directly instead of through `probeSession`, which
	 * collapses "no staff row" and "no session at all" into one
	 * `undefined` -- and those two are different people: the first belongs
	 * here, the second belongs on the login screen.
	 */
	onMount(async () => {
		let response: Response;
		try {
			response = await apiFetch('/api/staff/session');
		} catch {
			// Offline, or a rewrite miss. The screen below is still true of
			// whoever was sent here, so it stays.
			return;
		}
		if (response.status === 401) {
			await goto(resolve('/(signed-out)/login'));
			return;
		}
		if (!response.ok) return;

		const session: SessionInfo = await response.json();
		// She has a Practice after all -- a stale tab, or a bookmark kept
		// after an invitation was accepted. Asked through `decideLanding`,
		// which owns this question for every screen that asks it, and
		// answered by `/`, which owns where a person with one goes.
		if (decideLanding(session).type !== 'no-practice') await goto(resolve('/'));
	});

	async function handleSignOut(): Promise<SignOutOutcome> {
		const outcome = await signOutOfSession({
			// No Practice, so no push subscription scope to unregister --
			// same as /account's own layout.
			unsubscribeURL: undefined,
			fetcher: apiFetch,
			unregisterPush: unregisterPushSubscription
		});
		if (outcome.ok) await goto(resolve('/(signed-out)/login'));
		return outcome;
	}
</script>

{#snippet content()}
	<p>
		You are signed in, but this account does not belong to a Practice yet. That happens when
		setting up a Practice stopped part-way through, or when your last Practice removed you.
	</p>
	<p>There are two ways on from here.</p>
	<ul>
		<li>
			<Link href={resolve('/(signed-out)/signup')} label="Set up a Practice" /> — you keep this email
			address and password, and this account becomes its Owner.
		</li>
		<li>
			Wait for an invitation. An Owner at an existing Practice can invite this email address, and
			the invitation arrives by email.
		</li>
	</ul>

	<SignOutButton signOut={handleSignOut} />
{/snippet}

<EntryPage title="Your account is not part of a Practice" {content} />
