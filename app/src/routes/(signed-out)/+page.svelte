<script lang="ts">
	/*
	 * The wayfinding root, `/` (#357), now inside the signed-out route
	 * group so that it carries a shell (#678).
	 *
	 * #484 asked either that `/` render inside a shell or that the reason
	 * it does not be recorded; #357 built what `/` shows and never touched
	 * chrome, so `/` rendered with no skip link, no `<main>` landmark and
	 * no bar. Three of its four states -- the signed-out landing, the
	 * Staff picker, the portal picker -- are pages a
	 * person lands on and reads; only the single-destination redirect is
	 * gone before anyone sees it. A page a person reads gets the same
	 * chrome as every other, so the decision is: `/` carries a shell.
	 *
	 * It is the `(signed-out)` group's shell rather than a group of its
	 * own. That group is already the reduced bar's home, and already holds
	 * the one screen there that needs a session -- `/no-practice` (#745),
	 * whose session belongs to no Practice to put a name to. Every state
	 * of `/` is that same case: at first paint the app does not yet know
	 * which population is reading, and in the two picker states it knows
	 * the population but no Practice or Engagement has been chosen. The
	 * reduced bar is the only bar that is true in all three.
	 *
	 * Deliberately NOT swapped to `StaffTopBar`/`PortalTopBar` once
	 * `data.type` resolves: each needs its own population's sign-out
	 * handler and its own second render, and in every state that offers a
	 * destination each destination is one click away and lands in a fully
	 * dressed shell that already carries sign-out.
	 *
	 * One state offers no destination: a Portal Account with no
	 * Engagement at all. #1116 settled what that screen shows, which is
	 * the part the bar could not answer: the state names itself
	 * (NO_CARE_HEADING), the paragraph beneath it gives what to do about
	 * each of the two things that produce it, and the screen carries its
	 * own `SignOutButton` -- so the release the reduced bar withholds is
	 * on the page that says the session leads nowhere. The bar itself is
	 * unchanged, and the reasoning above still holds for the other three
	 * states.
	 *
	 * The template is `EntryPage`, whose own doc comment already names
	 * "a 'choose a Practice' or 'choose an Engagement' picker" as content
	 * it serves. It supplies the gutters and the column ADR-0018 keeps out
	 * of the layout, which is what gives this screen a readable measure at
	 * every width rather than text flush to both edges.
	 *
	 * The signed-out state's heading used to be `Doula Cloud`. With the
	 * bar above it now carrying the brand lockup, an `<h1>` repeating it
	 * says nothing about the page; per ADR-0021 the heading names what
	 * the page is for instead. `EntryPage` writes the tab title from the
	 * same string, so the tab now names the page too rather than the
	 * `Home` this route used to pass `PageTitle` directly.
	 */
	import { goto, invalidateAll } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { apiFetch } from '#lib/api.js';
	import { unregisterPushSubscription } from '#lib/pushRegistration.js';
	import { signOutOfSession, type SignOutOutcome } from '#lib/signOut.js';
	import Link from '#lib/components/atoms/Link.svelte';
	import SignOutButton from '#lib/components/molecules/SignOutButton.svelte';
	import EntryPage from '#lib/components/templates/EntryPage.svelte';
	import {
		CARE_HEADING,
		NO_CARE_HEADING,
		NO_CARE_MESSAGE,
		engagementLabel,
		engagementStatusLabel
	} from '#lib/clientRegister.js';
	import type { RootLanding } from './+page.js';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();

	const HEADINGS: Record<RootLanding['type'], string> = {
		'signed-out': 'Sign in or set up a Practice',
		'staff-picker': 'Choose a Practice',
		'portal-picker': CARE_HEADING
	};

	// The portal picker splits in two (#1116): a list of care, and the
	// state of having none. EntryPage writes the tab title from this same
	// string, so the tab names the state too rather than promising a list
	// that is not there.
	const hasNoCare = $derived(data.type === 'portal-picker' && data.engagements.length === 0);
	const title = $derived(hasNoCare ? NO_CARE_HEADING : HEADINGS[data.type]);

	/*
	 * The door out of a session that leads nowhere (#1116). Not extracted
	 * alongside `/no-practice`'s own handler, which is the only other
	 * scope-less sign-out: the two differ in where they land -- a Client
	 * sent to the Staff login would be shown a door that is not hers
	 * (#153) -- and in this one's `invalidateAll`, so they are two
	 * behaviors that read alike rather than one repeated twice.
	 */
	async function handleSignOut(): Promise<SignOutOutcome> {
		const outcome = await signOutOfSession({
			// No Engagement, so no Engagement-scoped push subscription to
			// unregister -- the state this screen reports is exactly the
			// absence of that scope.
			unsubscribeURL: undefined,
			// apiFetch, not apiFetchWithSession: an end-session request that
			// has already cleared the cookie must not read as an expired
			// session and navigate on its own, since a failed sign-out is
			// reported in place.
			fetcher: apiFetch,
			unregisterPush: unregisterPushSubscription
		});
		if (outcome.ok) {
			// `+page.ts`'s own load resolved this session; signing out changes
			// nothing SvelteKit tracks, so without this a Back to `/` would
			// redraw the still-signed-in answer it cached.
			await invalidateAll();
			await goto(resolve('/portal/(signed-out)/login'));
		}
		return outcome;
	}

	// Mirrors StepRail's own id-per-row join (#464): the status text sits
	// beside the link rather than inside its accessible name, so a
	// keyboard or screen-reader user still hears the status without it
	// being read as part of the link's own name. Telling two Engagements
	// at the same Practice apart is the link's own name's job instead
	// (#310): engagementLabel folds in when each began.
	function statusId(engagementId: string) {
		return `engagement-status-${engagementId}`;
	}
