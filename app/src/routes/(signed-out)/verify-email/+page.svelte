<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiBaseURL, probeSession } from '#lib/api.js';
	import type { SessionInfo } from '#lib/landing.js';
	import {
		isUnusableLink,
		LINK_EXPIRED_OR_USED,
		LINK_NOT_COMPLETE,
		refusalMessage,
		SERVICE_PROBLEM
	} from '#lib/formErrors.js';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import UnusableLink from '#lib/components/molecules/UnusableLink.svelte';
	import EntryPage from '#lib/components/templates/EntryPage.svelte';

	const token = page.url.searchParams.get('token') ?? '';

	// #2025: `unusable` is the link's own fault -- not complete, expired or
	// used -- and offers the way to a new one; `failed` is ours, and a
	// reload may still work.
	let status = $state<'checking' | 'verified' | 'unusable' | 'failed'>('checking');
	let errorMessage = $state('');
	// #1504: the link is spent in whatever browser she opens it in. In the
	// one she signed up in she is still signed in, and "Continue to sign in"
	// would misname where the link goes.
	let isSignedIn = $state(false);

	onMount(async () => {
		if (!token) {
			status = 'unusable';
			errorMessage = LINK_NOT_COMPLETE;
			return;
		}
		try {
			const response = await fetch(`${apiBaseURL()}/api/staff/verify-email`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ token })
			});
			if (isUnusableLink(response)) {
				errorMessage = LINK_EXPIRED_OR_USED;
				status = 'unusable';
				return;
			}
			if (!response.ok) {
				errorMessage = await refusalMessage(response);
				status = 'failed';
				return;
			}
			// #613: the cached ID token still says email_verified: false
			// until it's force-refreshed. Only matters if a live Firebase
			// user exists here, which is the exception, not the rule -- every
			// bootstrap flow (login, signup, accept-invite) signs out of the
			// Firebase SDK right after exchanging for the __session cookie,
			// so #606's own enrollment flow (not built yet) will need to
			// re-establish a live Firebase user and refresh again itself
			// before reading email_verified, rather than relying on this.
			const user = getFirebaseAuth().currentUser;
			if (user) {
				await user.reload();
				await user.getIdToken(true);
			}
			isSignedIn = (await probeSession<SessionInfo>('/api/staff/session')) !== undefined;
			status = 'verified';
		} catch {
			errorMessage = SERVICE_PROBLEM;
			status = 'failed';
		}
	});
</script>

<!--
	No `errorSummary`, so no "Error: " title prefix when the link fails
	(#1576): a failed link is not a refused form, and govuk-alignment.md's
	title-format rule keeps that prefix for refused forms only.
-->
<EntryPage title="Verify your email">
	{#snippet content()}
		{#if status === 'checking'}
			<Notice variant="info" message="Checking your link…" />
		{:else if status === 'verified'}
			<Notice variant="status" message="Your email address is verified." />
			{#if isSignedIn}
				<Link href={resolve('/(signed-out)')} label="Go to your Practice" />
			{:else}
				<Link href={resolve('/(signed-out)/login')} label="Continue to sign in" />
			{/if}
		{:else if status === 'unusable'}
			<!-- #2025: a signed-in person asks for a new link from the account screen. -->
			<UnusableLink
				message={errorMessage}
				next={{ label: 'Sign in to get a new link', href: resolve('/(signed-out)/login') }}
			/>
		{:else}
			<Notice variant="error" message={errorMessage} />
		{/if}
	{/snippet}
</EntryPage>
