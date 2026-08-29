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
	import SignOutButton from '#lib/components/molecules/SignOutButton.svelte';
	import Link from '#lib/components/atoms/Link.svelte';

	let { children } = $props();

	/*
	 * Temporary chrome. Until #452 builds the real Staff shell, these are
	 * the links the Practice landing page used to carry itself -- moved up
	 * here because navigation is chrome, and ADR-0018 puts all chrome on
	 * the layout. #423 needed them off the page: a menu of administration
	 * is precisely what made that page the abandon point in
	 * `docs/journeys/evaluator-doula.md`.
	 *
	 * Same links and the same Owner gate as before, so nothing a person is
	 * mid-journey through moves. The shell replaces this whole header.
	 */
	let roles = $state<string[]>([]);
	let isOwner = $derived(roles.includes('owner'));

	onMount(async () => {
		const practiceId = page.params.practiceId;
		if (practiceId === undefined) return;

		const response = await apiFetchWithSession(`/api/practices/${practiceId}/session`);
		if (!response.ok) return;

		const body: { roles: string[] } = await response.json();
		roles = body.roles;
	});

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
	{#if page.params.practiceId}
		<nav aria-label="Practice">
			<Link
				href={resolve('/practices/[practiceId]', { practiceId: page.params.practiceId })}
				label="Overview"
				variant="secondary"
			/>
			<Link
				href={resolve('/practices/[practiceId]/clients', { practiceId: page.params.practiceId })}
				label="Clients"
				variant="secondary"
			/>
			<Link
				href={resolve('/practices/[practiceId]/billing', { practiceId: page.params.practiceId })}
				label="Billing"
				variant="secondary"
			/>
			<!-- Everyone's own Offers, not only a Doula's: an Offer is
			     addressed to a person, and the inbox is scoped to her staff
			     id, so there is nothing here a role check would usefully
			     hide. -->
			<Link
				href={resolve('/practices/[practiceId]/offers', { practiceId: page.params.practiceId })}
				label="Your offers"
				variant="secondary"
			/>
			{#if isOwner}
				<Link
					href={resolve('/practices/[practiceId]/invite', { practiceId: page.params.practiceId })}
					label="Invite a Staff member"
					variant="secondary"
				/>
				<Link
					href={resolve('/practices/[practiceId]/staff', { practiceId: page.params.practiceId })}
					label="Staff"
					variant="secondary"
				/>
				<Link
					href={resolve('/practices/[practiceId]/settings/plan-templates', {
						practiceId: page.params.practiceId
					})}
					label="Plan Templates"
					variant="secondary"
				/>
				<Link
					href={resolve('/practices/[practiceId]/settings/contract-template', {
						practiceId: page.params.practiceId
					})}
					label="Contract Template"
					variant="secondary"
				/>
			{/if}
			<Link
				href={resolve('/practices/[practiceId]/settings/payments', {
					practiceId: page.params.practiceId
				})}
				label="Payments"
				variant="secondary"
			/>
		</nav>
	{/if}
	<div class="account">
		<!--
			Not inside the Practice nav above, and deliberately outside its
			practiceId gate: a Staff member's work state is a fact about her,
			not about any one Practice she works at (#437), so the way to it
			has to be present on every authenticated Staff screen rather than
			only the ones that happen to carry a Practice in the route.

			It also cannot hang off the Staff roster, which is the only other
			place a work state is shown. A Doula has no roster access at all
			and she is exactly the person who has to be able to correct her
			own -- an entry point she cannot reach is not an entry point.
		-->
		<Link href={resolve('/account')} label="Account" variant="secondary" />
		<SignOutButton signOut={handleSignOut} />
	</div>
</header>

{@render children()}

<style>
	@layer components {
		header {
			display: flex;
			align-items: center;
			justify-content: space-between;
			gap: var(--space-4);
			padding: var(--space-2) var(--space-4);
		}

		nav {
			display: flex;
			flex-wrap: wrap;
			gap: var(--space-3);
		}

		/* The person's own controls, kept together at the far end of the
		   header and apart from the Practice's links -- what they act on is
		   her, not whatever Practice she is looking at. */
		.account {
			display: flex;
			flex-wrap: wrap;
			align-items: center;
			gap: var(--space-3);
		}
	}
</style>