</script>

{#snippet content()}
	{#if data.type === 'signed-out'}
		<ul>
			<li><Link href={resolve('/(signed-out)/login')} label="Staff log in" /></li>
			<li><Link href={resolve('/(signed-out)/signup')} label="Set up a Practice" /></li>
			<li><Link href={resolve('/portal/(signed-out)/login')} label="Client portal log in" /></li>
		</ul>
	{:else if data.type === 'staff-picker'}
		<!--
			Always at least one: `+page.ts` redirects a Staff member with no
			Membership to `/no-practice` rather than handing this an empty list
			(#745).
		-->
		<ul>
			{#each data.memberships as membership (membership.practiceId)}
				<li>
					<Link
						href={resolve('/practices/[practiceId]', { practiceId: membership.practiceId })}
						label={membership.practiceName}
					/>
				</li>
			{/each}
		</ul>
	{:else if data.engagements.length === 0}
		<!--
			#1116: two people reach this state -- one whose Practice has not
			set her care up yet, and one who signed in with an address her
			Practice does not hold -- so it gives what to do about each,
			and then the way out of the session itself. ADR-0021's rule for
			a screen that reports a state: say what has happened (the
			heading) and what to do next.
		-->
		<p>{NO_CARE_MESSAGE}</p>
		<p>
			If your Practice has already set up your care, this may be a different email address from the
			one your Practice has. Sign out, then sign in with that address.
		</p>
		<SignOutButton signOut={handleSignOut} />
	{:else}
		<!--
			#312: every Engagement her Portal Account reaches, across every
			Practice, past and present -- a completed one stays listed and
			stays openable (ADR-0015). `+page.ts` already redirects straight
			through when there is only one, so this only ever renders with
			two or more.
		-->
		<ul>
			{#each data.engagements as engagement (engagement.engagementId)}
				<li>
					<Link
						href={resolve('/portal/(authenticated)/engagements/[engagementId]', {
							engagementId: engagement.engagementId
						})}
						label={engagementLabel(engagement)}
						describedBy={statusId(engagement.engagementId)}
					/>
					<p class="status" id={statusId(engagement.engagementId)}>
						{engagementStatusLabel(engagement.status)}
					</p>
				</li>
			{/each}
		</ul>
	{/if}
{/snippet}

<EntryPage {title} {content} />

<style>
	@layer components {
		.status {
			margin: 0 0 var(--space-3);
			color: var(--color-on-surface-muted);
			font-size: var(--text-body-sm-size);
		}
	}
</style>
