<script lang="ts">
	/*
	 * The account route's own shell (#484). It sits above the
	 * Practice-scoped tree and inherits only the bare root layout, so
	 * before this it was the one authenticated Staff screen with no skip
	 * link, bar or `<main>` landmark at all -- this supplies exactly that,
	 * matching every other authenticated Staff route (#474 built the
	 * "back to your practices" nav below; this is the chrome around it).
	 *
	 * Grown here rather than folded into a shared `(staff)` group with
	 * practices/+layout.svelte: that layout's own session read is a
	 * second, un-memoized `/api/staff/session` fetch, separate from
	 * loadAccountSession() below -- sharing the layout file would mean
	 * either paying for both fetches on every /account visit, or
	 * reworking that layout's session handling to share
	 * session.svelte.ts's memoization for a route it does not otherwise
	 * touch. Reading `loadAccountSession()`'s existing session for the
	 * bar keeps /account's one fetch its own.
	 *
	 * The bar's own content, with no Practice in this route: `navItems`
	 * is empty, because every nav item StaffTopBar offers is a
	 * Practice-scoped section (Clients, Invoices, ...) and /account is
	 * scoped to the person, not a Practice (#437) -- there is no
	 * Practice-scoped destination to offer here. `practices` is empty
	 * too: the "back to your practices" nav below already is the way to
	 * jump to one, and PracticeSwitcher only ever renders once its
	 * `currentPracticeId` matches one of its options, and /account has
	 * no current Practice to match -- a second, always-empty copy of
	 * that list would cost a render and carry nothing. The avatar stays:
	 * it is the only door back to /account itself (#452), so sign-out
	 * has to work from inside /account too, and it never carries a
	 * push-unregister scope here, the same no-Practice case
	 * practices-layout.svelte.spec.ts already covers for that layout.
	 *
	 * Known gap, filed rather than fixed here (out of scope: StaffTopBar's
	 * visual design): under its 49.25rem floor the narrow sheet still
	 * prints a bare "Practice" label over the empty switcher slot even
	 * with no Practice on the route. See #673.
	 *
	 * loadAccountSession() memoizes the fetch this layout and +page.svelte
	 * both need, so mounting together on every visit to /account costs one
	 * request, not two.
	 */
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { apiFetch } from '#lib/api.js';
	import type { Membership } from '#lib/landing.js';
	import { unregisterPushSubscription } from '#lib/pushRegistration.js';
	import { signOutOfSession, type SignOutOutcome } from '#lib/signOut.js';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import StaffTopBar from '#lib/components/organisms/StaffTopBar.svelte';
	import { loadAccountSession } from './session.svelte.js';

	let { children } = $props();

	let memberships = $state<Membership[]>([]);
	let name = $state('');
	let email = $state<string | undefined>();
	let isLoaded = $state(false);

	onMount(async () => {
		const result = await loadAccountSession();
		if (!result.ok) return;

		memberships = result.session.memberships;
		name = result.session.name;
		email = result.session.email;
		isLoaded = true;
	});

	async function handleSignOut(): Promise<SignOutOutcome> {
		const outcome = await signOutOfSession({
			// /account never carries a Practice, so there is no push
			// subscription scope to unregister here (contrast
			// practices/+layout.svelte's pushUnsubscribeURL()).
			unsubscribeURL: undefined,
			fetcher: apiFetch,
			unregisterPush: unregisterPushSubscription
		});
		if (outcome.ok) await goto(resolve('/(signed-out)/login'));
		return outcome;
	}
</script>

<Link href="#main" label="Skip to main content" variant="skip" />
<!--
	Rendered before the session lands, same as practices/+layout.svelte's
	own bar: it is a fixed 60px whatever it holds, so painting it
	immediately means the page below never moves.
-->
<StaffTopBar
	navItems={[]}
	practices={[]}
	currentPracticeId=""
	{name}
	{email}
	accountHref={resolve('/account')}
	signOut={handleSignOut}
/>
<main id="main" tabindex="-1">
	{@render children()}

	{#if isLoaded}
		<!--
			A way back. The session response already carries every Practice she
			belongs to, so this screen can return her to the one she came from
			without a second read -- and a top-level route outside the Practice
			layout would otherwise be a place with no exit but the back button.
			One Practice, one link; several, several.
		-->
		<nav aria-label="Your practices">
			<Heading level={2} variant="section" text="Back to your practices" />
			<ul>
				{#each memberships as membership (membership.practiceId)}
					<li>
						<Link
							href={resolve('/practices/[practiceId]', { practiceId: membership.practiceId })}
							label={membership.practiceName}
						/>
					</li>
				{/each}
			</ul>
		</nav>
	{/if}
</main>

<style>
	@layer components {
		ul {
			margin: 0;
			padding: 0;
			list-style: none;
			display: flex;
			flex-wrap: wrap;
			gap: var(--space-3);
		}

		nav {
			padding: var(--space-6) var(--page-gutter);
		}
	}
</style>
